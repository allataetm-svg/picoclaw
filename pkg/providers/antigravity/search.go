package antigravity

import (
	"strings"
)

type SearchMode string

const (
	SearchModeAuto   SearchMode = "auto"
	SearchModeAlways SearchMode = "always"
	SearchModeNone   SearchMode = "none"
)

type GroundingConfig struct {
	Mode SearchMode `json:"mode,omitempty"`
}

var SearchCapableModels = map[string]bool{
	"gemini-2.5-flash":       true,
	"gemini-2.5-pro":         true,
	"gemini-3-flash":         true,
	"gemini-3-flash-preview": true,
	"gemini-3-pro":           true,
	"gemini-3-pro-preview":   true,
	"gemini-3.1-pro":         true,
	"gemini-3.1-pro-preview": true,
}

func SupportsSearch(modelID string) bool {
	return SearchCapableModels[modelID] ||
		strings.HasPrefix(modelID, "gemini-2") ||
		strings.HasPrefix(modelID, "gemini-3")
}

func AddSearchGrounding(req map[string]any, modelID string, config GroundingConfig) {
	if !SupportsSearch(modelID) {
		return
	}

	if config.Mode == "" || config.Mode == SearchModeNone {
		return
	}

	// Add tools configuration for Google Search grounding
	tools := req["tools"].([]any)
	if tools == nil {
		tools = []any{}
	}

	// Google Search grounding tool
	searchTool := map[string]any{
		"googleSearchRetrieval": map[string]any{
			"dynamicRetrievalConfig": map[string]any{
				"mode": string(config.Mode),
			},
		},
	}

	tools = append(tools, searchTool)
	req["tools"] = tools
}

func ParseSearchMode(s string) SearchMode {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "auto":
		return SearchModeAuto
	case "always":
		return SearchModeAlways
	case "none", "":
		return SearchModeNone
	default:
		return SearchModeAuto
	}
}

func DefaultSearchConfig() GroundingConfig {
	return GroundingConfig{
		Mode: SearchModeNone,
	}
}

func BuildSearchTool(mode SearchMode) map[string]any {
	if mode == SearchModeNone {
		return nil
	}

	return map[string]any{
		"googleSearchRetrieval": map[string]any{
			"dynamicRetrievalConfig": map[string]any{
				"mode": string(mode),
			},
		},
	}
}
