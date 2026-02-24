// PicoClaw - Ultra-lightweight personal AI agent
// License: MIT

package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/agent"
	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/channels"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/cron"
	"github.com/sipeed/picoclaw/pkg/devices"
	"github.com/sipeed/picoclaw/pkg/health"
	"github.com/sipeed/picoclaw/pkg/heartbeat"
	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/providers"
	"github.com/sipeed/picoclaw/pkg/state"
	"github.com/sipeed/picoclaw/pkg/tools"
	"github.com/sipeed/picoclaw/pkg/voice"
)

func gatewayCmd() {
	// Check for --debug flag and subcommands
	args := os.Args[2:]

	var debug bool
	var subcommand string
	for _, arg := range args {
		if arg == "--debug" || arg == "-d" {
			debug = true
			logger.SetLevel(logger.DEBUG)
			fmt.Println("🔍 Debug mode enabled")
		} else if arg == "status" || arg == "--status" || arg == "-s" {
			subcommand = "status"
		}
	}

	// Fix #671: gateway status should not start the gateway
	if subcommand == "status" {
		showGatewayStatus()
		return
	}

	if debug {
		fmt.Println("🔍 Debug mode enabled")
	}

	cfg, err := loadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	provider, modelID, err := providers.CreateProvider(cfg)
	if err != nil {
		fmt.Printf("Error creating provider: %v\n", err)
		os.Exit(1)
	}
	// Use the resolved model ID from provider creation
	if modelID != "" {
		cfg.Agents.Defaults.Model = modelID
	}

	msgBus := bus.NewMessageBus()
	agentLoop := agent.NewAgentLoop(cfg, msgBus, provider)

	// Print agent startup info
	fmt.Println("\n📦 Agent Status:")
	startupInfo := agentLoop.GetStartupInfo()
	toolsInfo := startupInfo["tools"].(map[string]any)
	skillsInfo := startupInfo["skills"].(map[string]any)
	fmt.Printf("  • Tools: %d loaded\n", toolsInfo["count"])
	fmt.Printf("  • Skills: %d/%d available\n",
		skillsInfo["available"],
		skillsInfo["total"])

	// Log to file as well
	logger.InfoCF("agent", "Agent initialized",
		map[string]any{
			"tools_count":      toolsInfo["count"],
			"skills_total":     skillsInfo["total"],
			"skills_available": skillsInfo["available"],
		})

	// Setup cron tool and service
	execTimeout := time.Duration(cfg.Tools.Cron.ExecTimeoutMinutes) * time.Minute
	cronService := setupCronTool(
		agentLoop,
		msgBus,
		cfg.WorkspacePath(),
		cfg.Agents.Defaults.RestrictToWorkspace,
		execTimeout,
		cfg,
	)

	heartbeatService := heartbeat.NewHeartbeatService(
		cfg.WorkspacePath(),
		cfg.Heartbeat.Interval,
		cfg.Heartbeat.Enabled,
	)
	heartbeatService.SetBus(msgBus)
	heartbeatService.SetHandler(func(prompt, channel, chatID string) *tools.ToolResult {
		// Use cli:direct as fallback if no valid channel
		if channel == "" || chatID == "" {
			channel, chatID = "cli", "direct"
		}
		// Use ProcessHeartbeat - no session history, each heartbeat is independent
		var response string
		response, err = agentLoop.ProcessHeartbeat(context.Background(), prompt, channel, chatID)
		if err != nil {
			return tools.ErrorResult(fmt.Sprintf("Heartbeat error: %v", err))
		}
		if response == "HEARTBEAT_OK" {
			return tools.SilentResult("Heartbeat OK")
		}
		// For heartbeat, always return silent - the subagent result will be
		// sent to user via processSystemMessage when the async task completes
		return tools.SilentResult(response)
	})

	channelManager, err := channels.NewManager(cfg, msgBus)
	if err != nil {
		fmt.Printf("Error creating channel manager: %v\n", err)
		os.Exit(1)
	}

	// Inject channel manager into agent loop for command handling
	agentLoop.SetChannelManager(channelManager)

	var transcriber *voice.GroqTranscriber
	groqAPIKey := cfg.Providers.Groq.APIKey
	if groqAPIKey == "" {
		for _, mc := range cfg.ModelList {
			if strings.HasPrefix(mc.Model, "groq/") && mc.APIKey != "" {
				groqAPIKey = mc.APIKey
				break
			}
		}
	}
	if groqAPIKey != "" {
		transcriber = voice.NewGroqTranscriber(groqAPIKey)
		logger.InfoC("voice", "Groq voice transcription enabled")
	}

	if transcriber != nil {
		if telegramChannel, ok := channelManager.GetChannel("telegram"); ok {
			if tc, ok := telegramChannel.(*channels.TelegramChannel); ok {
				tc.SetTranscriber(transcriber)
				logger.InfoC("voice", "Groq transcription attached to Telegram channel")
			}
		}
		if discordChannel, ok := channelManager.GetChannel("discord"); ok {
			if dc, ok := discordChannel.(*channels.DiscordChannel); ok {
				dc.SetTranscriber(transcriber)
				logger.InfoC("voice", "Groq transcription attached to Discord channel")
			}
		}
		if slackChannel, ok := channelManager.GetChannel("slack"); ok {
			if sc, ok := slackChannel.(*channels.SlackChannel); ok {
				sc.SetTranscriber(transcriber)
				logger.InfoC("voice", "Groq transcription attached to Slack channel")
			}
		}
	}

	enabledChannels := channelManager.GetEnabledChannels()
	if len(enabledChannels) > 0 {
		fmt.Printf("✓ Channels enabled: %s\n", enabledChannels)
	} else {
		fmt.Println("⚠ Warning: No channels enabled")
	}

	fmt.Printf("✓ Gateway started on %s:%d\n", cfg.Gateway.Host, cfg.Gateway.Port)
	fmt.Println("Press Ctrl+C to stop")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := cronService.Start(); err != nil {
		fmt.Printf("Error starting cron service: %v\n", err)
	}
	fmt.Println("✓ Cron service started")

	if err := heartbeatService.Start(); err != nil {
		fmt.Printf("Error starting heartbeat service: %v\n", err)
	}
	fmt.Println("✓ Heartbeat service started")

	stateManager := state.NewManager(cfg.WorkspacePath())
	deviceService := devices.NewService(devices.Config{
		Enabled:    cfg.Devices.Enabled,
		MonitorUSB: cfg.Devices.MonitorUSB,
	}, stateManager)
	deviceService.SetBus(msgBus)
	if err := deviceService.Start(ctx); err != nil {
		fmt.Printf("Error starting device service: %v\n", err)
	} else if cfg.Devices.Enabled {
		fmt.Println("✓ Device event service started")
	}

	if err := channelManager.StartAll(ctx); err != nil {
		fmt.Printf("Error starting channels: %v\n", err)
	}

	healthServer := health.NewServer(cfg.Gateway.Host, cfg.Gateway.Port)
	go func() {
		if err := healthServer.Start(); err != nil && err != http.ErrServerClosed {
			logger.ErrorCF("health", "Health server error", map[string]any{"error": err.Error()})
		}
	}()
	fmt.Printf("✓ Health endpoints available at http://%s:%d/health and /ready\n", cfg.Gateway.Host, cfg.Gateway.Port)

	go agentLoop.Run(ctx)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	<-sigChan

	fmt.Println("\nShutting down...")
	cancel()
	healthServer.Stop(context.Background())
	deviceService.Stop()
	heartbeatService.Stop()
	cronService.Stop()
	agentLoop.Stop()
	channelManager.StopAll(ctx)
	fmt.Println("✓ Gateway stopped")
}

