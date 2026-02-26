# Codebase Structure

**Analysis Date:** 2026-02-26

## Directory Layout

```
picoclaw/
├── cmd/picoclaw/          # CLI command implementations
├── pkg/                  # Core packages
│   ├── agent/            # AI agent logic
│   ├── auth/             # Authentication
│   ├── bus/              # Message bus
│   ├── channels/         # Messaging platform integrations
│   ├── config/           # Configuration management
│   ├── constants/        # Constants
│   ├── cron/             # Scheduled tasks
│   ├── devices/          # Hardware device support
│   ├── gateway/          # HTTP server
│   ├── health/           # Health checks
│   ├── heartbeat/        # Heartbeat service
│   ├── logger/           # Logging
│   ├── migrate/          # Migration utilities
│   ├── pairing/          # Device pairing
│   ├── providers/        # LLM provider implementations
│   ├── routing/          # Message routing
│   ├── session/          # Session management
│   ├── skills/           # Skill system
│   ├── state/            # Persistent state
│   ├── tools/            # Tool implementations
│   ├── utils/            # Utilities
│   └── voice/            # Voice processing
├── config/               # Configuration examples
├── workspace/            # Workspace directory (generated)
├── go.mod                # Go module definition
└── Makefile              # Build commands
```

## Directory Purposes

**`cmd/picoclaw/`:**
- Purpose: CLI command entry points
- Contains: `main.go`, `cmd_*.go` files
- Key files: 
  - `main.go` - Main entry point with command routing
  - `cmd_gateway.go` - Gateway/server command
  - `cmd_onboard.go` - Initialization command
  - `cmd_agent.go` - Interactive agent mode
  - `cmd_auth.go` - Authentication management
  - `cmd_status.go` - Status display
  - `cmd_cron.go` - Cron job management
  - `cmd_pairing.go` - Device pairing
  - `cmd_skills.go` - Skill management

**`pkg/agent/`:**
- Purpose: Core AI agent implementation
- Contains: Agent loop, instance, registry, context, memory
- Key files:
  - `loop.go` - Main message processing loop (1301 lines)
  - `instance.go` - Agent instance representation
  - `registry.go` - Agent registry and routing
  - `context.go` - Context building for LLM
  - `memory.go` - Memory/context management

**`pkg/channels/`:**
- Purpose: Multi-platform messaging integrations
- Contains: Channel implementations for 12+ platforms
- Key files:
  - `manager.go` - Channel manager
  - `base.go` - Base channel interface
  - `telegram.go`, `discord.go`, `slack.go` - Platform integrations

**`pkg/providers/`:**
- Purpose: LLM provider implementations
- Contains: Multiple provider types
- Key files:
  - `factory.go` - Provider factory
  - `types.go` - Common types
  - `claude_provider.go` - Anthropic Claude
  - `github_copilot_provider.go` - GitHub Copilot
  - `fallback.go` - Provider fallback chain

**`pkg/tools/`:**
- Purpose: Tools available to agents
- Contains: Tool implementations
- Key files:
  - `base.go` - Tool interface
  - `registry.go` - Tool registry
  - `web.go` - Web search/fetch
  - `shell.go` - Shell execution
  - `filesystem.go` - File operations
  - `spawn.go` - Subagent spawning
  - `skills_*.go` - Skill tools

**`pkg/skills/`:**
- Purpose: Extensible skill system
- Contains: Skill loading, installation, registry
- Key files:
  - `loader.go` - Skill loader
  - `installer.go` - Skill installer
  - `registry.go` - Skill registry

**`pkg/config/`:**
- Purpose: Configuration management
- Contains: Config struct and loading
- Key files:
  - `config.go` - Main config (722 lines)
  - `defaults.go` - Default configuration
  - `migration.go` - Config migration

**`pkg/bus/`:**
- Purpose: Internal message communication
- Contains: Message bus implementation

**`pkg/gateway/`:**
- Purpose: HTTP server
- Contains: HTTP server implementation

## Key File Locations

**Entry Points:**
- `cmd/picoclaw/main.go` - CLI entry point
- `cmd/picoclaw/cmd_gateway.go` - Gateway command (primary server)

**Configuration:**
- `pkg/config/config.go` - Config struct definition
- `config/config.example.json` - Example config

**Core Logic:**
- `pkg/agent/loop.go` - Agent message processing
- `pkg/providers/factory.go` - Provider creation
- `pkg/channels/manager.go` - Channel management

**Testing:**
- Test files are co-located with source (e.g., `loop_test.go`, `config_test.go`)

## Naming Conventions

**Files:**
- Lowercase with underscores: `agent_loop.go`, `config_test.go`
- Commands: `cmd_*.go`
- Channel/platform specific: `{platform}.go` (telegram.go, discord.go)

**Directories:**
- Lowercase, single words or common names: `agent`, `channels`, `tools`
- Plural for collections: `channels`, `providers`

**Packages:**
- Descriptive single words: `agent`, `channels`, `providers`, `bus`
- Consistent with directory names

## Where to Add New Code

**New Channel Integration:**
- Implementation: `pkg/channels/{platform}.go`
- Tests: `pkg/channels/{platform}_test.go`
- Register in: `pkg/channels/manager.go`

**New LLM Provider:**
- Implementation: `pkg/providers/{provider}_provider.go`
- Tests: `pkg/providers/{provider}_test.go`
- Register in: `pkg/providers/factory.go`

**New Tool:**
- Implementation: `pkg/tools/{tool_name}.go`
- Tests: `pkg/tools/{tool_name}_test.go`
- Follow tool interface in `pkg/tools/base.go`

**New CLI Command:**
- Implementation: `cmd/picoclaw/cmd_{command}.go`
- Register in: `cmd/picoclaw/main.go` switch statement

**New Skill:**
- Managed via `pkg/skills/` - skills are loaded from workspace

## Special Directories

**`workspace/`:**
- Purpose: Default workspace for agent files
- Generated: Yes, created during onboard
- Committed: No, in .gitignore

**`config/`:**
- Purpose: Example configurations
- Generated: No
- Committed: Yes

---

*Structure analysis: 2026-02-26*
