package antigravity

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/auth"
	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/providers/protocoltypes"
)

type Provider struct {
	accountManager *AccountManager
	quotaManager   *DualQuotaManager
	config         ProviderConfig
	httpClient     *http.Client
}

func NewProvider(configDir string) *Provider {
	cfg := DefaultProviderConfig()

	// Load provider config if exists
	cfgPath := configDir + "/" + ConfigFile
	if data, err := os.ReadFile(cfgPath); err == nil {
		var loadedCfg ProviderConfig
		if json.Unmarshal(data, &loadedCfg) == nil {
			cfg = mergeConfig(cfg, loadedCfg)
		}
	}

	return &Provider{
		accountManager: NewAccountManager(configDir, cfg),
		quotaManager:   NewDualQuotaManager(cfg.CLIFirst),
		config:         cfg,
		httpClient: &http.Client{
			Timeout: 180 * time.Second,
		},
	}
}

func mergeConfig(base, override ProviderConfig) ProviderConfig {
	if override.KeepThinking {
		base.KeepThinking = override.KeepThinking
	}
	if !override.SessionRecovery {
		base.SessionRecovery = override.SessionRecovery
	}
	if override.CLIFirst {
		base.CLIFirst = override.CLIFirst
	}
	if override.AccountSelectionStrategy != "" {
		base.AccountSelectionStrategy = override.AccountSelectionStrategy
	}
	if override.SchedulingMode != "" {
		base.SchedulingMode = override.SchedulingMode
	}
	return base
}

func (p *Provider) Chat(
	ctx context.Context,
	messages []protocoltypes.Message,
	tools []protocoltypes.ToolDefinition,
	model string,
	options map[string]any,
) (*protocoltypes.LLMResponse, error) {
	// Select account
	account, err := p.accountManager.SelectAccount()
	if err != nil {
		return nil, fmt.Errorf("antigravity account selection: %w", err)
	}

	// Refresh token if needed
	if account.NeedsRefresh() && account.RefreshToken != "" {
		if err := p.refreshAccountToken(account); err != nil {
			p.accountManager.MarkFailure(account.Email)
			return nil, fmt.Errorf("refreshing token: %w", err)
		}
	}

	if account.IsExpired() {
		p.accountManager.MarkFailure(account.Email)
		return nil, fmt.Errorf("antigravity credentials expired for %s. Run: picoclaw auth login --provider google-antigravity", account.Email)
	}

	// Get project ID
	projectID := account.ProjectID
	if projectID == "" {
		projectID, err = FetchProjectID(account.AccessToken)
		if err != nil {
			logger.WarnCF("antigravity.provider", "Could not fetch project ID", map[string]any{"error": err.Error()})
			projectID = "rising-fact-p41fc"
		} else {
			p.accountManager.UpdateProjectID(account.Email, projectID)
		}
	}

	// Normalize model ID
	if model == "" || model == "antigravity" || model == "google-antigravity" {
		model = DefaultModel
	}
	model = strings.TrimPrefix(model, "google-antigravity/")
	model = strings.TrimPrefix(model, "antigravity/")

	// Get variant from options
	variant := ""
	if v, ok := options["variant"].(string); ok {
		variant = v
	}

	// Get search config from options
	searchConfig := DefaultSearchConfig()
	if v, ok := options["search"].(string); ok {
		searchConfig.Mode = ParseSearchMode(v)
	}

	logger.DebugCF("antigravity.provider", "Starting chat", map[string]any{
		"model":   model,
		"variant": variant,
		"project": projectID,
		"account": account.Email,
	})

	// Build request
	innerRequest := p.buildRequest(messages, tools, model, variant, searchConfig, options)

	// Wrap in v1internal envelope
	envelope := map[string]any{
		"project":     projectID,
		"model":       model,
		"request":     innerRequest,
		"requestType": "agent",
		"userAgent":   UserAgent,
		"requestId":   fmt.Sprintf("agent-%d-%s", time.Now().UnixMilli(), randomString(9)),
	}

	bodyBytes, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	// Build API URL
	apiURL := fmt.Sprintf("%s/v1internal:streamGenerateContent?alt=sse", BaseURL)

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Set headers
	clientMetadata, _ := json.Marshal(map[string]string{
		"ideType":    "IDE_UNSPECIFIED",
		"platform":   "PLATFORM_UNSPECIFIED",
		"pluginType": "GEMINI",
	})
	req.Header.Set("Authorization", "Bearer "+account.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("User-Agent", fmt.Sprintf("antigravity/%s linux/amd64", Version))
	req.Header.Set("X-Goog-Api-Client", XGoogClient)
	req.Header.Set("Client-Metadata", string(clientMetadata))

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("antigravity API call: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		logger.ErrorCF("antigravity.provider", "API call failed", map[string]any{
			"status_code": resp.StatusCode,
			"response":    string(respBody),
			"model":       model,
		})

		// Handle rate limiting
		if resp.StatusCode == 429 {
			duration := p.extractRateLimitDuration(respBody)
			p.accountManager.MarkRateLimited(account.Email, duration)
			return nil, fmt.Errorf("antigravity rate limited for %s (reset in %v)", account.Email, duration)
		}

		p.accountManager.MarkFailure(account.Email)
		return nil, p.parseError(resp.StatusCode, respBody)
	}

	// Parse SSE response
	llmResp, err := p.parseSSEResponse(string(respBody))
	if err != nil {
		return nil, err
	}

	// Check for empty response
	if llmResp.Content == "" && len(llmResp.ToolCalls) == 0 {
		return nil, fmt.Errorf("antigravity: model returned empty response")
	}

	// Clear failures on success
	p.accountManager.ClearFailures(account.Email)

	return llmResp, nil
}

