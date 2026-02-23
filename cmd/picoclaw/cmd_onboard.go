// PicoClaw - Ultra-lightweight personal AI agent
// License: MIT

package main

import (
	"bufio"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/sipeed/picoclaw/pkg/config"
)

var embeddedFiles embed.FS

func onboard() {
	configPath := getConfigPath()

	if _, err := os.Stat(configPath); err == nil {
		fmt.Printf("Config already exists at %s\n", configPath)
		fmt.Print("Overwrite? (y/n): ")
		var response string
		fmt.Scanln(&response)
		if response != "y" {
			fmt.Println("Aborted.")
			return
		}
	}

	fmt.Println("\n=== PicoClaw Setup ===")
	fmt.Println("")
	fmt.Println("Choose setup mode:")
	fmt.Println("  1) Quickstart   - Setup one provider + one channel (CLI)")
	fmt.Println("  2) Advanced      - Multiple providers, accounts, models, channels")
	fmt.Println("")

	choice := readChoice("Enter choice (1/2): ", []string{"1", "2"})

	if choice == "1" {
		quickstartSetup(configPath)
	} else {
		advancedSetup(configPath)
	}

	workspace := getWorkspacePath()
	createWorkspaceTemplates(workspace)

	fmt.Printf("\n%s picoclaw is ready!\n", logo)
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Chat: picoclaw agent -m \"Hello!\"")
	fmt.Println("  2. Or run: picoclaw agent (interactive mode)")
}

