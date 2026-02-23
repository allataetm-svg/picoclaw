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

//go:generate cp -r ../../workspace workspace
//go:embed workspace
var embeddedWorkspace embed.FS

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
	fmt.Println("  1) Quickstart   - One provider + one channel")
	fmt.Println("  2) Advanced      - Multiple providers, models, channels")
	fmt.Println("")

	choice := promptChoice([]string{"Quickstart", "Advanced"})

	cfg := config.DefaultConfig()

	if choice == 0 {
		quickstartSetup(cfg)
	} else {
		advancedSetup(cfg)
	}

	workspace := getWorkspacePath()
	createWorkspaceTemplates(workspace)

	if err := config.SaveConfig(configPath, cfg); err != nil {
		fmt.Printf("Error saving config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nConfig saved to %s\n", configPath)
	fmt.Printf("\n%s picoclaw is ready!\n", logo)
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Chat: picoclaw agent -m \"Hello!\"")
	fmt.Println("  2. Or run: picoclaw agent (interactive mode)")
}

func quickstartSetup(cfg *config.Config) {
	fmt.Println("\n--- Quickstart Setup ---")
	fmt.Println("")

	providerIdx := promptChoice(providerList)
	provider := providerList[providerIdx]

	apiKey := readLine("Enter API key: ")

	model := promptSelect("Select model:", provider)

	channel := promptChoice(channelList[:2])

	cfg.ModelList = []config.ModelConfig{{
		ModelName: model,
		Model:     "opencode/" + model,
		APIKey:    apiKey,
		APIBase:   "https://opencode.ai/zen",
	}}

	cfg.Agents.Defaults.Model = model

	if channel == 1 {
		cfg.Channels.Telegram.Enabled = true
		cfg.Channels.Telegram.Token = readLine("Enter Telegram bot token: ")
		cfg.Channels.Telegram.AllowFrom = config.FlexibleStringSlice{}
	}
}