func setupCronTool(
	agentLoop *agent.AgentLoop,
	msgBus *bus.MessageBus,
	workspace string,
	restrict bool,
	execTimeout time.Duration,
	cfg *config.Config,
) *cron.CronService {
	cronStorePath := filepath.Join(workspace, "cron", "jobs.json")

	// Create cron service
	cronService := cron.NewCronService(cronStorePath, nil)

	// Create and register CronTool
	cronTool := tools.NewCronTool(cronService, agentLoop, msgBus, workspace, restrict, execTimeout, cfg)
	agentLoop.RegisterTool(cronTool)

	// Set the onJob handler
	cronService.SetOnJob(func(job *cron.CronJob) (string, error) {
		result := cronTool.ExecuteJob(context.Background(), job)
		return result, nil
	})

	return cronService
}

// showGatewayStatus shows gateway status without starting the gateway
// Fix for issue #671: gateway status should not launch an extra worker
func showGatewayStatus() {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("📦 PicoClaw Gateway Status")
	fmt.Println("==============================")
	fmt.Printf("Config: %s\n", getConfigPath())
	fmt.Printf("Workspace: %s\n", cfg.WorkspacePath())
	fmt.Printf("Model: %s\n", cfg.Agents.Defaults.Model)
	fmt.Printf("Max Tokens: %d\n", cfg.Agents.Defaults.MaxTokens)
	fmt.Printf("Temperature: %v\n", cfg.Agents.Defaults.Temperature)
	fmt.Printf("Max Tool Iterations: %d\n", cfg.Agents.Defaults.MaxToolIterations)

	fmt.Println("\n📡 Channels:")
	channels := []string{}
	if cfg.Channels.Telegram.Enabled {
		channels = append(channels, "Telegram")
	}
	if cfg.Channels.Discord.Enabled {
		channels = append(channels, "Discord")
	}
	if cfg.Channels.WhatsApp.Enabled {
		channels = append(channels, "WhatsApp")
	}
	if cfg.Channels.Slack.Enabled {
		channels = append(channels, "Slack")
	}
	if cfg.Channels.Feishu.Enabled {
		channels = append(channels, "Feishu")
	}
	if cfg.Channels.DingTalk.Enabled {
		channels = append(channels, "DingTalk")
	}
	if cfg.Channels.QQ.Enabled {
		channels = append(channels, "QQ")
	}
	if cfg.Channels.OneBot.Enabled {
		channels = append(channels, "OneBot")
	}
	if cfg.Channels.WeCom.Enabled {
		channels = append(channels, "WeCom")
	}
	if cfg.Channels.MaixCam.Enabled {
		channels = append(channels, "MaixCam")
	}

	if len(channels) == 0 {
		fmt.Println("  (no channels enabled)")
	} else {
		for _, ch := range channels {
			fmt.Printf("  • %s\n", ch)
		}
	}

	fmt.Println("\n🛠️ Tools:")
	fmt.Printf("  Web Search: %v\n", cfg.Tools.Web.DuckDuckGo.Enabled)
	fmt.Printf("  Cron: enabled\n")
	fmt.Printf("  Exec: enabled\n")
	fmt.Printf("  Skills: enabled\n")

	fmt.Println("\n❤️ Heartbeat:")
	fmt.Printf("  Enabled: %v\n", cfg.Heartbeat.Enabled)
	if cfg.Heartbeat.Enabled {
		fmt.Printf("  Interval: %d minutes\n", cfg.Heartbeat.Interval)
	}

	fmt.Println("\n✅ Run 'picoclaw gateway' to start the gateway")
}