func quickstartSetup(configPath string) {
	fmt.Println("\n--- Quickstart Setup ---")
	fmt.Println("")

	provider := selectProvider()
	apiKey := readLine("Enter API key: ")
	modelName := selectModelForProvider(provider)
	channel := selectChannel()

	cfg := config.DefaultConfig()

	cfg.ModelList = []config.ModelConfig{}
	cfg.ModelList = append(cfg.ModelList, config.ModelConfig{
		ModelName: modelName,
		Model:     providerModels[provider],
		APIKey:    apiKey,
		APIBase:   providerBases[provider],
	})

	cfg.Agents.Defaults.Model = modelName
	cfg.Agents.Defaults.Provider = ""

	cfg.Channels = config.ChannelsConfig{}

	switch channel {
	case "telegram":
		cfg.Channels.Telegram = config.TelegramConfig{
			Enabled:   true,
			Token:     readLine("Enter Telegram bot token: "),
			AllowFrom: config.FlexibleStringSlice{},
		}
	}

	if err := config.SaveConfig(configPath, cfg); err != nil {
		fmt.Printf("Error saving config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nConfig saved to %s\n", configPath)
}

func advancedSetup(configPath string) {
	fmt.Println("\n--- Advanced Setup ---")
	fmt.Println("")

	cfg := config.DefaultConfig()

	fmt.Println("=== Models & Providers ===")
	fmt.Println("Add models (press Enter when done):")
	cfg.ModelList = []config.ModelConfig{}

	for {
		fmt.Printf("\nModel #%d:\n", len(cfg.ModelList)+1)
		provider := selectProvider()
		apiKey := readLine("  API key: ")
		modelName := readLine("  Model name (e.g., gpt-5.2): ")

		cfg.ModelList = append(cfg.ModelList, config.ModelConfig{
			ModelName: modelName,
			Model:     providerModels[provider],
			APIKey:    apiKey,
			APIBase:   providerBases[provider],
		})

		if !readYesNo("Add another model? (y/n): ") {
			break
		}
	}

	if len(cfg.ModelList) > 0 {
		defaultModel := cfg.ModelList[0].ModelName
		defaultModel = readLineWithDefault("Default model: ", defaultModel)
		cfg.Agents.Defaults.Model = defaultModel

		if readYesNo("Add model fallbacks? (y/n): ") {
			fmt.Println("Enter fallback models (one per line, empty to stop):")
			for {
				fb := readLine("  Fallback model: ")
				if fb == "" {
					break
				}
				cfg.Agents.Defaults.ModelFallbacks = append(cfg.Agents.Defaults.ModelFallbacks, fb)
			}
		}
	}

	fmt.Println("\n=== Channels ===")
	fmt.Println("Configure channels (press Enter when done):")

	cfg.Channels = config.ChannelsConfig{}

	channelTypes := []string{"cli", "telegram", "discord", "whatsapp", "slack", "feishu", "dingtalk", "qq", "onebot", "wecom", "maixcam"}

	for {
		fmt.Println("\nAvailable channels:", strings.Join(channelTypes, ", "))
		channel := readLine("Add channel (or Enter to skip): ")
		if channel == "" {
			break
		}

		switch channel {
		case "cli":
			fmt.Println("  CLI is always available (no config needed)")
		case "telegram":
			cfg.Channels.Telegram = config.TelegramConfig{
				Enabled:   true,
				Token:     readLine("  Bot token: "),
				AllowFrom: config.FlexibleStringSlice{},
			}
			fmt.Println("  Telegram channel enabled")
		case "discord":
			cfg.Channels.Discord = config.DiscordConfig{
				Enabled:   true,
				Token:     readLine("  Bot token: "),
				AllowFrom: config.FlexibleStringSlice{},
			}
			fmt.Println("  Discord channel enabled")
		default:
			fmt.Printf("  %s not yet implemented in quick mode, skipping\n", channel)
		}
	}

	if err := config.SaveConfig(configPath, cfg); err != nil {
		fmt.Printf("Error saving config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nConfig saved to %s\n", configPath)
}

var providerModels = map[string]string{
	"openai":     "openai/gpt-5.2",
	"anthropic":  "anthropic/claude-sonnet-4.6",
	"google":     "gemini/gemini-2.0-flash-exp",
	"deepseek":   "deepseek/deepseek-chat",
	"openrouter": "openrouter/auto",
	"ollama":     "ollama/llama3",
	"zhipu":      "zhipu/glm-4.7",
	"qwen":       "qwen/qwen-plus",
	"moonshot":   "moonshot/moonshot-v1-8k",
	"groq":       "groq/llama-3.3-70b-versatile",
	"opencode":   "opencode/minimax-m2.5-free",
}

var providerBases = map[string]string{
	"openai":     "https://api.openai.com/v1",
	"anthropic":  "https://api.anthropic.com/v1",
	"google":     "https://generativelanguage.googleapis.com/v1beta",
	"deepseek":   "https://api.deepseek.com/v1",
	"openrouter": "https://openrouter.ai/api/v1",
	"ollama":     "http://localhost:11434/v1",
	"zhipu":      "https://open.bigmodel.cn/api/paas/v4",
	"qwen":       "https://dashscope.aliyuncs.com/compatible-mode/v1",
	"moonshot":   "https://api.moonshot.cn/v1",
	"groq":       "https://api.groq.com/openai/v1",
	"opencode":   "https://opencode.ai/zen",
}

func selectProvider() string {
	providers := []string{
		"openai", "anthropic", "google", "deepseek", "openrouter",
		"ollama", "zhipu", "qwen", "moonshot", "groq", "opencode",
	}
	fmt.Println("Select provider:")
	for i, p := range providers {
		fmt.Printf("  %d) %s\n", i+1, p)
	}
	idx := readChoice("Enter number: ", intToStrings(1, len(providers)))
	return providers[strToInt(idx)-1]
}

func selectModelForProvider(provider string) string {
	return readLineWithDefault("Model name: ", providerModels[provider])
}

func selectChannel() string {
	channels := []string{"cli", "telegram"}
	fmt.Println("Select channel:")
	for i, c := range channels {
		fmt.Printf("  %d) %s\n", i+1, c)
	}
	idx := readChoice("Enter number: ", []string{"1", "2"})
	return channels[strToInt(idx)-1]
}

func readLine(prompt string) string {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}

func readLineWithDefault(prompt, def string) string {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	return line
}

func readChoice(prompt string, valid []string) string {
	for {
		fmt.Print(prompt)
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		for _, v := range valid {
			if line == v {
				return line
			}
		}
		fmt.Printf("Invalid choice. Valid options: %s\n", strings.Join(valid, ", "))
	}
}

func readYesNo(prompt string) bool {
	for {
		fmt.Print(prompt)
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		line = strings.ToLower(strings.TrimSpace(line))
		if line == "y" || line == "yes" {
			return true
		}
		if line == "n" || line == "no" {
			return false
		}
	}
}

func intToStrings(from, to int) []string {
	result := make([]string, to-from+1)
	for i := from; i <= to; i++ {
		result[i-from] = fmt.Sprintf("%d", i)
	}
	return result
}

func strToInt(s string) int {
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n
}

func getWorkspacePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".picoclaw", "workspace")
}

func copyEmbeddedToTarget(targetDir string) error {
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("Failed to create target directory: %w", err)
	}

	err := fs.WalkDir(embeddedFiles, "workspace", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := embeddedFiles.ReadFile(path)
		if err != nil {
			return fmt.Errorf("Failed to read embedded file %s: %w", path, err)
		}
		newPath, err := filepath.Rel("workspace", path)
		if err != nil {
			return fmt.Errorf("Failed to get relative path for %s: %v\n", path, err)
		}
		targetPath := filepath.Join(targetDir, newPath)
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return fmt.Errorf("Failed to create directory %s: %w", filepath.Dir(targetPath), err)
		}
		if err := os.WriteFile(targetPath, data, 0o644); err != nil {
			return fmt.Errorf("Failed to write file %s: %w", targetPath, err)
		}
		return nil
	})
	return err
}

func createWorkspaceTemplates(workspace string) {
	err := copyEmbeddedToTarget(workspace)
	if err != nil {
		fmt.Printf("Error copying workspace templates: %v\n", err)
	}
}
