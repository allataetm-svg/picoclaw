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
)

const (
	opencodeBaseURL      = "https://api.opencode.ai/v1"
	opencodeDefaultModel = "qwen-3-coder-480b"
	opencodeUserAgent    = "picoclaw/1.0"
)

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

	reqBody := p.buildOpenAIRequest(messages, tools, model, options)
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	apiURL := fmt.Sprintf("%s/chat/completions", p.apiBase)
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
		return nil, fmt.Errorf("parsing opencode response: %w", err)
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
