package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/logger"
)

const (
	opencodeBaseURL      = "https://opencode.ai/zen/v1"
	opencodeDefaultModel = "glm-5-free"
	opencodeUserAgent    = "picoclaw/1.0"
)

var opencodeModels = map[string]OpenCodeModel{
	// GPT Models (uses /v1/responses)
	"gpt-5.2":            {ID: "gpt-5.2", Name: "GPT 5.2", Endpoint: "/v1/responses", APIPackage: "@ai-sdk/openai"},
	"gpt-5.2-codex":      {ID: "gpt-5.2-codex", Name: "GPT 5.2 Codex", Endpoint: "/v1/responses", APIPackage: "@ai-sdk/openai"},
	"gpt-5.1":            {ID: "gpt-5.1", Name: "GPT 5.1", Endpoint: "/v1/responses", APIPackage: "@ai-sdk/openai"},
	"gpt-5.1-codex":      {ID: "gpt-5.1-codex", Name: "GPT 5.1 Codex", Endpoint: "/v1/responses", APIPackage: "@ai-sdk/openai"},
	"gpt-5.1-codex-max":  {ID: "gpt-5.1-codex-max", Name: "GPT 5.1 Codex Max", Endpoint: "/v1/responses", APIPackage: "@ai-sdk/openai"},
	"gpt-5.1-codex-mini": {ID: "gpt-5.1-codex-mini", Name: "GPT 5.1 Codex Mini", Endpoint: "/v1/responses", APIPackage: "@ai-sdk/openai"},
	"gpt-5":              {ID: "gpt-5", Name: "GPT 5", Endpoint: "/v1/responses", APIPackage: "@ai-sdk/openai"},
	"gpt-5-codex":        {ID: "gpt-5-codex", Name: "GPT 5 Codex", Endpoint: "/v1/responses", APIPackage: "@ai-sdk/openai"},
	"gpt-5-nano":         {ID: "gpt-5-nano", Name: "GPT 5 Nano", Endpoint: "/v1/responses", APIPackage: "@ai-sdk/openai", Free: true},

	// Claude Models (uses /v1/messages)
	"claude-opus-4-6":   {ID: "claude-opus-4-6", Name: "Claude Opus 4.6", Endpoint: "/v1/messages", APIPackage: "@ai-sdk/anthropic"},
	"claude-opus-4-5":   {ID: "claude-opus-4-5", Name: "Claude Opus 4.5", Endpoint: "/v1/messages", APIPackage: "@ai-sdk/anthropic"},
	"claude-opus-4-1":   {ID: "claude-opus-4-1", Name: "Claude Opus 4.1", Endpoint: "/v1/messages", APIPackage: "@ai-sdk/anthropic"},
	"claude-sonnet-4-6": {ID: "claude-sonnet-4-6", Name: "Claude Sonnet 4.6", Endpoint: "/v1/messages", APIPackage: "@ai-sdk/anthropic"},
	"claude-sonnet-4-5": {ID: "claude-sonnet-4-5", Name: "Claude Sonnet 4.5", Endpoint: "/v1/messages", APIPackage: "@ai-sdk/anthropic"},
	"claude-sonnet-4":   {ID: "claude-sonnet-4", Name: "Claude Sonnet 4", Endpoint: "/v1/messages", APIPackage: "@ai-sdk/anthropic"},
	"claude-haiku-4-5":  {ID: "claude-haiku-4-5", Name: "Claude Haiku 4.5", Endpoint: "/v1/messages", APIPackage: "@ai-sdk/anthropic"},
	"claude-3-5-haiku":  {ID: "claude-3-5-haiku", Name: "Claude Haiku 3.5", Endpoint: "/v1/messages", APIPackage: "@ai-sdk/anthropic"},

	// Gemini Models (uses /v1/models/{model})
	"gemini-3.1-pro": {ID: "gemini-3.1-pro", Name: "Gemini 3.1 Pro", Endpoint: "/v1/models/gemini-3.1-pro", APIPackage: "@ai-sdk/google"},
	"gemini-3-pro":   {ID: "gemini-3-pro", Name: "Gemini 3 Pro", Endpoint: "/v1/models/gemini-3-pro", APIPackage: "@ai-sdk/google"},
	"gemini-3-flash": {ID: "gemini-3-flash", Name: "Gemini 3 Flash", Endpoint: "/v1/models/gemini-3-flash", APIPackage: "@ai-sdk/google"},

	// OpenAI-Compatible Models (uses /v1/chat/completions)
	"minimax-m2.5":      {ID: "minimax-m2.5", Name: "MiniMax M2.5", Endpoint: "/v1/chat/completions", APIPackage: "@ai-sdk/openai-compatible"},
	"minimax-m2.5-free": {ID: "minimax-m2.5-free", Name: "MiniMax M2.5 Free", Endpoint: "/v1/chat/completions", APIPackage: "@ai-sdk/openai-compatible", Free: true},
	"minimax-m2.1":      {ID: "minimax-m2.1", Name: "MiniMax M2.1", Endpoint: "/v1/chat/completions", APIPackage: "@ai-sdk/openai-compatible"},
	"glm-5":             {ID: "glm-5", Name: "GLM 5", Endpoint: "/v1/chat/completions", APIPackage: "@ai-sdk/openai-compatible"},
	"glm-5-free":        {ID: "glm-5-free", Name: "GLM 5 Free", Endpoint: "/v1/chat/completions", APIPackage: "@ai-sdk/openai-compatible", Free: true},
	"glm-4.7":           {ID: "glm-4.7", Name: "GLM 4.7", Endpoint: "/v1/chat/completions", APIPackage: "@ai-sdk/openai-compatible"},
	"glm-4.6":           {ID: "glm-4.6", Name: "GLM 4.6", Endpoint: "/v1/chat/completions", APIPackage: "@ai-sdk/openai-compatible"},
	"kimi-k2.5":         {ID: "kimi-k2.5", Name: "Kimi K2.5", Endpoint: "/v1/chat/completions", APIPackage: "@ai-sdk/openai-compatible"},
	"kimi-k2.5-free":    {ID: "kimi-k2.5-free", Name: "Kimi K2.5 Free", Endpoint: "/v1/chat/completions", APIPackage: "@ai-sdk/openai-compatible", Free: true},
	"kimi-k2-thinking":  {ID: "kimi-k2-thinking", Name: "Kimi K2 Thinking", Endpoint: "/v1/chat/completions", APIPackage: "@ai-sdk/openai-compatible"},
	"kimi-k2":           {ID: "kimi-k2", Name: "Kimi K2", Endpoint: "/v1/chat/completions", APIPackage: "@ai-sdk/openai-compatible"},
	"qwen3-coder":       {ID: "qwen3-coder", Name: "Qwen3 Coder 480B", Endpoint: "/v1/chat/completions", APIPackage: "@ai-sdk/openai-compatible"},
	"big-pickle":        {ID: "big-pickle", Name: "Big Pickle", Endpoint: "/v1/chat/completions", APIPackage: "@ai-sdk/openai-compatible", Free: true},
}

