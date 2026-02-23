// PicoClaw - Ultra-lightweight personal AI agent
// License: MIT

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/auth"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/providers"
	"github.com/sipeed/picoclaw/pkg/providers/antigravity"
)

const supportedProvidersMsg = "Supported providers: openai, anthropic, google-antigravity"

func authCmd() {
	if len(os.Args) < 3 {
		authHelp()
		return
	}

	switch os.Args[2] {
	case "login":
		authLoginCmd()
	case "logout":
		authLogoutCmd()
	case "status":
		authStatusCmd()
	case "models":
		authModelsCmd()
	case "accounts":
		authAccountsCmd()
	case "quota":
		authQuotaCmd()
	case "account":
		authAccountCmd()
	case "setup":
		authSetupCmd()
	default:
		fmt.Printf("Unknown auth command: %s\n", os.Args[2])
		authHelp()
	}
}

func authAccountCmd() {
	if len(os.Args) < 4 {
		fmt.Println("Account commands:")
		fmt.Println("  add     Add a new account (alias for login)")
		fmt.Println("  remove  Remove an account by email")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  picoclaw auth account add")
		fmt.Println("  picoclaw auth account remove --email user@example.com")
		return
	}

	switch os.Args[3] {
	case "add":
		authAccountAddCmd()
	case "remove":
		authAccountRemoveCmd()
	default:
		fmt.Printf("Unknown account command: %s\n", os.Args[3])
	}
}

func authHelp() {
	fmt.Println("\nAuth commands:")
	fmt.Println("  login       Login via OAuth or paste token")
	fmt.Println("  logout      Remove stored credentials")
	fmt.Println("  status      Show current auth status")
	fmt.Println("  models      List available Antigravity models")
	fmt.Println("  accounts    List multi-account configuration (Antigravity)")
	fmt.Println("  quota       Check account quotas (Antigravity)")
	fmt.Println("  account     Manage accounts (add, remove)")
	fmt.Println("  setup       Interactive setup for providers (OpenCode, etc.)")
	fmt.Println()
	fmt.Println("Login options:")
	fmt.Println("  --provider <name>    Provider to login with (openai, anthropic, google-antigravity)")
	fmt.Println("  --device-code         Use device code flow (for headless environments)")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  picoclaw auth login --provider openai")
	fmt.Println("  picoclaw auth login --provider openai --device-code")
	fmt.Println("  picoclaw auth login --provider anthropic")
	fmt.Println("  picoclaw auth login --provider google-antigravity")
	fmt.Println("  picoclaw auth accounts")
	fmt.Println("  picoclaw auth quota")
	fmt.Println("  picoclaw auth models")
	fmt.Println("  picoclaw auth logout --provider openai")
	fmt.Println("  picoclaw auth status")
	fmt.Println("  picoclaw auth setup")
}

func authLoginCmd() {
	provider := ""
	useDeviceCode := false

	args := os.Args[3:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--provider", "-p":
			if i+1 < len(args) {
				provider = args[i+1]
				i++
			}
		case "--device-code":
			useDeviceCode = true
		}
	}

	if provider == "" {
		fmt.Println("Error: --provider is required")
		fmt.Println(supportedProvidersMsg)
		return
	}

	switch provider {
	case "openai":
		authLoginOpenAI(useDeviceCode)
	case "anthropic":
		authLoginPasteToken(provider)
	case "google-antigravity", "antigravity":
		authLoginGoogleAntigravity()
	default:
		fmt.Printf("Unsupported provider: %s\n", provider)
		fmt.Println(supportedProvidersMsg)
	}
}