func advancedSetup(cfg *config.Config) {
	fmt.Println("\n--- Advanced Setup ---")
	fmt.Println("")

	fmt.Println("=== Providers & Models ===")
	fmt.Println("Select providers (space to toggle, enter to confirm):")

	selectedProviders := promptMultiSelect(providerList)

	cfg.ModelList = []config.ModelConfig{}

	for _, pi := range selectedProviders {
		provider := providerList[pi]
		models := providerModels[provider]

		fmt.Printf("\nProvider: %s\n", provider)
		selectedModels := promptMultiSelect(models)

		apiKey := readLine(fmt.Sprintf("API key for %s: ", provider))

		for _, mi := range selectedModels {
			modelName := models[mi]
			cfg.ModelList = append(cfg.ModelList, config.ModelConfig{
				ModelName: modelName,
				Model:     "opencode/" + modelName,
				APIKey:    apiKey,
				APIBase:   "https://opencode.ai/zen",
			})
		}
	}

	if len(cfg.ModelList) > 0 {
		cfg.Agents.Defaults.Model = cfg.ModelList[0].ModelName

		if len(cfg.ModelList) > 1 {
			fmt.Println("\nSelect default model:")
			defaultIdx := promptSelectIndex("Default model:", modelsToNames(cfg.ModelList))
			cfg.Agents.Defaults.Model = cfg.ModelList[defaultIdx].ModelName

			if promptYesNo("Add model fallbacks?") {
				fmt.Println("Select fallback models (space to toggle, enter to confirm):")
				fallbacks := promptMultiSelectIndex(modelsToNames(cfg.ModelList))
				for _, fi := range fallbacks {
					if fi != defaultIdx {
						cfg.Agents.Defaults.ModelFallbacks = append(cfg.Agents.Defaults.ModelFallbacks, cfg.ModelList[fi].ModelName)
					}
				}
			}
		}
	}

	fmt.Println("\n=== Channels ===")
	fmt.Println("Select channels (space to toggle, enter to confirm):")
	selectedChannels := promptMultiSelect(channelList)

	for _, ci := range selectedChannels {
		switch ci {
		case 0:
			fmt.Println("  CLI is always available")
		case 1:
			cfg.Channels.Telegram.Enabled = true
			cfg.Channels.Telegram.Token = readLine("  Telegram bot token: ")
			cfg.Channels.Telegram.AllowFrom = config.FlexibleStringSlice{}
		case 2:
			cfg.Channels.Discord.Enabled = true
			cfg.Channels.Discord.Token = readLine("  Discord bot token: ")
			cfg.Channels.Discord.AllowFrom = config.FlexibleStringSlice{}
		case 3:
			cfg.Channels.WhatsApp.Enabled = true
			cfg.Channels.WhatsApp.BridgeURL = readLine("  WhatsApp bridge URL (default: ws://localhost:3001): ")
			if cfg.Channels.WhatsApp.BridgeURL == "" {
				cfg.Channels.WhatsApp.BridgeURL = "ws://localhost:3001"
			}
			cfg.Channels.WhatsApp.AllowFrom = config.FlexibleStringSlice{}
		case 4:
			cfg.Channels.Slack.Enabled = true
			cfg.Channels.Slack.BotToken = readLine("  Slack bot token (xoxb-...): ")
			cfg.Channels.Slack.AppToken = readLine("  Slack app token (xapp-...): ")
			cfg.Channels.Slack.AllowFrom = config.FlexibleStringSlice{}
		case 5:
			cfg.Channels.Feishu.Enabled = true
			cfg.Channels.Feishu.AppID = readLine("  Feishu app ID: ")
			cfg.Channels.Feishu.AppSecret = readLine("  Feishu app secret: ")
			cfg.Channels.Feishu.AllowFrom = config.FlexibleStringSlice{}
		case 6:
			cfg.Channels.DingTalk.Enabled = true
			cfg.Channels.DingTalk.ClientID = readLine("  DingTalk client ID: ")
			cfg.Channels.DingTalk.ClientSecret = readLine("  DingTalk client secret: ")
			cfg.Channels.DingTalk.AllowFrom = config.FlexibleStringSlice{}
		case 7:
			cfg.Channels.QQ.Enabled = true
			cfg.Channels.QQ.AppID = readLine("  QQ app ID: ")
			cfg.Channels.QQ.AppSecret = readLine("  QQ app secret: ")
			cfg.Channels.QQ.AllowFrom = config.FlexibleStringSlice{}
		case 8:
			cfg.Channels.OneBot.Enabled = true
			cfg.Channels.OneBot.WSUrl = readLine("  OneBot WebSocket URL (default: ws://127.0.0.1:3001): ")
			if cfg.Channels.OneBot.WSUrl == "" {
				cfg.Channels.OneBot.WSUrl = "ws://127.0.0.1:3001"
			}
			cfg.Channels.OneBot.AccessToken = readLine("  OneBot access token (optional): ")
			cfg.Channels.OneBot.AllowFrom = config.FlexibleStringSlice{}
		case 9:
			cfg.Channels.WeCom.Enabled = true
			cfg.Channels.WeCom.Token = readLine("  WeCom webhook token: ")
			cfg.Channels.WeCom.WebhookURL = readLine("  WeCom webhook URL: ")
			cfg.Channels.WeCom.AllowFrom = config.FlexibleStringSlice{}
		case 10:
			cfg.Channels.MaixCam.Enabled = true
			cfg.Channels.MaixCam.Host = readLine("  MaixCam host (default: 0.0.0.0): ")
			if cfg.Channels.MaixCam.Host == "" {
				cfg.Channels.MaixCam.Host = "0.0.0.0"
			}
			cfg.Channels.MaixCam.AllowFrom = config.FlexibleStringSlice{}
		}
	}
}

var providerList = []string{
	"openai",
	"anthropic",
	"google",
	"deepseek",
	"openrouter",
	"ollama",
	"zhipu",
	"qwen",
	"moonshot",
	"groq",
	"opencode",
}

