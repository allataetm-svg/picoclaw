package antigravity

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/sipeed/picoclaw/pkg/logger"
)

const (
	GeminiCLIBaseURL = "https://cloudcode-pa.googleapis.com"
)

type QuotaManager struct {
	mu              sync.RWMutex
	quotaCache      map[string]map[string]*QuotaInfo // email -> modelID -> QuotaInfo
	lastRefresh     map[string]time.Time             // email -> last refresh time
	refreshInterval time.Duration
	httpClient      *http.Client
}

func NewQuotaManager() *QuotaManager {
	return &QuotaManager{
		quotaCache:      make(map[string]map[string]*QuotaInfo),
		lastRefresh:     make(map[string]time.Time),
		refreshInterval: 15 * time.Minute,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (qm *QuotaManager) FetchQuotas(accessToken, projectID, email string) ([]ModelInfo, error) {
	reqBody, _ := json.Marshal(map[string]any{
		"project": projectID,
	})

	req, err := http.NewRequest("POST", BaseURL+"/v1internal:fetchAvailableModels", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("X-Goog-Api-Client", XGoogClient)

	resp, err := qm.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetchAvailableModels failed (HTTP %d): %s", resp.StatusCode, truncateString(string(body), 200))
	}

	var result struct {
		Models map[string]struct {
			DisplayName string `json:"displayName"`
			QuotaInfo   struct {
				RemainingFraction any    `json:"remainingFraction"`
				ResetTime         string `json:"resetTime"`
				IsExhausted       bool   `json:"isExhausted"`
			} `json:"quotaInfo"`
		} `json:"models"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing models response: %w", err)
	}

	qm.mu.Lock()
	defer qm.mu.Unlock()

	if qm.quotaCache[email] == nil {
		qm.quotaCache[email] = make(map[string]*QuotaInfo)
	}

	var models []ModelInfo
	for id, info := range result.Models {
		remainingFrac := parseRemainingFraction(info.QuotaInfo.RemainingFraction)

		quotaInfo := &QuotaInfo{
			ModelID:           id,
			IsExhausted:       info.QuotaInfo.IsExhausted,
			RemainingFraction: remainingFrac,
		}

		if info.QuotaInfo.ResetTime != "" {
			if t, err := time.Parse(time.RFC3339, info.QuotaInfo.ResetTime); err == nil {
				quotaInfo.ResetTime = t
			}
		}

		qm.quotaCache[email][id] = quotaInfo

		models = append(models, ModelInfo{
			ID:            id,
			DisplayName:   info.DisplayName,
			IsExhausted:   info.QuotaInfo.IsExhausted,
			RemainingFrac: remainingFrac,
			ResetTime:     info.QuotaInfo.ResetTime,
		})
	}

	// Ensure default models are in the list
	models = ensureDefaultModels(models)
	qm.lastRefresh[email] = time.Now()

	return models, nil
}

func parseRemainingFraction(v any) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case string:
		var f float64
		fmt.Sscanf(val, "%f", &f)
		return f
	default:
		return 0
	}
}

func ensureDefaultModels(models []ModelInfo) []ModelInfo {
	existing := make(map[string]bool)
	for _, m := range models {
		existing[m.ID] = true
	}

	for _, dm := range DefaultModels {
		if !existing[dm.ID] {
			models = append(models, dm)
		}
	}

	return models
}

func (qm *QuotaManager) GetQuota(email, modelID string) *QuotaInfo {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	if qm.quotaCache[email] == nil {
		return nil
	}
	return qm.quotaCache[email][modelID]
}

func (qm *QuotaManager) IsExhausted(email, modelID string) bool {
	quota := qm.GetQuota(email, modelID)
	if quota == nil {
		return false // Unknown quota, assume available
	}
	return quota.IsExhausted
}

func (qm *QuotaManager) ShouldRefresh(email string) bool {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	last, ok := qm.lastRefresh[email]
	if !ok {
		return true
	}
	return time.Since(last) > qm.refreshInterval
}

func (qm *QuotaManager) MarkExhausted(email, modelID string, resetTime time.Time) {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	if qm.quotaCache[email] == nil {
		qm.quotaCache[email] = make(map[string]*QuotaInfo)
	}

	qm.quotaCache[email][modelID] = &QuotaInfo{
		ModelID:           modelID,
		IsExhausted:       true,
		RemainingFraction: 0,
		ResetTime:         resetTime,
	}
}

func (qm *QuotaManager) GetCacheAge(email string) time.Duration {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	last, ok := qm.lastRefresh[email]
	if !ok {
		return 0
	}
	return time.Since(last)
}

type DualQuotaManager struct {
	antigravityQM *QuotaManager
	geminiCLIQM   *QuotaManager
	cliFirst      bool
}

func NewDualQuotaManager(cliFirst bool) *DualQuotaManager {
	return &DualQuotaManager{
		antigravityQM: NewQuotaManager(),
		geminiCLIQM:   NewQuotaManager(),
		cliFirst:      cliFirst,
	}
}

func (dqm *DualQuotaManager) SelectEndpoint(modelID string, account *Account) (EndpointType, error) {
	isGeminiModel := strings.Contains(modelID, "gemini")
	isClaudeModel := strings.Contains(modelID, "claude")

	// Claude models always use Antigravity
	if isClaudeModel {
		return EndpointAntigravity, nil
	}

	// Non-Gemini models use Antigravity
	if !isGeminiModel {
		return EndpointAntigravity, nil
	}

	// Gemini models: check CLI first if configured
	if dqm.cliFirst {
		// Check if CLI quota is available
		if !dqm.geminiCLIQM.IsExhausted(account.Email, modelID) {
			return EndpointGeminiCLI, nil
		}
		// Fall back to Antigravity
		if !dqm.antigravityQM.IsExhausted(account.Email, modelID) {
			return EndpointAntigravity, nil
		}
	} else {
		// Antigravity first (default)
		if !dqm.antigravityQM.IsExhausted(account.Email, modelID) {
			return EndpointAntigravity, nil
		}
		// Fall back to CLI
		if !dqm.geminiCLIQM.IsExhausted(account.Email, modelID) {
			return EndpointGeminiCLI, nil
		}
	}

	return EndpointAntigravity, fmt.Errorf("both quota pools exhausted for model %s", modelID)
}

func (dqm *DualQuotaManager) MarkExhausted(email, modelID string, endpoint EndpointType, resetTime time.Time) {
	switch endpoint {
	case EndpointAntigravity:
		dqm.antigravityQM.MarkExhausted(email, modelID, resetTime)
	case EndpointGeminiCLI:
		dqm.geminiCLIQM.MarkExhausted(email, modelID, resetTime)
	}
}

func (dqm *DualQuotaManager) FetchAllQuotas(accessToken, projectID, email string) (antigravity, geminiCLI []ModelInfo, err error) {
	// Fetch Antigravity quotas
	antigravity, err = dqm.antigravityQM.FetchQuotas(accessToken, projectID, email)
	if err != nil {
		logger.WarnCF("antigravity.quota", "Failed to fetch Antigravity quotas", map[string]any{
			"email": email,
			"error": err.Error(),
		})
	}

	// For Gemini CLI, we use the same API but potentially different project
	// In practice, the CLI quota is tied to the same Google account
	geminiCLI, err = dqm.geminiCLIQM.FetchQuotas(accessToken, projectID, email)
	if err != nil {
		logger.WarnCF("antigravity.quota", "Failed to fetch Gemini CLI quotas", map[string]any{
			"email": email,
			"error": err.Error(),
		})
	}

	return antigravity, geminiCLI, nil
}

func (dqm *DualQuotaManager) GetQuotaSummary(email string) map[string]QuotaInfo {
	dqm.antigravityQM.mu.RLock()
	defer dqm.antigravityQM.mu.RUnlock()

	result := make(map[string]QuotaInfo)
	if dqm.antigravityQM.quotaCache[email] != nil {
		for modelID, info := range dqm.antigravityQM.quotaCache[email] {
			result[modelID] = *info
		}
	}

	return result
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
