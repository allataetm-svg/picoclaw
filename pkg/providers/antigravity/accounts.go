package antigravity

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sipeed/picoclaw/pkg/auth"
	"github.com/sipeed/picoclaw/pkg/logger"
)

type AccountManager struct {
	mu          sync.RWMutex
	accounts    *AccountsConfig
	config      ProviderConfig
	configDir   string
	currentIdx  int
	lastUsedIdx int
}

func NewAccountManager(configDir string, cfg ProviderConfig) *AccountManager {
	if configDir == "" {
		home, _ := os.UserHomeDir()
		configDir = filepath.Join(home, ".picoclaw")
	}
	am := &AccountManager{
		config:     cfg,
		configDir:  configDir,
		currentIdx: 0,
	}
	am.loadAccounts()
	return am
}

func (am *AccountManager) accountsPath() string {
	return filepath.Join(am.configDir, AccountsFile)
}

func (am *AccountManager) configPath() string {
	return filepath.Join(am.configDir, ConfigFile)
}

func (am *AccountManager) loadAccounts() {
	am.mu.Lock()
	defer am.mu.Unlock()

	am.accounts = &AccountsConfig{
		Accounts: []Account{},
	}

	data, err := os.ReadFile(am.accountsPath())
	if err != nil {
		if !os.IsNotExist(err) {
			logger.WarnCF("antigravity.accounts", "Failed to load accounts file", map[string]any{"error": err.Error()})
		}
		return
	}

	if err := json.Unmarshal(data, am.accounts); err != nil {
		logger.WarnCF("antigravity.accounts", "Failed to parse accounts file", map[string]any{"error": err.Error()})
		return
	}

	// Migrate from single credential store if no accounts
	if len(am.accounts.Accounts) == 0 {
		am.migrateFromSingleCredential()
	}

	logger.DebugCF("antigravity.accounts", "Loaded accounts", map[string]any{"count": len(am.accounts.Accounts)})
}

func (am *AccountManager) migrateFromSingleCredential() {
	cred, err := auth.GetCredential("google-antigravity")
	if err != nil || cred == nil {
		return
	}

	account := Account{
		Email:        cred.Email,
		RefreshToken: cred.RefreshToken,
		AccessToken:  cred.AccessToken,
		ProjectID:    cred.ProjectID,
		ExpiresAt:    cred.ExpiresAt,
	}

	am.accounts.Accounts = []Account{account}
	am.saveAccountsWithoutLock()

	logger.InfoCF("antigravity.accounts", "Migrated from single credential store", map[string]any{"email": cred.Email})
}

func (am *AccountManager) SaveAccounts() error {
	am.mu.Lock()
	defer am.mu.Unlock()
	return am.saveAccountsWithoutLock()
}

func (am *AccountManager) saveAccountsWithoutLock() error {
	if err := os.MkdirAll(am.configDir, 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := json.MarshalIndent(am.accounts, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling accounts: %w", err)
	}

	if err := os.WriteFile(am.accountsPath(), data, 0600); err != nil {
		return fmt.Errorf("writing accounts file: %w", err)
	}

	return nil
}

func (am *AccountManager) AddAccount(account Account) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	// Check for duplicate email
	for i, existing := range am.accounts.Accounts {
		if existing.Email == account.Email {
			// Update existing account
			am.accounts.Accounts[i] = account
			logger.InfoCF("antigravity.accounts", "Updated existing account", map[string]any{"email": account.Email})
			return am.saveAccountsWithoutLock()
		}
	}

	// Add new account
	am.accounts.Accounts = append(am.accounts.Accounts, account)
	logger.InfoCF("antigravity.accounts", "Added new account", map[string]any{
		"email": account.Email,
		"total": len(am.accounts.Accounts),
	})

	return am.saveAccountsWithoutLock()
}

func (am *AccountManager) RemoveAccount(email string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	for i, account := range am.accounts.Accounts {
		if account.Email == email {
			am.accounts.Accounts = append(am.accounts.Accounts[:i], am.accounts.Accounts[i+1:]...)
			if am.currentIdx >= len(am.accounts.Accounts) {
				am.currentIdx = 0
			}
			logger.InfoCF("antigravity.accounts", "Removed account", map[string]any{"email": email})
			return am.saveAccountsWithoutLock()
		}
	}

	return fmt.Errorf("account not found: %s", email)
}

func (am *AccountManager) GetAccounts() []Account {
	am.mu.RLock()
	defer am.mu.RUnlock()

	accounts := make([]Account, len(am.accounts.Accounts))
	copy(accounts, am.accounts.Accounts)
	return accounts
}

func (am *AccountManager) GetAccount(email string) *Account {
	am.mu.RLock()
	defer am.mu.RUnlock()

	for i := range am.accounts.Accounts {
		if am.accounts.Accounts[i].Email == email {
			return &am.accounts.Accounts[i]
		}
	}
	return nil
}

func (am *AccountManager) AccountCount() int {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return len(am.accounts.Accounts)
}