var providerModels = map[string][]string{
	"openai": {
		"gpt-5.2", "gpt-5.1", "gpt-5", "gpt-5-nano",
	},
	"anthropic": {
		"claude-opus-4-6", "claude-sonnet-4-6", "claude-haiku-4-5",
	},
	"google": {
		"gemini-3-pro", "gemini-3-flash", "gemini-3.1-pro",
	},
	"deepseek": {
		"deepseek-chat", "deepseek-v3",
	},
	"openrouter": {
		"openrouter/auto", "openrouter/gpt-5.2",
	},
	"ollama": {
		"llama3", "llama3.1", "mistral", "qwen2.5",
	},
	"zhipu": {
		"glm-5", "glm-5-free", "glm-4.7", "glm-4.6",
	},
	"qwen": {
		"qwen-plus", "qwen-turbo", "qwen-max",
	},
	"moonshot": {
		"moonshot-v1-8k", "moonshot-v1-32k", "moonshot-v1-128k",
	},
	"groq": {
		"llama-3.3-70b", "llama-3.1-70b", "mixtral-8x7b",
	},
	"opencode": {
		"glm-5-free", "glm-5", "glm-4.7", "glm-4.6",
		"minimax-m2.5-free", "minimax-m2.5", "minimax-m2.1",
		"kimi-k2.5-free", "kimi-k2.5", "kimi-k2", "kimi-k2-thinking",
		"qwen3-coder", "big-pickle",
		"gpt-5.2", "gpt-5.1", "gpt-5", "gpt-5-nano",
		"claude-opus-4-6", "claude-sonnet-4-6", "claude-haiku-4-5",
		"gemini-3-pro", "gemini-3-flash",
	},
}

var channelList = []string{
	"CLI (terminal)",
	"Telegram",
	"Discord",
	"WhatsApp",
	"Slack",
	"Feishu (飞书)",
	"DingTalk (钉钉)",
	"QQ",
	"OneBot",
	"WeCom (企业微信)",
	"MaixCam",
}

func modelsToNames(list []config.ModelConfig) []string {
	result := make([]string, len(list))
	for i, m := range list {
		result[i] = m.ModelName
	}
	return result
}

func readLine(prompt string) string {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}