type OpenCodeModel struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Endpoint   string `json:"endpoint"`
	APIPackage string `json:"api_package"`
	Free       bool   `json:"free"`
}

type OpenCodeProvider struct {
	apiKey     string
	apiBase    string
	httpClient *http.Client
}

func NewOpenCodeProvider(apiKey, apiBase string) *OpenCodeProvider {
	if apiBase == "" {
		apiBase = opencodeBaseURL
	}
	return &OpenCodeProvider{
		apiKey:  apiKey,
		apiBase: apiBase,
		httpClient: &http.Client{
			Timeout: 180 * time.Second,
		},
	}
}

func (p *OpenCodeProvider) Chat(
	ctx context.Context,
	messages []Message,
	tools []ToolDefinition,
	model string,
	options map[string]any,
) (*LLMResponse, error) {
	if model == "" || model == "opencode" {
		model = opencodeDefaultModel
	}
	model = strings.TrimPrefix(model, "opencode/")

	// Get model info for endpoint
	modelInfo := opencodeModels[model]
	if modelInfo.Endpoint == "" {
		modelInfo.Endpoint = "/v1/chat/completions" // Default
	}

	reqBody := p.buildOpenAIRequest(messages, tools, model, options)
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	// Determine endpoint based on model type
	// Most models use /v1/chat/completions, but some need different endpoints
	var apiURL string
	switch {
	case strings.HasPrefix(modelInfo.Endpoint, "/v1/models/"):
		// Gemini models - handled separately
		apiURL = p.apiBase + modelInfo.Endpoint + ":generateContent"
	case modelInfo.Endpoint == "/v1/messages":
		// Claude models - use messages endpoint
		apiURL = p.apiBase + modelInfo.Endpoint
	case modelInfo.Endpoint == "/v1/responses":
		// GPT models - use responses endpoint
		apiURL = p.apiBase + modelInfo.Endpoint
	default:
		// Default to chat/completions (MiniMax, GLM, Kimi, Qwen, Big Pickle)
		apiURL = p.apiBase + "/v1/chat/completions"
	}

	// Debug: log request details
	logger.DebugCF("provider.opencode", "Request", map[string]any{
		"url":    apiURL,
		"model":  model,
		"header": "Bearer " + p.apiKey[:min(10, len(p.apiKey))] + "...",
	})

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", opencodeUserAgent)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("opencode API call: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("opencode API error (HTTP %d): %s", resp.StatusCode, truncateString(string(respBody), 500))
	}

	// Debug: log response for troubleshooting
	logger.DebugCF("provider.opencode", "Response", map[string]any{
		"status": resp.StatusCode,
		"body":   truncateString(string(respBody), 200),
	})

	if strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		return p.parseSSEResponse(string(respBody))
	}

	return p.parseJSONResponse(respBody)
}

