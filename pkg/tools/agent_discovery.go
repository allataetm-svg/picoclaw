package tools

import (
	"context"
	"fmt"
	"strings"
)

type AgentInfo struct {
	ID           string
	Name         string
	Description  string
	Model        string
	Capabilities []string
	TeamLeader   bool
	TeamMembers  []string
}

type AgentRegistryInterface interface {
	FindAgentsByCapability(capability string) []*AgentInfo
	FindAgentsByKeyword(keyword string) []*AgentInfo
}

type AgentDiscoveryTool struct {
	registry AgentRegistryInterface
}

func NewAgentDiscoveryTool(registry AgentRegistryInterface) *AgentDiscoveryTool {
	return &AgentDiscoveryTool{
		registry: registry,
	}
}

func (t *AgentDiscoveryTool) Name() string {
	return "agent_discovery"
}

func (t *AgentDiscoveryTool) Description() string {
	return `Discover and query available agents. Use this to find agents capable of specific tasks.
Returns a list of matching agents with their capabilities and descriptions.
Use when you need to delegate a task and want to find the best agent for the job.`
}

func (t *AgentDiscoveryTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "Search query (e.g., 'coding', 'writing', 'research', 'math', or agent name)",
			},
			"capability": map[string]any{
				"type":        "string",
				"description": "Optional: specific capability to search for (e.g., 'coding', 'writing')",
			},
		},
		"required": []string{"query"},
	}
}

func (t *AgentDiscoveryTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	query, _ := args["query"].(string)
	capability, _ := args["capability"].(string)

	if query == "" && capability == "" {
		return ErrorResult("Either query or capability must be provided")
	}

	var agents []*AgentInfo

	if capability != "" {
		agents = t.registry.FindAgentsByCapability(capability)
	} else {
		agents = t.registry.FindAgentsByKeyword(query)
	}

	if len(agents) == 0 {
		return &ToolResult{
			ForLLM:  "No agents found matching your query. You may need to handle this task yourself or expand the search.",
			ForUser: "No agents found matching your criteria.",
		}
	}

	var sb strings.Builder
	sb.WriteString("Found the following agents:\n\n")

	for i, a := range agents {
		sb.WriteString(fmt.Sprintf("%d. **%s** (ID: %s)\n", i+1, a.Name, a.ID))
		if a.Description != "" {
			sb.WriteString(fmt.Sprintf("   Description: %s\n", a.Description))
		}
		if len(a.Capabilities) > 0 {
			sb.WriteString(fmt.Sprintf("   Capabilities: %s\n", strings.Join(a.Capabilities, ", ")))
		}
		sb.WriteString(fmt.Sprintf("   Model: %s\n", a.Model))
		if a.TeamLeader {
			sb.WriteString("   Role: Team Leader\n")
		}
		if len(a.TeamMembers) > 0 {
			sb.WriteString(fmt.Sprintf("   Team Members: %s\n", strings.Join(a.TeamMembers, ", ")))
		}
		sb.WriteString("\n")
	}

	return &ToolResult{
		ForLLM:  sb.String() + "\nUse this information to select the best agent for the task or delegate appropriately.",
		ForUser: sb.String(),
	}
}