func promptYesNo(prompt string) bool {
	fmt.Print(prompt + " (y/n): ")
	reader := bufio.NewReader(os.Stdin)
	for {
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

func promptChoice(options []string) int {
	return promptChoiceTUI(options)
}

func promptSelect(prompt, provider string) string {
	models := providerModels[provider]
	return promptSelectSimple(prompt, models)
}

func promptSelectIndex(prompt string, options []string) int {
	return promptSelectIndexSimple(prompt, options)
}

func promptMultiSelect(options []string) []int {
	return promptMultiSelectSimple(options)
}

func promptMultiSelectIndex(options []string) []int {
	return promptMultiSelectIndexSimple(options)
}

func promptChoiceSimple(options []string) int {
	for i, opt := range options {
		fmt.Printf("  %d) %s\n", i+1, opt)
	}
	fmt.Print("Enter choice: ")
	var n int
	fmt.Scanln(&n)
	if n < 1 || n > len(options) {
		return 0
	}
	return n - 1
}

func promptSelectSimple(prompt string, models []string) string {
	fmt.Println(prompt)
	for i, m := range models {
		fmt.Printf("  %d) %s\n", i+1, m)
	}
	fmt.Print("Select: ")
	var n int
	fmt.Scanln(&n)
	if n < 1 || n > len(models) {
		return models[0]
	}
	return models[n-1]
}

func promptSelectIndexSimple(prompt string, options []string) int {
	fmt.Println(prompt)
	for i, opt := range options {
		fmt.Printf("  %d) %s\n", i+1, opt)
	}
	fmt.Print("Select: ")
	var n int
	fmt.Scanln(&n)
	if n < 1 || n > len(options) {
		return 0
	}
	return n - 1
}

func promptMultiSelectSimple(options []string) []int {
	for i, opt := range options {
		fmt.Printf("  %d) %s\n", i+1, opt)
	}
	fmt.Print("Enter numbers (comma-separated): ")
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')

	var result []int
	for _, part := range strings.Split(line, ",") {
		var n int
		fmt.Sscanf(strings.TrimSpace(part), "%d", &n)
		if n >= 1 && n <= len(options) {
			result = append(result, n-1)
		}
	}
	return result
}

func promptMultiSelectIndexSimple(options []string) []int {
	return promptMultiSelectSimple(options)
}

func promptChoiceTUI(options []string) int {
	for {
		fmt.Println("\nUse arrow keys to navigate, Enter to select.")
		fmt.Println("(or press 1-" + fmt.Sprint(len(options)) + " then Enter)")
		fmt.Println("")
		for i, opt := range options {
			fmt.Printf("  %d) %s\n", i+1, opt)
		}
		fmt.Print("> ")

		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)

		if line != "" {
			var n int
			if _, err := fmt.Sscanf(line, "%d", &n); err == nil {
				if n >= 1 && n <= len(options) {
					return n - 1
				}
			}
			fmt.Println("Invalid choice, try again.")
			continue
		}
		return 0
	}
}

func promptSelectTUI(prompt string, models []string) int {
	for {
		fmt.Println("\n" + prompt)
		fmt.Println("(enter number 1-" + fmt.Sprint(len(models)) + ")")
		fmt.Println("")
		for i, m := range models {
			fmt.Printf("  %d) %s\n", i+1, m)
		}
		fmt.Print("> ")

		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)

		if line != "" {
			var n int
			if _, err := fmt.Sscanf(line, "%d", &n); err == nil {
				if n >= 1 && n <= len(models) {
					return n - 1
				}
			}
			fmt.Println("Invalid choice, try again.")
			continue
		}
		return 0
	}
}

func promptMultiSelectTUI(options []string) []int {
	selected := make([]bool, len(options))

	for {
		fmt.Println("\nSelect (space to toggle, Enter to confirm)")
		fmt.Println("(or enter numbers separated by commas, e.g., 1,3,5)")
		fmt.Println("")
		for i, opt := range options {
			mark := " "
			if selected[i] {
				mark = "x"
			}
			fmt.Printf("  [%s] %d) %s\n", mark, i+1, opt)
		}
		fmt.Print("> ")

		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)

		if line != "" {
			selected = make([]bool, len(options))
			for _, part := range strings.Split(line, ",") {
				var n int
				if _, err := fmt.Sscanf(strings.TrimSpace(part), "%d", &n); err == nil {
					if n >= 1 && n <= len(options) {
						selected[n-1] = true
					}
				}
			}
			var result []int
			for i, s := range selected {
				if s {
					result = append(result, i)
				}
			}
			if len(result) > 0 {
				return result
			}
		}
		fmt.Println("Invalid selection, try again.")
	}
}

func promptMultiSelectIndexTUI(options []string) []int {
	return promptMultiSelectTUI(options)
}

func clearScreen() {
	fmt.Print("\033[2J\033[H")
}

func getWorkspacePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".picoclaw", "workspace")
}

func createWorkspaceTemplates(workspace string) {
	err := copyEmbeddedToTarget(workspace)
	if err != nil {
		fmt.Printf("Error copying workspace templates: %v\n", err)
	}
}

func copyEmbeddedToTarget(targetDir string) error {
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	err := fs.WalkDir(embeddedWorkspace, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if path == "." {
			return nil
		}
		data, err := embeddedWorkspace.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read embedded file %s: %w", path, err)
		}
		targetPath := filepath.Join(targetDir, path)
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", filepath.Dir(targetPath), err)
		}
		if err := os.WriteFile(targetPath, data, 0o644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", targetPath, err)
		}
		return nil
	})
	return err
}
