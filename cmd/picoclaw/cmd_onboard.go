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

	"github.com/charmbracelet/huh"
	"github.com/sipeed/picoclaw/pkg/config"
)

//go:generate cp -r ../../workspace workspace
//go:embed workspace
var embeddedWorkspace embed.FS

func onboard() {
	configPath := getConfigPath()

	if _, err := os.Stat(configPath); err == nil {
		var overwrite bool
		huh.NewConfirm().
			Title("Config already exists").
			Description(fmt.Sprintf("Overwrite existing config at %s?", configPath)).
			Affirmative("Yes, overwrite").
			Negative("No, cancel").
			Value(&overwrite).
			Run()

		if !overwrite {
			fmt.Println("Aborted.")
			return
		}
	}

	cfg := config.DefaultConfig()

	var setupMode int
	huh.NewSelect[int]().
		Title("PicoClaw Setup").
		Options(
			huh.NewOption("Quickstart - One provider + one channel", 0),
			huh.NewOption("Advanced - Multiple providers, models, channels", 1),
		).
		Value(&setupMode).
		Run()

	if setupMode == 0 {
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

	fmt.Printf("\n✅ Config saved to %s\n", configPath)
	fmt.Printf("%s picoclaw is ready!\n", logo)
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Chat: picoclaw agent -m \"Hello!\"")
	fmt.Println("  2. Or run: picoclaw agent (interactive mode)")
}

func quickstartSetup(cfg *config.Config) {
	fmt.Println("\n--- Quickstart Setup ---")

	var providerIdx int
	huh.NewSelect[int]().
		Title("Select Provider").
		Options(
			huh.NewOption("OpenAI", 0),
			huh.NewOption("Anthropic", 1),
			huh.NewOption("Google", 2),
			huh.NewOption("DeepSeek", 3),
			huh.NewOption("OpenRouter", 4),
			huh.NewOption("Ollama", 5),
			huh.NewOption("Zhipu", 6),
			huh.NewOption("Qwen", 7),
			huh.NewOption("Moonshot", 8),
			huh.NewOption("Groq", 9),
			huh.NewOption("OpenCode", 10),
		).
		Value(&providerIdx).
		Run()

	provider := providerList[providerIdx]

	var apiKey string
	huh.NewInput().
		Title(fmt.Sprintf("Enter API key for %s", provider)).
		Placeholder("sk-...").
		EchoMode(huh.EchoModePassword).
		Value(&apiKey).
		Run()

	model := promptSelectModel(provider)

	var channelIdx int
	huh.NewSelect[int]().
		Title("Select Channel").
		Options(
			huh.NewOption("CLI (terminal only)", 0),
			huh.NewOption("Telegram", 1),
		).
		Value(&channelIdx).
		Run()

	cfg.ProviderGroups = map[string]config.ProviderGroupConfig{
		"opencode": {
			APIKey:  apiKey,
			APIBase: "https://opencode.ai/zen",
			Models:  []string{model},
		},
	}

	cfg.Agents.Defaults.Model = model

	if channelIdx == 1 {
		cfg.Channels.Telegram.Enabled = true
		var token string
		huh.NewInput().
			Title("Enter Telegram bot token").
			Placeholder("123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11").
			Value(&token).
			Run()
		cfg.Channels.Telegram.Token = token
		cfg.Channels.Telegram.AllowFrom = config.FlexibleStringSlice{}
	}
}

