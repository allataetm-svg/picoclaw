package channels

import (
	"context"
	"strings"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/pairing"
)

type Channel interface {
	Name() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Send(ctx context.Context, msg bus.OutboundMessage) error
	IsRunning() bool
	IsAllowed(senderID string) bool
}

type BaseChannel struct {
	config      any
	bus         *bus.MessageBus
	running     bool
	name        string
	allowList   []string
	pairingMgr  *pairing.PairingManager
	pairingMode bool
}

func NewBaseChannel(name string, config any, bus *bus.MessageBus, allowList []string) *BaseChannel {
	return &BaseChannel{
		config:    config,
		bus:       bus,
		name:      name,
		allowList: allowList,
		running:   false,
	}
}

func (c *BaseChannel) SetPairingManager(pm *pairing.PairingManager, mode bool) {
	c.pairingMgr = pm
	c.pairingMode = mode
}

func (c *BaseChannel) IsPaired(senderID string) bool {
	if c.pairingMgr == nil || !c.pairingMode {
		return true
	}
	return c.pairingMgr.IsApproved(c.name, senderID)
}

func (c *BaseChannel) Send(ctx context.Context, msg bus.OutboundMessage) error {
	return nil
}

func (c *BaseChannel) Name() string {
	return c.name
}

func (c *BaseChannel) IsRunning() bool {
	return c.running
}

func (c *BaseChannel) IsAllowed(senderID string) bool {
	if len(c.allowList) == 0 {
		return true
	}

	// Extract parts from compound senderID like "123456|username"
	idPart := senderID
	userPart := ""
	if idx := strings.Index(senderID, "|"); idx > 0 {
		idPart = senderID[:idx]
		userPart = senderID[idx+1:]
	}

	for _, allowed := range c.allowList {
		// Strip leading "@" from allowed value for username matching
		trimmed := strings.TrimPrefix(allowed, "@")
		allowedID := trimmed
		allowedUser := ""
		if idx := strings.Index(trimmed, "|"); idx > 0 {
			allowedID = trimmed[:idx]
			allowedUser = trimmed[idx+1:]
		}

		// Support either side using "id|username" compound form.
		// This keeps backward compatibility with legacy Telegram allowlist entries.
		if senderID == allowed ||
			idPart == allowed ||
			senderID == trimmed ||
			idPart == trimmed ||
			idPart == allowedID ||
			(allowedUser != "" && senderID == allowedUser) ||
			(userPart != "" && (userPart == allowed || userPart == trimmed || userPart == allowedUser)) {
			return true
		}
	}

	return false
}

func (c *BaseChannel) HandleMessage(senderID, chatID, content string, media []string, metadata map[string]string) {
	if !c.IsAllowed(senderID) {
		return
	}

	if c.pairingMode && c.pairingMgr != nil && !c.pairingMgr.IsApproved(c.name, senderID) {
		c.handleUnpairedUser(senderID, chatID, content)
		return
	}

	msg := bus.InboundMessage{
		Channel:  c.name,
		SenderID: senderID,
		ChatID:   chatID,
		Content:  content,
		Media:    media,
		Metadata: metadata,
	}

	c.bus.PublishInbound(msg)
}

func (c *BaseChannel) handleUnpairedUser(senderID, chatID, content string) {
	code := c.pairingMgr.GenerateCodeForApproval(c.name, senderID, "")

	msg := bus.OutboundMessage{
		Channel: c.name,
		ChatID:  chatID,
		Content: "🔐 You are not paired with this bot.\n\n" +
			"To get access, run this command in your terminal:\n" +
			"```\npicoclaw pairing approve " + code + "\n```\n\n" +
			"This code expires in 5 minutes.",
	}

	c.bus.PublishOutbound(msg)
}

func (c *BaseChannel) setRunning(running bool) {
	c.running = running
}