func (p *Provider) GetDefaultModel() string {
	return DefaultModel
}

func (p *Provider) GetAccountManager() *AccountManager {
	return p.accountManager
}

func (p *Provider) GetQuotaManager() *DualQuotaManager {
	return p.quotaManager
}

type antigravityRequest struct {
	Contents       []antigravityContent     `json:"contents"`
	Tools          []antigravityTool        `json:"tools,omitempty"`
	SystemPrompt   *antigravitySystemPrompt `json:"systemInstruction,omitempty"`
	Config         *antigravityGenConfig    `json:"generationConfig,omitempty"`
	ThinkingConfig map[string]any           `json:"thinkingConfig,omitempty"`
	ThinkingLevel  string                   `json:"thinkingLevel,omitempty"`
}

type antigravityContent struct {
	Role  string            `json:"role"`
	Parts []antigravityPart `json:"parts"`
}

type antigravityPart struct {
	Text                  string                   `json:"text,omitempty"`
	ThoughtSignature      string                   `json:"thoughtSignature,omitempty"`
	ThoughtSignatureSnake string                   `json:"thought_signature,omitempty"`
	FunctionCall          *antigravityFunctionCall `json:"functionCall,omitempty"`
	FunctionResponse      *antigravityFunctionResp `json:"functionResponse,omitempty"`
}

type antigravityFunctionCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

