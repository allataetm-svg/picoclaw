package agent

import (
	"strings"
	"sync"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/providers"
	"github.com/sipeed/picoclaw/pkg/routing"
	"github.com/sipeed/picoclaw/pkg/tools"
)

// AgentRegistry manages multiple agent instances and routes messages to them.
type AgentRegistry struct {
	agents   map[string]*AgentInstance
	resolver *routing.RouteResolver
	mu       sync.RWMutex
}

// NewAgentRegistry creates a registry from config, instantiating all agents.
func NewAgentRegistry(
	cfg *config.Config,
	provider providers.LLMProvider,
) *AgentRegistry {
	registry := &AgentRegistry{
		agents:   make(map[string]*AgentInstance),
		resolver: routing.NewRouteResolver(cfg),
	}

	agentConfigs := cfg.Agents.List
	if len(agentConfigs) == 0 {
		implicitAgent := &config.AgentConfig{
			ID:      "main",
			Default: true,
		}
		instance := NewAgentInstance(implicitAgent, &cfg.Agents.Defaults, cfg, provider)
		registry.agents["main"] = instance
		logger.InfoCF("agent", "Created implicit main agent (no agents.list configured)", nil)
	} else {
		for i := range agentConfigs {
			ac := &agentConfigs[i]
			id := routing.NormalizeAgentID(ac.ID)
			instance := NewAgentInstance(ac, &cfg.Agents.Defaults, cfg, provider)
			registry.agents[id] = instance
			logger.InfoCF("agent", "Registered agent",
				map[string]any{
					"agent_id":  id,
					"name":      ac.Name,
					"workspace": instance.Workspace,
					"model":     instance.Model,
				})
		}
	}

	return registry
}

// GetAgent returns the agent instance for a given ID.
func (r *AgentRegistry) GetAgent(agentID string) (*AgentInstance, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id := routing.NormalizeAgentID(agentID)
	agent, ok := r.agents[id]
	return agent, ok
}

// ResolveRoute determines which agent handles the message.
func (r *AgentRegistry) ResolveRoute(input routing.RouteInput) routing.ResolvedRoute {
	return r.resolver.ResolveRoute(input)
}

// ListAgentIDs returns all registered agent IDs.
func (r *AgentRegistry) ListAgentIDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.agents))
	for id := range r.agents {
		ids = append(ids, id)
	}
	return ids
}

// CanSpawnSubagent checks if parentAgentID is allowed to spawn targetAgentID.
func (r *AgentRegistry) CanSpawnSubagent(parentAgentID, targetAgentID string) bool {
	parent, ok := r.GetAgent(parentAgentID)
	if !ok {
		return false
	}
	if parent.Subagents == nil || parent.Subagents.AllowAgents == nil {
		return false
	}
	targetNorm := routing.NormalizeAgentID(targetAgentID)
	for _, allowed := range parent.Subagents.AllowAgents {
		if allowed == "*" {
			return true
		}
		if routing.NormalizeAgentID(allowed) == targetNorm {
			return true
		}
	}
	return false
}

// GetDefaultAgent returns the default agent instance.
func (r *AgentRegistry) GetDefaultAgent() *AgentInstance {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if agent, ok := r.agents["main"]; ok {
		return agent
	}
	for _, agent := range r.agents {
		return agent
	}
	return nil
}

// GetAllAgents returns all registered agent instances.
func (r *AgentRegistry) GetAllAgents() []*AgentInstance {
	r.mu.RLock()
	defer r.mu.RUnlock()
	agents := make([]*AgentInstance, 0, len(r.agents))
	for _, agent := range r.agents {
		agents = append(agents, agent)
	}
	return agents
}

// FindAgentsByCapability returns agents that have the given capability.
func (r *AgentRegistry) FindAgentsByCapability(capability string) []*AgentInstance {
	r.mu.RLock()
	defer r.mu.RUnlock()
	capability = strings.ToLower(capability)
	var agents []*AgentInstance
	for _, agent := range r.agents {
		for _, cap := range agent.Capabilities {
			if strings.ToLower(cap) == capability {
				agents = append(agents, agent)
				break
			}
		}
	}
	return agents
}

// FindAgentsByKeyword returns agents whose name, description, or capabilities match the keyword.
func (r *AgentRegistry) FindAgentsByKeyword(keyword string) []*AgentInstance {
	r.mu.RLock()
	defer r.mu.RUnlock()
	keyword = strings.ToLower(keyword)
	var agents []*AgentInstance
	for _, agent := range r.agents {
		if strings.Contains(strings.ToLower(agent.Name), keyword) ||
			strings.Contains(strings.ToLower(agent.Description), keyword) {
			agents = append(agents, agent)
			continue
		}
		for _, cap := range agent.Capabilities {
			if strings.Contains(strings.ToLower(cap), keyword) {
				agents = append(agents, agent)
				break
			}
		}
	}
	return agents
}

type AgentInfoAdapter struct {
	registry *AgentRegistry
}

func (a *AgentInfoAdapter) toAgentInfo(agent *AgentInstance) *tools.AgentInfo {
	return &tools.AgentInfo{
		ID:           agent.ID,
		Name:         agent.Name,
		Description:  agent.Description,
		Model:        agent.Model,
		Capabilities: agent.Capabilities,
		TeamLeader:   agent.TeamLeader,
		TeamMembers:  agent.TeamMembers,
	}
}

func (a *AgentInfoAdapter) FindAgentsByCapability(capability string) []*tools.AgentInfo {
	agents := a.registry.FindAgentsByCapability(capability)
	result := make([]*tools.AgentInfo, len(agents))
	for i, agent := range agents {
		result[i] = a.toAgentInfo(agent)
	}
	return result
}

func (a *AgentInfoAdapter) FindAgentsByKeyword(keyword string) []*tools.AgentInfo {
	agents := a.registry.FindAgentsByKeyword(keyword)
	result := make([]*tools.AgentInfo, len(agents))
	for i, agent := range agents {
		result[i] = a.toAgentInfo(agent)
	}
	return result
}

func (r *AgentRegistry) GetAgentInfoAdapter() tools.AgentRegistryInterface {
	return &AgentInfoAdapter{registry: r}
}