func authLoginOpenAI(useDeviceCode bool) {
	cfg := auth.OpenAIOAuthConfig()

	var cred *auth.AuthCredential
	var err error

	if useDeviceCode {
		cred, err = auth.LoginDeviceCode(cfg)
	} else {
		cred, err = auth.LoginBrowser(cfg)
	}

	if err != nil {
		fmt.Printf("Login failed: %v\n", err)
		os.Exit(1)
	}

	if err = auth.SetCredential("openai", cred); err != nil {
		fmt.Printf("Failed to save credentials: %v\n", err)
		os.Exit(1)
	}

	appCfg, err := loadConfig()
	if err == nil {
		// Update Providers (legacy format)
		appCfg.Providers.OpenAI.AuthMethod = "oauth"

		// Update or add openai in ModelList
		foundOpenAI := false
		for i := range appCfg.ModelList {
			if isOpenAIModel(appCfg.ModelList[i].Model) {
				appCfg.ModelList[i].AuthMethod = "oauth"
				foundOpenAI = true
				break
			}
		}

		// If no openai in ModelList, add it
		if !foundOpenAI {
			appCfg.ModelList = append(appCfg.ModelList, config.ModelConfig{
				ModelName:  "gpt-5.2",
				Model:      "openai/gpt-5.2",
				AuthMethod: "oauth",
			})
		}

		// Update default model to use OpenAI
		appCfg.Agents.Defaults.Model = "gpt-5.2"

		if err := config.SaveConfig(getConfigPath(), appCfg); err != nil {
			fmt.Printf("Warning: could not update config: %v\n", err)
		}
	}

	fmt.Println("Login successful!")
	if cred.AccountID != "" {
		fmt.Printf("Account: %s\n", cred.AccountID)
	}
	fmt.Println("Default model set to: gpt-5.2")
}

func authLoginGoogleAntigravity() {
	cfg := auth.GoogleAntigravityOAuthConfig()

	cred, err := auth.LoginBrowser(cfg)
	if err != nil {
		fmt.Printf("Login failed: %v\n", err)
		os.Exit(1)
	}

	cred.Provider = "google-antigravity"

	// Fetch user email from Google userinfo
	email, err := fetchGoogleUserEmail(cred.AccessToken)
	if err != nil {
		fmt.Printf("Warning: could not fetch email: %v\n", err)
	} else {
		cred.Email = email
		fmt.Printf("Email: %s\n", email)
	}

	// Fetch Cloud Code Assist project ID
	projectID, err := providers.FetchAntigravityProjectID(cred.AccessToken)
	if err != nil {
		fmt.Printf("Warning: could not fetch project ID: %v\n", err)
		fmt.Println("You may need Google Cloud Code Assist enabled on your account.")
	} else {
		cred.ProjectID = projectID
		fmt.Printf("Project: %s\n", projectID)
	}

	if err = auth.SetCredential("google-antigravity", cred); err != nil {
		fmt.Printf("Failed to save credentials: %v\n", err)
		os.Exit(1)
	}

	appCfg, err := loadConfig()
	if err == nil {
		// Update Providers (legacy format, for backward compatibility)
		appCfg.Providers.Antigravity.AuthMethod = "oauth"

		// Update or add antigravity in ModelList
		foundAntigravity := false
		for i := range appCfg.ModelList {
			if isAntigravityModel(appCfg.ModelList[i].Model) {
				appCfg.ModelList[i].AuthMethod = "oauth"
				foundAntigravity = true
				break
			}
		}

		// If no antigravity in ModelList, add it
		if !foundAntigravity {
			appCfg.ModelList = append(appCfg.ModelList, config.ModelConfig{
				ModelName:  "gemini-flash",
				Model:      "antigravity/gemini-3-flash",
				AuthMethod: "oauth",
			})
		}

		// Update default model
		appCfg.Agents.Defaults.Model = "gemini-flash"

		if err := config.SaveConfig(getConfigPath(), appCfg); err != nil {
			fmt.Printf("Warning: could not update config: %v\n", err)
		}
	}

	fmt.Println("\n✓ Google Antigravity login successful!")
	fmt.Println("Default model set to: gemini-flash")
	fmt.Println("Try it: picoclaw agent -m \"Hello world\"")
}

func fetchGoogleUserEmail(accessToken string) (string, error) {
	req, err := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("userinfo request failed: %s", string(body))
	}

	var userInfo struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return "", err
	}
	return userInfo.Email, nil
}