func (p *OpenCodeProvider) GetDefaultModel() string {
	return opencodeDefaultModel
}

type openAIRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Tools       []openAITool    `json:"tools,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

type openAIMessage struct {
	Role       string           `json:"role"`
	Content    any              `json:"content"`
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

type openAIToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Function openAIFunctionCall `json:"function"`
}

type openAIFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAITool struct {
	Type     string             `json:"type"`
	Function openAIToolFunction `json:"function"`
}

type openAIToolFunction struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

func (p *OpenCodeProvider) buildOpenAIRequest(
	messages []Message,
	tools []ToolDefinition,
	model string,
	options map[string]any,
) openAIRequest {
	req := openAIRequest{
		Model:    model,
		Messages: make([]openAIMessage, 0, len(messages)),
		Stream:   false,
	}

	for _, msg := range messages {
		oaiMsg := openAIMessage{Role: msg.Role}

		switch msg.Role {
		case "system":
			oaiMsg.Content = msg.Content
		case "user":
			if msg.ToolCallID != "" {
				oaiMsg.Role = "tool"
				oaiMsg.Content = msg.Content
				oaiMsg.ToolCallID = msg.ToolCallID
			} else {
				oaiMsg.Content = buildContent(msg)
			}
		case "assistant":
			oaiMsg.Content = msg.Content
			if len(msg.ToolCalls) > 0 {
				oaiMsg.ToolCalls = make([]openAIToolCall, len(msg.ToolCalls))
				for i, tc := range msg.ToolCalls {
					args := tc.Arguments
					if args == nil && tc.Function != nil {
						args = map[string]any{}
						if tc.Function.Arguments != "" {
							json.Unmarshal([]byte(tc.Function.Arguments), &args)
						}
					}
					argsJSON, _ := json.Marshal(args)
					oaiMsg.ToolCalls[i] = openAIToolCall{
						ID:   tc.ID,
						Type: "function",
						Function: openAIFunctionCall{
							Name:      tc.Name,
							Arguments: string(argsJSON),
						},
					}
				}
			}
		case "tool":
			oaiMsg.Content = msg.Content
			oaiMsg.ToolCallID = msg.ToolCallID
		}

		req.Messages = append(req.Messages, oaiMsg)
	}

	if len(tools) > 0 {
		req.Tools = make([]openAITool, 0, len(tools))
		for _, t := range tools {
			if t.Type == "function" {
				req.Tools = append(req.Tools, openAITool{
					Type: "function",
					Function: openAIToolFunction{
						Name:        t.Function.Name,
						Description: t.Function.Description,
						Parameters:  t.Function.Parameters,
					},
				})
			}
		}
	}

	if maxTokens, ok := options["max_tokens"]; ok {
		switch v := maxTokens.(type) {
		case int:
			req.MaxTokens = v
		case float64:
			req.MaxTokens = int(v)
		}
	}
	if temp, ok := options["temperature"].(float64); ok {
		req.Temperature = temp
	}

	return req
}

func buildContent(msg Message) any {
	return msg.Content
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Role      string           `json:"role"`
			Content   string           `json:"content"`
			ToolCalls []openAIToolCall `json:"tool_calls,omitempty"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