func advancedSetup(cfg *config.Config) {
	fmt.Println("\n--- Advanced Setup ---")

	fmt.Println("\n=== Providers & Models ===")

	cfg.ProviderGroups = map[string]config.ProviderGroupConfig{}

	var addMore bool
	for {
		var providerIdx int
		huh.NewSelect[int]().
			Title("Select provider to add (or Done to finish)").
			Options(
				huh.NewOption("Done adding providers", 99),
				huh.NewOption("OpenAI", 0),
				huh.NewOption("Anthropic", 1),
				huh.NewOption("Google", 2),
				huh.NewOption("DeepSeek", 3),
				huh.NewOption("OpenRouter", 4),
				huh.NewOption("Ollama", 5),
				huh.NewOption("Zhipu", 6),
				huh.NewOption("Qwen", 7),
				huh.NewOption("Moonshot", 8),
				huh.NewOption("Groq", 9),
				huh.NewOption("OpenCode", 10),
			).
			Value(&providerIdx).
			Run()

		if providerIdx == 99 {
			break
		}

		provider := providerList[providerIdx]
		models := providerModels[provider]

		fmt.Println()
		var apiKey string
		huh.NewInput().
			Title(fmt.Sprintf("API key for %s (%d models)", provider, len(models))).
			Placeholder("sk-...").
			EchoMode(huh.EchoModePassword).
			Value(&apiKey).
			Run()

		cfg.ProviderGroups[provider] = config.ProviderGroupConfig{
			APIKey:  apiKey,
			APIBase: "https://opencode.ai/zen",
			Models:  models,
		}

		huh.NewConfirm().
			Title("Add another provider?").
			Affirmative("Yes").
			Negative("No, done").
			Value(&addMore).
			Run()

		if !addMore {
			break
		}
	}

	allModels := collectAllModels(cfg.ProviderGroups)
	if len(allModels) > 0 {
		cfg.Agents.Defaults.Model = allModels[0]

		if len(allModels) > 1 {
			var defaultIdx int
			huh.NewSelect[int]().
				Title("Select default model").
				Options(huhOptionsFromSlice(allModels)...).
				Value(&defaultIdx).
				Run()
			cfg.Agents.Defaults.Model = allModels[defaultIdx]

			var useFallbacks bool
			huh.NewConfirm().
				Title("Add model fallbacks?").
				Affirmative("Yes").
				Negative("No").
				Value(&useFallbacks).
				Run()

			if useFallbacks {
				var fallbacks []int
				huh.NewMultiSelect[int]().
					Title("Select fallback models (space to toggle, enter to confirm)").
					Options(huhOptionsFromSlice(allModels)...).
					Value(&fallbacks).
					Run()
				for _, fi := range fallbacks {
					if fi != defaultIdx {
						cfg.Agents.Defaults.ModelFallbacks = append(cfg.Agents.Defaults.ModelFallbacks, allModels[fi])
					}
				}
			}
		}
	}

	fmt.Println("\n=== Channels ===")

	var selectedChannels []int
	huh.NewMultiSelect[int]().
		Title("Select channels (space to toggle, enter to confirm)").
		Options(
			huh.NewOption("CLI (terminal)", 0),
			huh.NewOption("Telegram", 1),
			huh.NewOption("Discord", 2),
			huh.NewOption("WhatsApp", 3),
			huh.NewOption("Slack", 4),
			huh.NewOption("Feishu (飞书)", 5),
			huh.NewOption("DingTalk (钉钉)", 6),
			huh.NewOption("QQ", 7),
			huh.NewOption("OneBot", 8),
			huh.NewOption("WeCom (企业微信)", 9),
			huh.NewOption("MaixCam", 10),
		).
		Value(&selectedChannels).
		Run()

	for _, ci := range selectedChannels {
		switch ci {
		case 0:
			fmt.Println("  CLI is always available")
		case 1:
			cfg.Channels.Telegram.Enabled = true
			var token string
			huh.NewInput().
				Title("Telegram bot token").
				Placeholder("123456:ABC-DEF...").
				Value(&token).
				Run()
			cfg.Channels.Telegram.Token = token
			cfg.Channels.Telegram.AllowFrom = config.FlexibleStringSlice{}
		case 2:
			cfg.Channels.Discord.Enabled = true
			var token string
			huh.NewInput().
				Title("Discord bot token").
				Placeholder("MTk...").
				Value(&token).
				Run()
			cfg.Channels.Discord.Token = token
			cfg.Channels.Discord.AllowFrom = config.FlexibleStringSlice{}
		case 3:
			cfg.Channels.WhatsApp.Enabled = true
			var url string
			huh.NewInput().
				Title("WhatsApp bridge URL").
				Placeholder("ws://localhost:3001").
				Value(&url).
				Run()
			if url == "" {
				url = "ws://localhost:3001"
			}
			cfg.Channels.WhatsApp.BridgeURL = url
			cfg.Channels.WhatsApp.AllowFrom = config.FlexibleStringSlice{}
		case 4:
			cfg.Channels.Slack.Enabled = true
			var botToken, appToken string
			huh.NewInput().
				Title("Slack bot token (xoxb-...)").
				Value(&botToken).
				Run()
			huh.NewInput().
				Title("Slack app token (xapp-...)").
				Value(&appToken).
				Run()
			cfg.Channels.Slack.BotToken = botToken
			cfg.Channels.Slack.AppToken = appToken
			cfg.Channels.Slack.AllowFrom = config.FlexibleStringSlice{}
		case 5:
			cfg.Channels.Feishu.Enabled = true
			var appID, appSecret string
			huh.NewInput().
				Title("Feishu app ID").
				Value(&appID).
				Run()
			huh.NewInput().
				Title("Feishu app secret").
				Value(&appSecret).
				Run()
			cfg.Channels.Feishu.AppID = appID
			cfg.Channels.Feishu.AppSecret = appSecret
			cfg.Channels.Feishu.AllowFrom = config.FlexibleStringSlice{}
		case 6:
			cfg.Channels.DingTalk.Enabled = true
			var clientID, clientSecret string
			huh.NewInput().
				Title("DingTalk client ID").
				Value(&clientID).
				Run()
			huh.NewInput().
				Title("DingTalk client secret").
				Value(&clientSecret).
				Run()
			cfg.Channels.DingTalk.ClientID = clientID
			cfg.Channels.DingTalk.ClientSecret = clientSecret
			cfg.Channels.DingTalk.AllowFrom = config.FlexibleStringSlice{}
		case 7:
			cfg.Channels.QQ.Enabled = true
			var appID, appSecret string
			huh.NewInput().
				Title("QQ app ID").
				Value(&appID).
				Run()
			huh.NewInput().
				Title("QQ app secret").
				Value(&appSecret).
				Run()
			cfg.Channels.QQ.AppID = appID
			cfg.Channels.QQ.AppSecret = appSecret
			cfg.Channels.QQ.AllowFrom = config.FlexibleStringSlice{}
		case 8:
			cfg.Channels.OneBot.Enabled = true
			var wsUrl, accessToken string
			huh.NewInput().
				Title("OneBot WebSocket URL").
				Placeholder("ws://127.0.0.1:3001").
				Value(&wsUrl).
				Run()
			if wsUrl == "" {
				wsUrl = "ws://127.0.0.1:3001"
			}
			huh.NewInput().
				Title("OneBot access token (optional)").
				Value(&accessToken).
				Run()
			cfg.Channels.OneBot.WSUrl = wsUrl
			cfg.Channels.OneBot.AccessToken = accessToken
			cfg.Channels.OneBot.AllowFrom = config.FlexibleStringSlice{}
		case 9:
			cfg.Channels.WeCom.Enabled = true
			var token, webhookURL string
			huh.NewInput().
				Title("WeCom webhook token").
				Value(&token).
				Run()
			huh.NewInput().
				Title("WeCom webhook URL").
				Value(&webhookURL).
				Run()
			cfg.Channels.WeCom.Token = token
			cfg.Channels.WeCom.WebhookURL = webhookURL
			cfg.Channels.WeCom.AllowFrom = config.FlexibleStringSlice{}
		case 10:
			cfg.Channels.MaixCam.Enabled = true
			var host string
			huh.NewInput().
				Title("MaixCam host").
				Placeholder("0.0.0.0").
				Value(&host).
				Run()
			if host == "" {
				host = "0.0.0.0"
			}
			cfg.Channels.MaixCam.Host = host
			cfg.Channels.MaixCam.AllowFrom = config.FlexibleStringSlice{}
		}
	}
}

func huhOptionsFromSlice(options []string) []huh.Option[int] {
	opts := make([]huh.Option[int], len(options))
	for i, opt := range options {
		opts[i] = huh.NewOption(opt, i)
	}
	return opts
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

func promptSelectModel(provider string) string {
	models := providerModels[provider]
	var idx int
	huh.NewSelect[int]().
		Title(fmt.Sprintf("Select model for %s", provider)).
		Options(huhOptionsFromSlice(models)...).
		Value(&idx).
		Run()
	return models[idx]
}

func readLine(prompt string) string {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
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

func collectAllModels(groups map[string]config.ProviderGroupConfig) []string {
	var all []string
	for _, g := range groups {
		all = append(all, g.Models...)
	}
	return all
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
