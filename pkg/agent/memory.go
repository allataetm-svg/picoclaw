// PicoClaw - Ultra-lightweight personal AI agent
// Inspired by and based on nanobot: https://github.com/HKUDS/nanobot
// License: MIT
//
// Copyright (c) 2026 PicoClaw contributors

package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MemoryStore manages persistent memory for the agent.
// - Long-term memory: memory/MEMORY.md
// - Daily notes: memory/YYYYMM/YYYYMMDD.md
// - Agent-specific memory: memory/agents/{agent_id}.md
// - Shared knowledge base: memory/shared/
// - Learned facts: memory/facts.json
type MemoryStore struct {
	workspace  string
	memoryDir  string
	memoryFile string
	agentID    string
	sharedDir  string
	factsFile  string
}

// Fact represents a learned fact by an agent
type Fact struct {
	ID        string   `json:"id"`
	Content   string   `json:"content"`
	Source    string   `json:"source"`
	AgentIDs  []string `json:"agent_ids"`
	CreatedAt int64    `json:"created_at"`
	Tags      []string `json:"tags"`
}

// NewMemoryStore creates a new MemoryStore with the given workspace path.
// It ensures the memory directory exists.
func NewMemoryStore(workspace string) *MemoryStore {
	return NewMemoryStoreWithAgent(workspace, "")
}

// NewMemoryStoreWithAgent creates a MemoryStore for a specific agent.
func NewMemoryStoreWithAgent(workspace, agentID string) *MemoryStore {
	memoryDir := filepath.Join(workspace, "memory")
	memoryFile := filepath.Join(memoryDir, "MEMORY.md")
	sharedDir := filepath.Join(memoryDir, "shared")
	factsFile := filepath.Join(memoryDir, "facts.json")

	// Ensure memory directory exists
	os.MkdirAll(memoryDir, 0o755)
	os.MkdirAll(sharedDir, 0o755)

	return &MemoryStore{
		workspace:  workspace,
		memoryDir:  memoryDir,
		memoryFile: memoryFile,
		agentID:    agentID,
		sharedDir:  sharedDir,
		factsFile:  factsFile,
	}
}

// getTodayFile returns the path to today's daily note file (memory/YYYYMM/YYYYMMDD.md).
func (ms *MemoryStore) getTodayFile() string {
	today := time.Now().Format("20060102") // YYYYMMDD
	monthDir := today[:6]                  // YYYYMM
	filePath := filepath.Join(ms.memoryDir, monthDir, today+".md")
	return filePath
}

// ReadLongTerm reads the long-term memory (MEMORY.md).
// Returns empty string if the file doesn't exist.
func (ms *MemoryStore) ReadLongTerm() string {
	if data, err := os.ReadFile(ms.memoryFile); err == nil {
		return string(data)
	}
	return ""
}

// WriteLongTerm writes content to the long-term memory file (MEMORY.md).
func (ms *MemoryStore) WriteLongTerm(content string) error {
	return os.WriteFile(ms.memoryFile, []byte(content), 0o644)
}

// ReadToday reads today's daily note.
// Returns empty string if the file doesn't exist.
func (ms *MemoryStore) ReadToday() string {
	todayFile := ms.getTodayFile()
	if data, err := os.ReadFile(todayFile); err == nil {
		return string(data)
	}
	return ""
}

// AppendToday appends content to today's daily note.
// If the file doesn't exist, it creates a new file with a date header.
func (ms *MemoryStore) AppendToday(content string) error {
	todayFile := ms.getTodayFile()

	// Ensure month directory exists
	monthDir := filepath.Dir(todayFile)
	os.MkdirAll(monthDir, 0o755)

	var existingContent string
	if data, err := os.ReadFile(todayFile); err == nil {
		existingContent = string(data)
	}

	var newContent string
	if existingContent == "" {
		// Add header for new day
		header := fmt.Sprintf("# %s\n\n", time.Now().Format("2006-01-02"))
		newContent = header + content
	} else {
		// Append to existing content
		newContent = existingContent + "\n" + content
	}

	return os.WriteFile(todayFile, []byte(newContent), 0o644)
}

// GetRecentDailyNotes returns daily notes from the last N days.
// Contents are joined with "---" separator.
func (ms *MemoryStore) GetRecentDailyNotes(days int) string {
	var sb strings.Builder
	first := true

	for i := 0; i < days; i++ {
		date := time.Now().AddDate(0, 0, -i)
		dateStr := date.Format("20060102") // YYYYMMDD
		monthDir := dateStr[:6]            // YYYYMM
		filePath := filepath.Join(ms.memoryDir, monthDir, dateStr+".md")

		if data, err := os.ReadFile(filePath); err == nil {
			if !first {
				sb.WriteString("\n\n---\n\n")
			}
			sb.Write(data)
			first = false
		}
	}

	return sb.String()
}

// GetMemoryContext returns formatted memory context for the agent prompt.
// Includes long-term memory and recent daily notes.
func (ms *MemoryStore) GetMemoryContext() string {
	longTerm := ms.ReadLongTerm()
	recentNotes := ms.GetRecentDailyNotes(3)

	if longTerm == "" && recentNotes == "" {
		return ""
	}

	var sb strings.Builder

	if longTerm != "" {
		sb.WriteString("## Long-term Memory\n\n")
		sb.WriteString(longTerm)
	}

	if recentNotes != "" {
		if longTerm != "" {
			sb.WriteString("\n\n---\n\n")
		}
		sb.WriteString("## Recent Daily Notes\n\n")
		sb.WriteString(recentNotes)
	}

	return sb.String()
}

func (ms *MemoryStore) GetAgentMemoryFile() string {
	if ms.agentID == "" {
		return ms.memoryFile
	}
	return filepath.Join(ms.memoryDir, "agents", ms.agentID+".md")
}

func (ms *MemoryStore) ReadAgentMemory() string {
	agentFile := ms.GetAgentMemoryFile()
	if data, err := os.ReadFile(agentFile); err == nil {
		return string(data)
	}
	return ""
}

func (ms *MemoryStore) WriteAgentMemory(content string) error {
	if ms.agentID == "" {
		return ms.WriteLongTerm(content)
	}
	agentFile := ms.GetAgentMemoryFile()
	agentDir := filepath.Dir(agentFile)
	os.MkdirAll(agentDir, 0o755)
	return os.WriteFile(agentFile, []byte(content), 0o644)
}

func (ms *MemoryStore) AppendAgentMemory(content string) error {
	existing := ms.ReadAgentMemory()
	var newContent string
	if existing == "" {
		newContent = content
	} else {
		newContent = existing + "\n" + content
	}
	return ms.WriteAgentMemory(newContent)
}

func (ms *MemoryStore) ReadSharedMemory(agentID string) string {
	sharedFile := filepath.Join(ms.sharedDir, agentID+".md")
	if data, err := os.ReadFile(sharedFile); err == nil {
		return string(data)
	}
	return ""
}

func (ms *MemoryStore) WriteSharedMemory(agentID, content string) error {
	sharedFile := filepath.Join(ms.sharedDir, agentID+".md")
	return os.WriteFile(sharedFile, []byte(content), 0o644)
}

func (ms *MemoryStore) GetAllSharedMemory() string {
	var sb strings.Builder
	entries, err := os.ReadDir(ms.sharedDir)
	if err != nil {
		return ""
	}
	first := true
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			data, err := os.ReadFile(filepath.Join(ms.sharedDir, e.Name()))
			if err == nil {
				if !first {
					sb.WriteString("\n\n---\n\n")
				}
				sb.Write(data)
				first = false
			}
		}
	}
	return sb.String()
}