func authLoginPasteToken(provider string) {
	cred, err := auth.LoginPasteToken(provider, os.Stdin)
	if err != nil {
		fmt.Printf("Login failed: %v\n", err)
		os.Exit(1)
	}

	if err = auth.SetCredential(provider, cred); err != nil {
		fmt.Printf("Failed to save credentials: %v\n", err)
		os.Exit(1)
	}

	appCfg, err := loadConfig()
	if err == nil {
		switch provider {
		case "anthropic":
			appCfg.Providers.Anthropic.AuthMethod = "token"
			// Update ModelList
			found := false
			for i := range appCfg.ModelList {
				if isAnthropicModel(appCfg.ModelList[i].Model) {
					appCfg.ModelList[i].AuthMethod = "token"
					found = true
					break
				}
			}
			if !found {
				appCfg.ModelList = append(appCfg.ModelList, config.ModelConfig{
					ModelName:  "claude-sonnet-4.6",
					Model:      "anthropic/claude-sonnet-4.6",
					AuthMethod: "token",
				})
			}
			// Update default model
			appCfg.Agents.Defaults.Model = "claude-sonnet-4.6"
		case "openai":
			appCfg.Providers.OpenAI.AuthMethod = "token"
			// Update ModelList
			found := false
			for i := range appCfg.ModelList {
				if isOpenAIModel(appCfg.ModelList[i].Model) {
					appCfg.ModelList[i].AuthMethod = "token"
					found = true
					break
				}
			}
			if !found {
				appCfg.ModelList = append(appCfg.ModelList, config.ModelConfig{
					ModelName:  "gpt-5.2",
					Model:      "openai/gpt-5.2",
					AuthMethod: "token",
				})
			}
			// Update default model
			appCfg.Agents.Defaults.Model = "gpt-5.2"
		}
		if err := config.SaveConfig(getConfigPath(), appCfg); err != nil {
			fmt.Printf("Warning: could not update config: %v\n", err)
		}
	}

	fmt.Printf("Token saved for %s!\n", provider)
	fmt.Printf("Default model set to: %s\n", appCfg.Agents.Defaults.Model)
}

func authLogoutCmd() {
	provider := ""

	args := os.Args[3:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--provider", "-p":
			if i+1 < len(args) {
				provider = args[i+1]
				i++
			}
		}
	}

	if provider != "" {
		if err := auth.DeleteCredential(provider); err != nil {
			fmt.Printf("Failed to remove credentials: %v\n", err)
			os.Exit(1)
		}

		appCfg, err := loadConfig()
		if err == nil {
			// Clear AuthMethod in ModelList
			for i := range appCfg.ModelList {
				switch provider {
				case "openai":
					if isOpenAIModel(appCfg.ModelList[i].Model) {
						appCfg.ModelList[i].AuthMethod = ""
					}
				case "anthropic":
					if isAnthropicModel(appCfg.ModelList[i].Model) {
						appCfg.ModelList[i].AuthMethod = ""
					}
				case "google-antigravity", "antigravity":
					if isAntigravityModel(appCfg.ModelList[i].Model) {
						appCfg.ModelList[i].AuthMethod = ""
					}
				}
			}
			// Clear AuthMethod in Providers (legacy)
			switch provider {
			case "openai":
				appCfg.Providers.OpenAI.AuthMethod = ""
			case "anthropic":
				appCfg.Providers.Anthropic.AuthMethod = ""
			case "google-antigravity", "antigravity":
				appCfg.Providers.Antigravity.AuthMethod = ""
			}
			config.SaveConfig(getConfigPath(), appCfg)
		}

		fmt.Printf("Logged out from %s\n", provider)
	} else {
		if err := auth.DeleteAllCredentials(); err != nil {
			fmt.Printf("Failed to remove credentials: %v\n", err)
			os.Exit(1)
		}

		appCfg, err := loadConfig()
		if err == nil {
			// Clear all AuthMethods in ModelList
			for i := range appCfg.ModelList {
				appCfg.ModelList[i].AuthMethod = ""
			}
			// Clear all AuthMethods in Providers (legacy)
			appCfg.Providers.OpenAI.AuthMethod = ""
			appCfg.Providers.Anthropic.AuthMethod = ""
			appCfg.Providers.Antigravity.AuthMethod = ""
			config.SaveConfig(getConfigPath(), appCfg)
		}

		fmt.Println("Logged out from all providers")
	}
}