type antigravityFunctionResp struct {
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

type antigravityTool struct {
	FunctionDeclarations []antigravityFuncDecl `json:"functionDeclarations"`
}

type antigravityFuncDecl struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

type antigravitySystemPrompt struct {
	Parts []antigravityPart `json:"parts"`
}

type antigravityGenConfig struct {
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
	Temperature     float64 `json:"temperature,omitempty"`
}

func (p *Provider) buildRequest(
	messages []protocoltypes.Message,
	tools []protocoltypes.ToolDefinition,
	model, variant string,
	searchConfig GroundingConfig,
	options map[string]any,
) antigravityRequest {
	req := antigravityRequest{}
	toolCallNames := make(map[string]string)

	// Build contents from messages
	for _, msg := range messages {
		switch msg.Role {
		case "system":
			req.SystemPrompt = &antigravitySystemPrompt{
				Parts: []antigravityPart{{Text: msg.Content}},
			}
		case "user":
			if msg.ToolCallID != "" {
				toolName := resolveToolResponseName(msg.ToolCallID, toolCallNames)
				req.Contents = append(req.Contents, antigravityContent{
					Role: "user",
					Parts: []antigravityPart{{
						FunctionResponse: &antigravityFunctionResp{
							Name: toolName,
							Response: map[string]any{
								"result": msg.Content,
							},
						},
					}},
				})
			} else {
				req.Contents = append(req.Contents, antigravityContent{
					Role:  "user",
					Parts: []antigravityPart{{Text: msg.Content}},
				})
			}
		case "assistant":
			content := antigravityContent{Role: "model"}
			if msg.Content != "" {
				content.Parts = append(content.Parts, antigravityPart{Text: msg.Content})
			}
			for _, tc := range msg.ToolCalls {
				toolName, toolArgs, thoughtSig := normalizeToolCall(tc)
				if toolName == "" {
					continue
				}
				if tc.ID != "" {
					toolCallNames[tc.ID] = toolName
				}
				content.Parts = append(content.Parts, antigravityPart{
					ThoughtSignature:      thoughtSig,
					ThoughtSignatureSnake: thoughtSig,
					FunctionCall: &antigravityFunctionCall{
						Name: toolName,
						Args: toolArgs,
					},
				})
			}
			if len(content.Parts) > 0 {
				req.Contents = append(req.Contents, content)
			}
		case "tool":
			toolName := resolveToolResponseName(msg.ToolCallID, toolCallNames)
			req.Contents = append(req.Contents, antigravityContent{
				Role: "user",
				Parts: []antigravityPart{{
					FunctionResponse: &antigravityFunctionResp{
						Name: toolName,
						Response: map[string]any{
							"result": msg.Content,
						},
					},
				}},
			})
		}
	}

	// Build tools
	if len(tools) > 0 {
		var funcDecls []antigravityFuncDecl
		for _, t := range tools {
			if t.Type == "function" {
				params := sanitizeSchema(t.Function.Parameters)
				funcDecls = append(funcDecls, antigravityFuncDecl{
					Name:        t.Function.Name,
					Description: t.Function.Description,
					Parameters:  params,
				})
			}
		}
		if len(funcDecls) > 0 {
			req.Tools = []antigravityTool{{FunctionDeclarations: funcDecls}}
		}
	}

	// Add search grounding if enabled
	if SupportsSearch(model) && searchConfig.Mode != SearchModeNone {
		searchTool := BuildSearchTool(searchConfig.Mode)
		if searchTool != nil {
			req.Tools = append(req.Tools, antigravityTool{
				FunctionDeclarations: []antigravityFuncDecl{}, // placeholder
			})
		}
	}

	// Add thinking config
	thinkingCfg := GetThinkingConfig(model, variant)
	if thinkingCfg.BudgetTokens > 0 {
		req.ThinkingConfig = map[string]any{
			"thinkingBudget": thinkingCfg.BudgetTokens,
		}
	} else if thinkingCfg.Level != "" {
		req.ThinkingLevel = thinkingCfg.Level
	}

	// Generation config
	config := &antigravityGenConfig{}
	if val, ok := options["max_tokens"]; ok {
		switch v := val.(type) {
		case int:
			config.MaxOutputTokens = v
		case float64:
			config.MaxOutputTokens = int(v)
		}
	}
	if temp, ok := options["temperature"].(float64); ok {
		config.Temperature = temp
	}
	if config.MaxOutputTokens > 0 || config.Temperature > 0 {
		req.Config = config
	}

	return req
}

func normalizeToolCall(tc protocoltypes.ToolCall) (string, map[string]any, string) {
	name := tc.Name
	args := tc.Arguments
	thoughtSig := ""

	if name == "" && tc.Function != nil {
		name = tc.Function.Name
		thoughtSig = tc.Function.ThoughtSignature
	} else if tc.Function != nil {
		thoughtSig = tc.Function.ThoughtSignature
	}

	if args == nil {
		args = map[string]any{}
	}

	if len(args) == 0 && tc.Function != nil && tc.Function.Arguments != "" {
		var parsed map[string]any
		if json.Unmarshal([]byte(tc.Function.Arguments), &parsed) == nil && parsed != nil {
			args = parsed
		}
	}

	return name, args, thoughtSig
}

func resolveToolResponseName(toolCallID string, toolCallNames map[string]string) string {
	if toolCallID == "" {
		return ""
	}
	if name, ok := toolCallNames[toolCallID]; ok && name != "" {
		return name
	}
	return inferToolNameFromCallID(toolCallID)
}

func inferToolNameFromCallID(toolCallID string) string {
	if !strings.HasPrefix(toolCallID, "call_") {
		return toolCallID
	}
	rest := strings.TrimPrefix(toolCallID, "call_")
	if idx := strings.LastIndex(rest, "_"); idx > 0 {
		candidate := rest[:idx]
		if candidate != "" {
			return candidate
		}
	}
	return toolCallID
}

func sanitizeSchema(schema map[string]any) map[string]any {
	if schema == nil {
		return nil
	}

	unsupported := map[string]bool{
		"patternProperties": true, "additionalProperties": true, "$schema": true,
		"$id": true, "$ref": true, "$defs": true, "definitions": true,
		"examples": true, "minLength": true, "maxLength": true,
		"minimum": true, "maximum": true, "multipleOf": true,
		"pattern": true, "format": true, "minItems": true,
		"maxItems": true, "uniqueItems": true, "minProperties": true,
		"maxProperties": true,
	}

	result := make(map[string]any)
	for k, v := range schema {
		if unsupported[k] {
			continue
		}
		switch val := v.(type) {
		case map[string]any:
			result[k] = sanitizeSchema(val)
		case []any:
			sanitized := make([]any, len(val))
			for i, item := range val {
				if m, ok := item.(map[string]any); ok {
					sanitized[i] = sanitizeSchema(m)
				} else {
					sanitized[i] = item
				}
			}
			result[k] = sanitized
		default:
			result[k] = v
		}
	}

	if _, hasProps := result["properties"]; hasProps {
		if _, hasType := result["type"]; !hasType {
			result["type"] = "object"
		}
	}

	return result
}

type antigravityJSONResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text                  string                   `json:"text,omitempty"`
				ThoughtSignature      string                   `json:"thoughtSignature,omitempty"`
				ThoughtSignatureSnake string                   `json:"thought_signature,omitempty"`
				FunctionCall          *antigravityFunctionCall `json:"functionCall,omitempty"`
			} `json:"parts"`
			Role string `json:"role"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}

func (p *Provider) parseSSEResponse(body string) (*protocoltypes.LLMResponse, error) {
	var contentParts []string
	var toolCalls []protocoltypes.ToolCall
	var usage *protocoltypes.UsageInfo
	var finishReason string

	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var sseChunk struct {
			Response antigravityJSONResponse `json:"response"`
		}
		if err := json.Unmarshal([]byte(data), &sseChunk); err != nil {
			continue
		}
		resp := sseChunk.Response

		for _, candidate := range resp.Candidates {
			for _, part := range candidate.Content.Parts {
				if part.Text != "" {
					contentParts = append(contentParts, part.Text)
				}
				if part.FunctionCall != nil {
					argsJSON, _ := json.Marshal(part.FunctionCall.Args)
					thoughtSig := part.ThoughtSignature
					if thoughtSig == "" {
						thoughtSig = part.ThoughtSignatureSnake
					}
					toolCalls = append(toolCalls, protocoltypes.ToolCall{
						ID:        fmt.Sprintf("call_%s_%d", part.FunctionCall.Name, time.Now().UnixNano()),
						Name:      part.FunctionCall.Name,
						Arguments: part.FunctionCall.Args,
						Function: &protocoltypes.FunctionCall{
							Name:             part.FunctionCall.Name,
							Arguments:        string(argsJSON),
							ThoughtSignature: thoughtSig,
						},
					})
				}
			}
			if candidate.FinishReason != "" {
				finishReason = candidate.FinishReason
			}
		}

		if resp.UsageMetadata.TotalTokenCount > 0 {
			usage = &protocoltypes.UsageInfo{
				PromptTokens:     resp.UsageMetadata.PromptTokenCount,
				CompletionTokens: resp.UsageMetadata.CandidatesTokenCount,
				TotalTokens:      resp.UsageMetadata.TotalTokenCount,
			}
		}
	}

	mappedFinish := "stop"
	if len(toolCalls) > 0 {
		mappedFinish = "tool_calls"
	}
	if finishReason == "MAX_TOKENS" {
		mappedFinish = "length"
	}

	return &protocoltypes.LLMResponse{
		Content:      strings.Join(contentParts, ""),
		ToolCalls:    toolCalls,
		FinishReason: mappedFinish,
		Usage:        usage,
	}, nil
}

func (p *Provider) parseError(statusCode int, body []byte) error {
	var errResp struct {
		Error struct {
			Code    int              `json:"code"`
			Message string           `json:"message"`
			Status  string           `json:"status"`
			Details []map[string]any `json:"details"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &errResp); err != nil {
		return fmt.Errorf("antigravity API error (HTTP %d): %s", statusCode, truncateString(string(body), 500))
	}

	return fmt.Errorf("antigravity API error (%s): %s", errResp.Error.Status, errResp.Error.Message)
}

