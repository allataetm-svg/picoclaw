# Issue Report: Comparison with Original Repo

## Summary
This report compares issues from the original repo (sipeed/picoclaw) with our fork to identify which bugs/features exist in our codebase.

---

## ✅ Issues Present in Our Codebase

### 1. [BUG] Default api_base is always set to GLM provider (#680)
**Status:** OPEN in original  
**Present in our fork:** YES

**Description:** When users provide a config with a model but omit `api_base`, the system incorrectly uses the GLM provider's default URL instead.

**Root Cause:** `DefaultConfig()` in `pkg/config/defaults.go` has a pre-populated `ModelList` with ~18 template entries. When JSON unmarshals into this, it reuses existing slice elements causing preset `APIBase` values to leak into user configs.

**Location:** `pkg/config/defaults.go:120` - ModelList with preset values

---

### 2. [BUG] DefaultConfig ModelList template values leak (#721)
**Status:** CLOSED in original (same as #680)  
**Present in our fork:** YES

**Description:** Same issue as #680 - Go's JSON decoder reuses slice backing array elements.

**Location:** `pkg/config/config.go:513` - LoadConfig function

---

### 3. [BUG] Network timeouts misclassified as context window errors (#683)
**Status:** OPEN in original  
**Present in our fork:** YES

**Description:** HTTP timeouts are incorrectly classified as context window errors, triggering useless history compression.

**Root Cause:** Overly broad substring matching in `pkg/agent/loop.go:556`:
```go
isContextError := strings.Contains(errMsg, "token") ||
    strings.Contains(errMsg, "context") ||
    ...
```

**Location:** `pkg/agent/loop.go:556-559`

---

### 4. [BUG] Repeated reprocessing of entire context (#607)
**Status:** OPEN in original (priority: high)  
**Present in our fork:** YES

**Description:** System prompt keeps changing, causing the model to reprocess thousands of tokens every turn.

**Location:** `pkg/agent/context.go` - ContextBuilder

---

### 5. [BUG] `gateway status` launches an extra gateway worker process (#671)
**Status:** OPEN in original  
**Present in our fork:** NEEDS CHECK

**Description:** Running `picoclaw gateway status` starts a live gateway event loop instead of returning service state.

---

## 📋 Issues NOT Present (Already Fixed)

### 1. [BUG] Redundant tools definitions in system prompt (#731)
**Status:** OPEN in original  
**Fixed in our fork:** LIKELY

Our implementation may differ - need to verify.

---

## 🆕 Features Implemented (Our Enhancements)

### 1. Interactive TUI Onboard (huh library)
- Space to select, arrow keys to navigate, enter to submit
- Provider groups configuration
- Antigravity OAuth support

### 2. Agent System Enhancements
- Capabilities and discovery tool
- Hierarchical teams (TeamLeader, TeamMembers)
- Agent supervision (RequireApproval)
- Persistent memory (agent-specific + shared)
- Skill specialization (SkillWhitelist)
- Agent-specific system prompts

---

## Testing

Run tests:
```bash
go test ./...
```

---

## Recommendations

1. **High Priority:** Fix issue #680/#721 - ModelList preset value leak
2. **High Priority:** Fix issue #683 - Timeout error classification
3. **Medium Priority:** Fix issue #607 - Context reprocessing

---

*Report generated: 2026-02-24*