func authStatusCmd() {
	store, err := auth.LoadStore()
	if err != nil {
		fmt.Printf("Error loading auth store: %v\n", err)
		return
	}

	if len(store.Credentials) == 0 {
		fmt.Println("No authenticated providers.")
		fmt.Println("Run: picoclaw auth login --provider <name>")
		return
	}

	fmt.Println("\nAuthenticated Providers:")
	fmt.Println("------------------------")
	for provider, cred := range store.Credentials {
		status := "active"
		if cred.IsExpired() {
			status = "expired"
		} else if cred.NeedsRefresh() {
			status = "needs refresh"
		}

		fmt.Printf("  %s:\n", provider)
		fmt.Printf("    Method: %s\n", cred.AuthMethod)
		fmt.Printf("    Status: %s\n", status)
		if cred.AccountID != "" {
			fmt.Printf("    Account: %s\n", cred.AccountID)
		}
		if cred.Email != "" {
			fmt.Printf("    Email: %s\n", cred.Email)
		}
		if cred.ProjectID != "" {
			fmt.Printf("    Project: %s\n", cred.ProjectID)
		}
		if !cred.ExpiresAt.IsZero() {
			fmt.Printf("    Expires: %s\n", cred.ExpiresAt.Format("2006-01-02 15:04"))
		}
	}
}

func authModelsCmd() {
	cred, err := auth.GetCredential("google-antigravity")
	if err != nil || cred == nil {
		fmt.Println("Not logged in to Google Antigravity.")
		fmt.Println("Run: picoclaw auth login --provider google-antigravity")
		return
	}

	// Refresh token if needed
	if cred.NeedsRefresh() && cred.RefreshToken != "" {
		oauthCfg := auth.GoogleAntigravityOAuthConfig()
		refreshed, refreshErr := auth.RefreshAccessToken(cred, oauthCfg)
		if refreshErr == nil {
			cred = refreshed
			_ = auth.SetCredential("google-antigravity", cred)
		}
	}

	projectID := cred.ProjectID
	if projectID == "" {
		fmt.Println("No project ID stored. Try logging in again.")
		return
	}

	fmt.Printf("Fetching models for project: %s\n\n", projectID)

	models, err := providers.FetchAntigravityModels(cred.AccessToken, projectID)
	if err != nil {
		fmt.Printf("Error fetching models: %v\n", err)
		return
	}

	if len(models) == 0 {
		fmt.Println("No models available.")
		return
	}

	fmt.Println("Available Antigravity Models:")
	fmt.Println("-----------------------------")
	for _, m := range models {
		status := "✓"
		if m.IsExhausted {
			status = "✗ (quota exhausted)"
		}
		name := m.ID
		if m.DisplayName != "" {
			name = fmt.Sprintf("%s (%s)", m.ID, m.DisplayName)
		}
		fmt.Printf("  %s %s\n", status, name)
	}
}

// isAntigravityModel checks if a model string belongs to antigravity provider
func isAntigravityModel(model string) bool {
	return model == "antigravity" ||
		model == "google-antigravity" ||
		strings.HasPrefix(model, "antigravity/") ||
		strings.HasPrefix(model, "google-antigravity/")
}

// isOpenAIModel checks if a model string belongs to openai provider
func isOpenAIModel(model string) bool {
	return model == "openai" ||
		strings.HasPrefix(model, "openai/")
}

// isAnthropicModel checks if a model string belongs to anthropic provider
func isAnthropicModel(model string) bool {
	return model == "anthropic" ||
		strings.HasPrefix(model, "anthropic/")
}

// --- Multi-account Antigravity support ---

func authAccountsCmd() {
	home, _ := os.UserHomeDir()
	configDir := filepath.Join(home, ".picoclaw")
	accountManager := antigravity.NewProvider(configDir).GetAccountManager()

	accounts := accountManager.GetAccounts()
	if len(accounts) == 0 {
		fmt.Println("No accounts configured.")
		fmt.Println("Run: picoclaw auth login --provider google-antigravity")
		return
	}

	fmt.Println("\nAntigravity Accounts:")
	fmt.Println("----------------------")
	for i, acc := range accounts {
		active := ""
		if i == 0 {
			active = " (active)"
		}
		disabled := ""
		if acc.Disabled {
			disabled = " [DISABLED]"
		}
		rateLimited := ""
		if acc.IsRateLimited() {
			rateLimited = " [RATE LIMITED]"
		}
		fmt.Printf("  %d. %s%s%s%s\n", i+1, acc.Email, active, disabled, rateLimited)
		if acc.ProjectID != "" {
			fmt.Printf("      Project: %s\n", acc.ProjectID)
		}
	}
	fmt.Printf("\nTotal: %d account(s)\n", len(accounts))
}

