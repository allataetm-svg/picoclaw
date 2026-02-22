package antigravity

import (
	"encoding/json"
	"fmt"
	"strings"
)

type ThinkingBudget struct {
	// For Claude models: budget in tokens (e.g., 8192, 32768)
	BudgetTokens int `json:"budget_tokens,omitempty"`

	// For Gemini models: thinking level
	Level string `json:"level,omitempty"` // minimal, low, medium, high
}

var VariantThinkingBudgets = map[string]ThinkingBudget{
	// Gemini variants
	"minimal": {Level: "minimal"},
	"low":     {Level: "low"},
	"medium":  {Level: "medium"},
	"high":    {Level: "high"},

	// Claude variants
	"thinking-low": {BudgetTokens: 8192},
	"thinking-max": {BudgetTokens: 32768},
	"max":          {BudgetTokens: 32768},
}

var ModelThinkingDefaults = map[string]ThinkingBudget{
	"gemini-3-pro":             {Level: "medium"},
	"gemini-3-pro-preview":     {Level: "medium"},
	"gemini-3.1-pro":           {Level: "medium"},
	"gemini-3.1-pro-preview":   {Level: "medium"},
	"gemini-3-flash":           {Level: "low"},
	"gemini-3-flash-preview":   {Level: "low"},
	"claude-opus-4.6-thinking": {BudgetTokens: 8192},
}

func GetThinkingConfig(modelID, variant string) ThinkingBudget {
	// First check if variant specifies thinking
	if variant != "" {
		if budget, ok := VariantThinkingBudgets[variant]; ok {
			return budget
		}
	}

	// Fall back to model default
	if budget, ok := ModelThinkingDefaults[modelID]; ok {
		return budget
	}

	return ThinkingBudget{}
}

func IsClaudeThinkingModel(modelID string) bool {
	return strings.Contains(modelID, "claude") && strings.Contains(modelID, "thinking")
}

func IsGeminiThinkingModel(modelID string) bool {
	return strings.Contains(modelID, "gemini-3")
}

func AddThinkingToRequest(req map[string]any, modelID string, budget ThinkingBudget) error {
	if budget.BudgetTokens == 0 && budget.Level == "" {
		// Check for model default
		budget = ModelThinkingDefaults[modelID]
		if budget.BudgetTokens == 0 && budget.Level == "" {
			return nil
		}
	}

	if IsClaudeThinkingModel(modelID) && budget.BudgetTokens > 0 {
		// Claude models use thinkingConfig
		req["thinkingConfig"] = map[string]any{
			"thinkingBudget": budget.BudgetTokens,
		}
	} else if IsGeminiThinkingModel(modelID) && budget.Level != "" {
		// Gemini models use thinkingLevel
		req["thinkingLevel"] = budget.Level
	}

	return nil
}

func ParseThinkingFromJSON(data []byte) (string, map[string]any) {
	var resp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text             string `json:"text"`
					ThoughtSignature string `json:"thoughtSignature"`
					Thought          string `json:"thought"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return "", nil
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", nil
	}

	var thoughtContent string
	var thoughtSig map[string]any

	for _, part := range resp.Candidates[0].Content.Parts {
		if part.Thought != "" {
			thoughtContent = part.Thought
		}
		if part.ThoughtSignature != "" {
			thoughtSig = map[string]any{
				"thoughtSignature": part.ThoughtSignature,
			}
		}
	}

	return thoughtContent, thoughtSig
}

type ThoughtBlock struct {
	Signature string `json:"signature"`
	Content   string `json:"content,omitempty"`
}

func ExtractThoughtSignature(part map[string]any) string {
	if sig, ok := part["thoughtSignature"].(string); ok {
		return sig
	}
	if sig, ok := part["thought_signature"].(string); ok {
		return sig
	}
	return ""
}

func BuildThinkingPart(thoughtSignature, content string) map[string]any {
	part := map[string]any{}
	if thoughtSignature != "" {
		part["thoughtSignature"] = thoughtSignature
	}
	if content != "" {
		part["text"] = content
	}
	return part
}

func GetModelVariants(modelID string) []string {
	switch {
	case strings.Contains(modelID, "gemini-3-flash"):
		return []string{"minimal", "low", "medium", "high"}
	case strings.Contains(modelID, "gemini-3-pro"), strings.Contains(modelID, "gemini-3.1-pro"):
		return []string{"low", "high"}
	case strings.Contains(modelID, "claude-opus") && strings.Contains(modelID, "thinking"):
		return []string{"low", "max"}
	default:
		return nil
	}
}

func FormatThinkingLevel(level string) (string, error) {
	level = strings.ToLower(strings.TrimSpace(level))

	validLevels := map[string]bool{
		"minimal": true,
		"low":     true,
		"medium":  true,
		"high":    true,
	}

	if !validLevels[level] {
		return "", fmt.Errorf("invalid thinking level: %s (valid: minimal, low, medium, high)", level)
	}

	return level, nil
}

func FormatThinkingBudget(budget int) (int, error) {
	if budget <= 0 {
		return 0, fmt.Errorf("thinking budget must be positive")
	}
	if budget > 65536 {
		return 65536, nil
	}
	return budget, nil
}