func (am *AccountManager) SelectAccount() (*Account, error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	if len(am.accounts.Accounts) == 0 {
		return nil, fmt.Errorf("no accounts configured. Run: picoclaw auth login --provider google-antigravity")
	}

	strategy := am.config.AccountSelectionStrategy
	if am.accounts.AccountSelectionStrategy != "" {
		strategy = am.accounts.AccountSelectionStrategy
	}

	// Filter available accounts
	var available []int
	for i := range am.accounts.Accounts {
		account := &am.accounts.Accounts[i]
		if account.Disabled {
			continue
		}
		if account.IsRateLimited() {
			continue
		}
		available = append(available, i)
	}

	if len(available) == 0 {
		// All accounts are rate limited - find the one with shortest wait
		return nil, am.findBestRateLimitedAccount()
	}

	var selectedIdx int
	switch AccountSelectionStrategy(strategy) {
	case StrategySticky:
		// Try to use the same account for session continuity
		if am.currentIdx < len(am.accounts.Accounts) && !am.accounts.Accounts[am.currentIdx].Disabled {
			selectedIdx = am.currentIdx
		} else {
			selectedIdx = available[0]
		}

	case StrategyRoundRobin:
		// Rotate through all accounts
		for _, idx := range available {
			if idx > am.currentIdx {
				selectedIdx = idx
				break
			}
		}
		if selectedIdx == 0 && len(available) > 0 {
			selectedIdx = available[0]
		}

	case StrategyPerformance:
		// Prefer accounts with fewer failures
		selectedIdx = am.findBestPerformingAccount(available)

	case StrategyHybrid:
		fallthrough
	default:
		// Hybrid: prefer same account (for cache), fall back to best available
		if am.currentIdx < len(am.accounts.Accounts) {
			account := &am.accounts.Accounts[am.currentIdx]
			if !account.Disabled && !account.IsRateLimited() && account.FailureScore() < 3 {
				selectedIdx = am.currentIdx
				break
			}
		}
		selectedIdx = am.findBestPerformingAccount(available)
	}

	am.currentIdx = selectedIdx
	account := &am.accounts.Accounts[selectedIdx]

	logger.DebugCF("antigravity.accounts", "Selected account", map[string]any{
		"email":    account.Email,
		"index":    selectedIdx,
		"strategy": strategy,
	})

	return account, nil
}

func (am *AccountManager) findBestPerformingAccount(available []int) int {
	if len(available) == 0 {
		return 0
	}

	bestIdx := available[0]
	bestScore := am.accounts.Accounts[bestIdx].FailureScore()

	for _, idx := range available[1:] {
		score := am.accounts.Accounts[idx].FailureScore()
		if score < bestScore {
			bestScore = score
			bestIdx = idx
		}
	}

	return bestIdx
}

func (am *AccountManager) findBestRateLimitedAccount() error {
	if len(am.accounts.Accounts) == 0 {
		return fmt.Errorf("no accounts available")
	}

	// Find account with earliest rate limit expiry
	var bestAccount *Account
	var earliestExpiry time.Time

	for i := range am.accounts.Accounts {
		account := &am.accounts.Accounts[i]
		if account.Disabled {
			continue
		}
		rlUntil := account.RateLimitedUntil()
		if bestAccount == nil || rlUntil.Before(earliestExpiry) {
			bestAccount = account
			earliestExpiry = rlUntil
		}
	}

	if bestAccount == nil {
		return fmt.Errorf("all accounts are disabled")
	}

	waitDuration := time.Until(earliestExpiry)
	if waitDuration > 0 {
		return fmt.Errorf("all accounts rate limited. Next available in %v (%s)",
			waitDuration.Round(time.Second), bestAccount.Email)
	}

	return nil
}

func (am *AccountManager) MarkRateLimited(email string, duration time.Duration) {
	am.mu.Lock()
	defer am.mu.Unlock()

	for i := range am.accounts.Accounts {
		if am.accounts.Accounts[i].Email == email {
			am.accounts.Accounts[i].SetRateLimited(duration)
			logger.InfoCF("antigravity.accounts", "Account rate limited", map[string]any{
				"email":    email,
				"duration": duration.String(),
			})
			break
		}
	}
}

func (am *AccountManager) MarkFailure(email string) {
	am.mu.Lock()
	defer am.mu.Unlock()

	for i := range am.accounts.Accounts {
		if am.accounts.Accounts[i].Email == email {
			am.accounts.Accounts[i].IncrementFailure()
			logger.DebugCF("antigravity.accounts", "Account failure recorded", map[string]any{
				"email": email,
				"count": am.accounts.Accounts[i].FailureScore(),
			})
			break
		}
	}
}

func (am *AccountManager) ClearFailures(email string) {
	am.mu.Lock()
	defer am.mu.Unlock()

	for i := range am.accounts.Accounts {
		if am.accounts.Accounts[i].Email == email {
			am.accounts.Accounts[i].ResetFailures()
			break
		}
	}
}

func (am *AccountManager) UpdateAccessToken(email, accessToken string, expiresAt time.Time) {
	am.mu.Lock()
	defer am.mu.Unlock()

	for i := range am.accounts.Accounts {
		if am.accounts.Accounts[i].Email == email {
			am.accounts.Accounts[i].AccessToken = accessToken
			am.accounts.Accounts[i].ExpiresAt = expiresAt
			_ = am.saveAccountsWithoutLock()
			break
		}
	}
}

func (am *AccountManager) UpdateProjectID(email, projectID string) {
	am.mu.Lock()
	defer am.mu.Unlock()

	for i := range am.accounts.Accounts {
		if am.accounts.Accounts[i].Email == email {
			am.accounts.Accounts[i].ProjectID = projectID
			_ = am.saveAccountsWithoutLock()
			break
		}
	}
}

func (am *AccountManager) SetCurrentAccount(email string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	for i, account := range am.accounts.Accounts {
		if account.Email == email {
			am.currentIdx = i
			return nil
		}
	}

	return fmt.Errorf("account not found: %s", email)
}

func (am *AccountManager) GetCurrentAccount() *Account {
	am.mu.RLock()
	defer am.mu.RUnlock()

	if len(am.accounts.Accounts) == 0 || am.currentIdx >= len(am.accounts.Accounts) {
		return nil
	}

	return &am.accounts.Accounts[am.currentIdx]
}