func (p *Provider) extractRateLimitDuration(body []byte) time.Duration {
	var errResp struct {
		Error struct {
			Details []map[string]any `json:"details"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &errResp); err != nil {
		return 60 * time.Second
	}

	for _, detail := range errResp.Error.Details {
		if typeVal, ok := detail["@type"].(string); ok && strings.HasSuffix(typeVal, "ErrorInfo") {
			if metadata, ok := detail["metadata"].(map[string]any); ok {
				if delay, ok := metadata["quotaResetDelay"].(string); ok {
					var d time.Duration
					fmt.Sscanf(delay, "%ds", &d)
					if d > 0 {
						return d * time.Second
					}
				}
			}
		}
	}

	return 60 * time.Second
}

func (p *Provider) refreshAccountToken(account *Account) error {
	cfg := auth.GoogleAntigravityOAuthConfig()

	cred := &auth.AuthCredential{
		RefreshToken: account.RefreshToken,
		Provider:     "google-antigravity",
	}

	refreshed, err := auth.RefreshAccessToken(cred, cfg)
	if err != nil {
		return err
	}

	account.AccessToken = refreshed.AccessToken
	account.ExpiresAt = refreshed.ExpiresAt
	if refreshed.RefreshToken != "" {
		account.RefreshToken = refreshed.RefreshToken
	}

	p.accountManager.UpdateAccessToken(account.Email, account.AccessToken, account.ExpiresAt)

	return nil
}

func FetchProjectID(accessToken string) (string, error) {
	reqBody, _ := json.Marshal(map[string]any{
		"metadata": map[string]any{
			"ideType":    "IDE_UNSPECIFIED",
			"platform":   "PLATFORM_UNSPECIFIED",
			"pluginType": "GEMINI",
		},
	})

	req, err := http.NewRequest("POST", BaseURL+"/v1internal:loadCodeAssist", bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("X-Goog-Api-Client", XGoogClient)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("loadCodeAssist failed: %s", string(body))
	}

	var result struct {
		CloudAICompanionProject string `json:"cloudaicompanionProject"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	if result.CloudAICompanionProject == "" {
		return "", fmt.Errorf("no project ID in loadCodeAssist response")
	}

	return result.CloudAICompanionProject, nil
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