func (p *OpenCodeProvider) parseJSONResponse(body []byte) (*LLMResponse, error) {
	var resp openAIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parsing opencode response: %w (body: %s)", err, truncateString(string(body), 200))
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("opencode: no choices in response")
	}

	choice := resp.Choices[0]
	var toolCalls []ToolCall

	for _, tc := range choice.Message.ToolCalls {
		var args map[string]any
		if tc.Function.Arguments != "" {
			json.Unmarshal([]byte(tc.Function.Arguments), &args)
		}
		toolCalls = append(toolCalls, ToolCall{
			ID:   tc.ID,
			Name: tc.Function.Name,
			Function: &FunctionCall{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
			Arguments: args,
		})
	}

	finishReason := choice.FinishReason
	if len(toolCalls) > 0 {
		finishReason = "tool_calls"
	}

	var usage *UsageInfo
	if resp.Usage.TotalTokens > 0 {
		usage = &UsageInfo{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		}
	}

	return &LLMResponse{
		Content:      choice.Message.Content,
		ToolCalls:    toolCalls,
		FinishReason: finishReason,
		Usage:        usage,
	}, nil
}

func (p *OpenCodeProvider) parseSSEResponse(body string) (*LLMResponse, error) {
	var contentParts []string
	var toolCalls []ToolCall
	var finishReason string
	var usage *UsageInfo

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

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content   string           `json:"content"`
					ToolCalls []openAIToolCall `json:"tool_calls,omitempty"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
			Usage *struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			} `json:"usage"`
		}

		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				contentParts = append(contentParts, choice.Delta.Content)
			}
			for _, tc := range choice.Delta.ToolCalls {
				var args map[string]any
				if tc.Function.Arguments != "" {
					json.Unmarshal([]byte(tc.Function.Arguments), &args)
				}
				toolCalls = append(toolCalls, ToolCall{
					ID:   tc.ID,
					Name: tc.Function.Name,
					Function: &FunctionCall{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
					Arguments: args,
				})
			}
			if choice.FinishReason != "" {
				finishReason = choice.FinishReason
			}
		}

		if chunk.Usage != nil && chunk.Usage.TotalTokens > 0 {
			usage = &UsageInfo{
				PromptTokens:     chunk.Usage.PromptTokens,
				CompletionTokens: chunk.Usage.CompletionTokens,
				TotalTokens:      chunk.Usage.TotalTokens,
			}
		}
	}

	if len(toolCalls) > 0 {
		finishReason = "tool_calls"
	}

	return &LLMResponse{
		Content:      strings.Join(contentParts, ""),
		ToolCalls:    toolCalls,
		FinishReason: finishReason,
		Usage:        usage,
	}, nil
}

func GetOpenCodeModels() []OpenCodeModel {
	models := make([]OpenCodeModel, 0, len(opencodeModels))
	for _, m := range opencodeModels {
		models = append(models, m)
	}
	return models
}

func GetOpenCodeModel(modelID string) *OpenCodeModel {
	if m, ok := opencodeModels[modelID]; ok {
		return &m
	}
	return nil
}