func authQuotaCmd() {
	home, _ := os.UserHomeDir()
	configDir := filepath.Join(home, ".picoclaw")
	provider := antigravity.NewProvider(configDir)

	accountManager := provider.GetAccountManager()
	accounts := accountManager.GetAccounts()
	if len(accounts) == 0 {
		fmt.Println("No accounts configured.")
		return
	}

	for _, acc := range accounts {
		fmt.Printf("\nAccount: %s\n", acc.Email)
		fmt.Println(strings.Repeat("-", 40))

		// Refresh token if needed
		account := accountManager.GetAccount(acc.Email)
		if account != nil && account.NeedsRefresh() && account.RefreshToken != "" {
			fmt.Println("  Refreshing token...")
		}

		projectID := acc.ProjectID
		if projectID == "" {
			fmt.Println("  No project ID. Skipping quota check.")
			continue
		}

		// Note: Would need to fetch access token here in a real implementation
		_ = projectID // Placeholder for actual quota fetch
		fmt.Printf("  Project: %s\n", projectID)
		fmt.Println("  (Run 'picoclaw auth models' for detailed model list)")
	}
}

func authAccountAddCmd() {
	authLoginGoogleAntigravityMulti()
}

func authAccountRemoveCmd() {
	email := ""
	args := os.Args[4:]
	for i := 0; i < len(args); i++ {
		if args[i] == "--email" || args[i] == "-e" {
			if i+1 < len(args) {
				email = args[i+1]
				break
			}
		}
	}

	if email == "" {
		fmt.Println("Error: --email is required")
		fmt.Println("Usage: picoclaw auth account remove --email user@example.com")
		return
	}

	home, _ := os.UserHomeDir()
	configDir := filepath.Join(home, ".picoclaw")
	accountManager := antigravity.NewProvider(configDir).GetAccountManager()

	if err := accountManager.RemoveAccount(email); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Removed account: %s\n", email)
}

func authLoginGoogleAntigravityMulti() {
	cfg := auth.GoogleAntigravityOAuthConfig()

	cred, err := auth.LoginBrowser(cfg)
	if err != nil {
		fmt.Printf("Login failed: %v\n", err)
		os.Exit(1)
	}

	cred.Provider = "google-antigravity"

	// Fetch user email from Google userinfo
	email, err := fetchGoogleUserEmail(cred.AccessToken)
	if err != nil {
		fmt.Printf("Warning: could not fetch email: %v\n", err)
	} else {
		cred.Email = email
		fmt.Printf("Email: %s\n", email)
	}

	// Fetch Cloud Code Assist project ID
	projectID, err := providers.FetchAntigravityProjectID(cred.AccessToken)
	if err != nil {
		fmt.Printf("Warning: could not fetch project ID: %v\n", err)
		fmt.Println("You may need Google Cloud Code Assist enabled on your account.")
	} else {
		cred.ProjectID = projectID
		fmt.Printf("Project: %s\n", projectID)
	}

	// Add to multi-account store
	home, _ := os.UserHomeDir()
	configDir := filepath.Join(home, ".picoclaw")
	accountManager := antigravity.NewProvider(configDir).GetAccountManager()

	account := antigravity.Account{
		Email:        cred.Email,
		RefreshToken: cred.RefreshToken,
		AccessToken:  cred.AccessToken,
		ProjectID:    cred.ProjectID,
		ExpiresAt:    cred.ExpiresAt,
	}

	if err := accountManager.AddAccount(account); err != nil {
		fmt.Printf("Failed to save account: %v\n", err)
		os.Exit(1)
	}

	// Also save to legacy credential store for backward compatibility
	if err := auth.SetCredential("google-antigravity", cred); err != nil {
		fmt.Printf("Warning: could not update legacy credential store: %v\n", err)
	}

	appCfg, err := loadConfig()
	if err == nil {
		// Update or add antigravity in ModelList
		foundAntigravity := false
		for i := range appCfg.ModelList {
			if isAntigravityModel(appCfg.ModelList[i].Model) {
				appCfg.ModelList[i].AuthMethod = "oauth"
				foundAntigravity = true
				break
			}
		}

		if !foundAntigravity {
			appCfg.ModelList = append(appCfg.ModelList, config.ModelConfig{
				ModelName:  "gemini-flash",
				Model:      "antigravity/gemini-3-flash",
				AuthMethod: "oauth",
			})
		}

		appCfg.Agents.Defaults.Model = "gemini-flash"

		if err := config.SaveConfig(getConfigPath(), appCfg); err != nil {
			fmt.Printf("Warning: could not update config: %v\n", err)
		}
	}

	fmt.Println("\n✓ Google Antigravity login successful!")
	accountCount := accountManager.AccountCount()
	fmt.Printf("Total accounts: %d\n", accountCount)
	fmt.Println("Default model set to: gemini-flash")
	fmt.Println("Run again to add more accounts for higher quotas!")
}

