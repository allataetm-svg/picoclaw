# Architecture

**Analysis Date:** 2026-02-26

## Project Overview

PicoClaw is an ultra-lightweight personal AI agent framework that connects to multiple messaging channels and uses various LLM providers to process messages and generate responses. It is inspired by nanobot (HKUDS) and is written in Go.

## Pattern Overview

**Overall:** Event-driven, message-bus architecture with plugin-based channel integrations

**Key Characteristics:**
- Message routing via internal event bus (`pkg/bus`)
- Channel-agnostic design - supports 12+ messaging platforms
- Multi-provider LLM support with fallback chains
- Tool-augmented agents with extensible skill system
- Session-based conversation context management

## Layers

**CLI/Command Layer (`cmd/picoclaw/`):**
- Purpose: Command-line interface entry points
- Location: `cmd/picoclaw/`
- Contains: Main CLI commands (onboard, agent, gateway, status, auth, cron, pairing, skills)
- Depends on: All core packages
- Used by: End users via CLI

**Gateway Layer (`pkg/gateway/`):**
- Purpose: HTTP server for receiving webhooks and serving WebUI
- Location: `pkg/gateway/`
- Contains: `server.go` - HTTP server implementation
- Depends on: `pkg/channels`, `pkg/bus`
- Used by: Channel webhooks and optional WebUI

**Channel Layer (`pkg/channels/`):**
- Purpose: Messaging platform integrations
- Location: `pkg/channels/`
- Contains: Individual channel implementations (telegram.go, discord.go, slack.go, etc.)
- Depends on: `pkg/bus`, `pkg/config`
- Used by: Gateway for receiving messages

**Agent Layer (`pkg/agent/`):**
- Purpose: Core AI agent logic and message processing
- Location: `pkg/agent/`
- Contains: `loop.go` - main agent loop, `instance.go` - agent instance, `registry.go` - agent registry, `context.go` - context building
- Depends on: `pkg/providers`, `pkg/tools`, `pkg/skills`, `pkg/bus`, `pkg/session`
- Used by: Gateway command

**Provider Layer (`pkg/providers/`):**
- Purpose: LLM provider abstractions and implementations
- Location: `pkg/providers/`
- Contains: Multiple provider implementations (claude_provider.go, openai_provider.go, anthropic, etc.)
- Depends on: External LLM APIs
- Used by: Agent layer

**Tools Layer (`pkg/tools/`):**
- Purpose: Tools available to agents for executing actions
- Location: `pkg/tools/`
- Contains: Tool implementations (web.go, shell.go, filesystem.go, spawn.go, skills_*.go, etc.)
- Depends on: Various system resources
- Used by: Agent layer

**Skills Layer (`pkg/skills/`):**
- Purpose: Skill management system for extensible agent capabilities
- Location: `pkg/skills/`
- Contains: `loader.go` - skill loading, `installer.go` - skill installation, `registry.go` - skill registry
- Depends on: Filesystem, network
- Used by: Tools layer, CLI commands

**Support Layers:**
- `pkg/config/` - Configuration loading and management
- `pkg/bus/` - Internal message bus for communication
- `pkg/session/` - Conversation session management
- `pkg/state/` - Persistent state management
- `pkg/routing/` - Message routing logic
- `pkg/cron/` - Scheduled task management
- `pkg/devices/` - Hardware device support (USB monitoring)

## Data Flow

**Incoming Message Flow:**
1. Channel receives message (webhook/websocket/polling)
2. Channel publishes to message bus as `InboundMessage`
3. Agent loop consumes from bus
4. Route determines which agent handles message
5. Agent loop processes with LLM + tools
6. Response published to bus as `OutboundMessage`
7. Channel sends response to platform

**Configuration Flow:**
1. CLI command invoked
2. Config loaded from JSON file (`~/.picoclaw/config.json`)
3. Environment variables override JSON (via caarlos0/env)
4. Config validated and passed to components

## Key Abstractions

**Message Bus:**
- Purpose: Decouple components via internal events
- Examples: `pkg/bus/bus.go`
- Pattern: Pub/sub with typed message structures

**LLM Provider Interface:**
- Purpose: Uniform interface for different LLM APIs
- Examples: `pkg/providers/types.go`, `pkg/providers/factory.go`
- Pattern: Factory pattern with provider implementations

**Tool Registry:**
- Purpose: Register and execute tools available to agents
- Examples: `pkg/tools/registry.go`
- Pattern: Plugin architecture

**Channel Manager:**
- Purpose: Manage multiple channel instances
- Examples: `pkg/channels/manager.go`
- Pattern: Manager pattern with channel interface

## Entry Points

**Main Gateway (Primary):**
- Location: `cmd/picoclaw/main.go` → `gatewayCmd()` in `cmd_gateway.go`
- Triggers: `picoclaw gateway` command
- Responsibilities: Start HTTP server, initialize all components, process messages

**Onboard:**
- Location: `cmd/picoclaw/cmd_onboard.go`
- Triggers: `picoclaw onboard` command
- Responsibilities: Initialize configuration and workspace

**Agent CLI:**
- Location: `cmd/pic_agent.go`
-oclaw/cmd Triggers: `picoclaw agent` command
- Responsibilities: Interactive agent mode

## Error Handling

**Strategy:** Error wrapping with contextual logging

**Patterns:**
- Errors returned from functions with context
- Logging at appropriate levels (Info, Warn, Error)
- Graceful degradation (e.g., fallback providers)
- User-friendly error messages in responses

## Cross-Cutting Concerns

**Logging:** Custom logger in `pkg/logger` with structured logging (InfoCF, WarnCF, ErrorCF, DebugCF)

**Validation:** Config validation in `pkg/config/config.go`

**Authentication:** OAuth and API key management in `pkg/auth/`

---

*Architecture analysis: 2026-02-26*
