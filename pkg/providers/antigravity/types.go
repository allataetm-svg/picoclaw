package antigravity

import (
	"time"
)

const (
	BaseURL      = "https://cloudcode-pa.googleapis.com"
	DefaultModel = "gemini-3-flash"
	UserAgent    = "antigravity"
	XGoogClient  = "google-cloud-sdk vscode_cloudshelleditor/0.1"
	Version      = "1.15.8"
	AccountsFile = "antigravity-accounts.json"
	ConfigFile   = "antigravity.json"
)

type Account struct {
	Email        string    `json:"email"`
	RefreshToken string    `json:"refresh_token"`
	AccessToken  string    `json:"access_token,omitempty"`
	ProjectID    string    `json:"project_id,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
	Disabled     bool      `json:"disabled,omitempty"`

	// Runtime fields (not persisted)
	rateLimitedUntil time.Time
	failureCount     int
	lastFailure      time.Time
}

type AccountsConfig struct {
	Accounts                 []Account `json:"accounts"`
	ActiveAccountIndex       int       `json:"active_account_index,omitempty"`
	AccountSelectionStrategy string    `json:"account_selection_strategy,omitempty"` // sticky, round-robin, hybrid
	CLIFirst                 bool      `json:"cli_first,omitempty"`
}

type ProviderConfig struct {
	KeepThinking             bool   `json:"keep_thinking,omitempty"`
	SessionRecovery          bool   `json:"session_recovery,omitempty"`
	CLIFirst                 bool   `json:"cli_first,omitempty"`
	QuietMode                bool   `json:"quiet_mode,omitempty"`
	Debug                    bool   `json:"debug,omitempty"`
	DebugTUI                 bool   `json:"debug_tui,omitempty"`
	AutoUpdate               bool   `json:"auto_update,omitempty"`
	AccountSelectionStrategy string `json:"account_selection_strategy,omitempty"` // sticky, round-robin, hybrid, performance_first
	MaxCacheFirstWaitSec     int    `json:"max_cache_first_wait_seconds,omitempty"`
	FailureTTLSec            int    `json:"failure_ttl_seconds,omitempty"`
	SchedulingMode           string `json:"scheduling_mode,omitempty"` // cache_first, balance, performance_first
	SoftQuotaThreshold       int    `json:"soft_quota_threshold_percent,omitempty"`
	QuotaRefreshInterval     int    `json:"quota_refresh_interval_minutes,omitempty"`
	SoftQuotaCacheTTL        string `json:"soft_quota_cache_ttl_minutes,omitempty"`
	PIDOffsetEnabled         bool   `json:"pid_offset_enabled,omitempty"`
}

func DefaultProviderConfig() ProviderConfig {
	return ProviderConfig{
		KeepThinking:             false,
		SessionRecovery:          true,
		CLIFirst:                 false,
		QuietMode:                false,
		Debug:                    false,
		DebugTUI:                 false,
		AutoUpdate:               true,
		AccountSelectionStrategy: "hybrid",
		MaxCacheFirstWaitSec:     60,
		FailureTTLSec:            3600,
		SchedulingMode:           "cache_first",
		SoftQuotaThreshold:       90,
		QuotaRefreshInterval:     15,
		SoftQuotaCacheTTL:        "auto",
		PIDOffsetEnabled:         false,
	}
}

type ThinkingConfig struct {
	Budget  int    `json:"budget,omitempty"` // For Claude models (tokens)
	Level   string `json:"level,omitempty"`  // For Gemini: minimal, low, medium, high
	Enabled bool   `json:"enabled,omitempty"`
}

type ModelInfo struct {
	ID            string  `json:"id"`
	DisplayName   string  `json:"display_name"`
	IsExhausted   bool    `json:"is_exhausted"`
	RemainingFrac float64 `json:"remaining_fraction,omitempty"`
	ResetTime     string  `json:"reset_time,omitempty"`
}

type QuotaInfo struct {
	ModelID           string    `json:"model_id"`
	IsExhausted       bool      `json:"is_exhausted"`
	RemainingFraction float64   `json:"remaining_fraction"`
	ResetTime         time.Time `json:"reset_time,omitempty"`
	RateLimitedUntil  time.Time `json:"rate_limited_until,omitempty"`
}

type SearchConfig struct {
	Enabled bool   `json:"enabled,omitempty"`
	Mode    string `json:"mode,omitempty"` // auto, always
}

type SchedulingMode string

const (
	SchedulingCacheFirst       SchedulingMode = "cache_first"
	SchedulingBalance          SchedulingMode = "balance"
	SchedulingPerformanceFirst SchedulingMode = "performance_first"
)

type AccountSelectionStrategy string

const (
	StrategySticky      AccountSelectionStrategy = "sticky"
	StrategyRoundRobin  AccountSelectionStrategy = "round-robin"
	StrategyHybrid      AccountSelectionStrategy = "hybrid"
	StrategyPerformance AccountSelectionStrategy = "performance_first"
)

type EndpointType string

const (
	EndpointAntigravity EndpointType = "antigravity"
	EndpointGeminiCLI   EndpointType = "gemini_cli"
)

func (a *Account) IsExpired() bool {
	if a.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(a.ExpiresAt)
}

func (a *Account) NeedsRefresh() bool {
	if a.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().Add(5 * time.Minute).After(a.ExpiresAt)
}

func (a *Account) IsRateLimited() bool {
	return a.rateLimitedUntil.After(time.Now())
}

func (a *Account) SetRateLimited(d time.Duration) {
	a.rateLimitedUntil = time.Now().Add(d)
}

func (a *Account) RateLimitedUntil() time.Time {
	return a.rateLimitedUntil
}

func (a *Account) IncrementFailure() {
	a.failureCount++
	a.lastFailure = time.Now()
}

func (a *Account) ResetFailures() {
	a.failureCount = 0
	a.lastFailure = time.Time{}
}

func (a *Account) FailureScore() int {
	return a.failureCount
}

var DefaultModels = []ModelInfo{
	{ID: "gemini-3-flash", DisplayName: "Gemini 3 Flash"},
	{ID: "gemini-3-flash-preview", DisplayName: "Gemini 3 Flash Preview"},
	{ID: "gemini-3-pro", DisplayName: "Gemini 3 Pro"},
	{ID: "gemini-3-pro-preview", DisplayName: "Gemini 3 Pro Preview"},
	{ID: "gemini-2.5-flash", DisplayName: "Gemini 2.5 Flash"},
	{ID: "gemini-2.5-pro", DisplayName: "Gemini 2.5 Pro"},
	{ID: "claude-sonnet-4.6", DisplayName: "Claude Sonnet 4.6"},
	{ID: "claude-opus-4.6-thinking", DisplayName: "Claude Opus 4.6 Thinking"},
}

var ThinkingModels = map[string]bool{
	"gemini-3-pro":             true,
	"gemini-3-pro-preview":     true,
	"gemini-3-flash":           true,
	"gemini-3-flash-preview":   true,
	"claude-opus-4.6-thinking": true,
}

func IsThinkingModel(modelID string) bool {
	return ThinkingModels[modelID]
}