type ProviderInfo struct {
	ID          string
	Name        string
	Description string
	AuthType    string
}

var availableProviders = []ProviderInfo{
	{
		ID:          "opencode",
		Name:        "OpenCode Zen",
		Description: "Access GPT, Claude, Gemini, GLM, Kimi, MiniMax and more",
		AuthType:    "api_key",
	},
}

func authSetupCmd() {
	fmt.Println("\n╔═══════════════════════════════════════════╗")
	fmt.Println("║       PicoClaw Auth Setup                ║")
	fmt.Println("╚═══════════════════════════════════════════╝")
	fmt.Println()

	fmt.Println("Available Providers:")
	fmt.Println("---------------------")
	for i, p := range availableProviders {
		authType := "API Key"
		if p.AuthType == "oauth" {
			authType = "OAuth"
		}
		fmt.Printf("  [%d] %s (%s)\n", i+1, p.Name, authType)
		fmt.Printf("      %s\n", p.Description)
	}
	fmt.Println()

	fmt.Print("Enter provider numbers to configure (comma-separated, e.g., 1,2): ")
	var selection string
	fmt.Scanln(&selection)

	selected := parseSelection(selection, len(availableProviders))
	if len(selected) == 0 {
		fmt.Println("No providers selected. Exiting.")
		return
	}

	fmt.Println()
	fmt.Println(strings.Repeat("─", 40))
	fmt.Println()

	for _, idx := range selected {
		p := availableProviders[idx-1]
		fmt.Printf("Configuring: %s\n", p.Name)

		if p.AuthType == "api_key" {
			configureAPIKeyProvider(p)
		} else if p.AuthType == "oauth" {
			configureOAuthProvider(p)
		}
		fmt.Println()
	}

	fmt.Println("✓ Setup complete!")
	fmt.Println("\nRun 'picoclaw auth status' to verify your configuration.")
}

func parseSelection(input string, max int) []int {
	var result []int
	input = strings.ReplaceAll(input, " ", "")
	parts := strings.Split(input, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		var num int
		fmt.Sscanf(part, "%d", &num)
		if num >= 1 && num <= max {
			exists := false
			for _, v := range result {
				if v == num {
					exists = true
					break
				}
			}
			if !exists {
				result = append(result, num)
			}
		}
	}
	return result
}

func configureAPIKeyProvider(p ProviderInfo) {
	fmt.Printf("  API Key: ")
	var apiKey string
	fmt.Scanln(&apiKey)
	apiKey = strings.TrimSpace(apiKey)

	if apiKey == "" {
		fmt.Println("  ✗ API key cannot be empty")
		return
	}

	appCfg, err := loadConfig()
	if err != nil {
		fmt.Printf("  ✗ Failed to load config: %v\n", err)
		return
	}

	switch p.ID {
	case "opencode":
		found := false
		for i := range appCfg.ModelList {
			if strings.HasPrefix(appCfg.ModelList[i].Model, "opencode/") {
				appCfg.ModelList[i].APIKey = apiKey
				appCfg.ModelList[i].AuthMethod = "token"
				found = true
				break
			}
		}
		if !found {
			appCfg.ModelList = append(appCfg.ModelList, config.ModelConfig{
				ModelName:  "opencode-qwen3",
				Model:      "opencode/qwen3-coder",
				APIKey:     apiKey,
				AuthMethod: "token",
			})
		}
		if appCfg.Agents.Defaults.Model == "" {
			appCfg.Agents.Defaults.Model = "opencode-qwen3"
		}
		fmt.Printf("  ✓ API key saved for %s\n", p.Name)
	}

	if err := config.SaveConfig(getConfigPath(), appCfg); err != nil {
		fmt.Printf("  ✗ Failed to save config: %v\n", err)
		return
	}
}

func configureOAuthProvider(p ProviderInfo) {
	fmt.Printf("  Starting OAuth flow for %s...\n", p.Name)
	fmt.Println("  (OAuth not yet implemented for interactive setup)")
}
