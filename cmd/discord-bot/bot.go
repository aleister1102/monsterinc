package main

import (
	"context"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"
)

// Bot represents the Discord bot instance
type Bot struct {
	session        *discordgo.Session
	config         *Config
	fileManager    *FileManager
	serviceMonitor *ServiceMonitor
	rateLimiter    *rate.Limiter
}

// NewBot creates a new Discord bot instance
func NewBot(config *Config) (*Bot, error) {
	// Create Discord session
	session, err := discordgo.New("Bot " + config.Discord.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to create Discord session: %w", err)
	}

	// Create file manager
	fileManager, err := NewFileManager(config.Paths, log.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create file manager: %w", err)
	}

	// Create service monitor
	serviceMonitor := NewServiceMonitor(config)

	// Create rate limiter (10 requests per minute)
	rateLimiter := rate.NewLimiter(rate.Every(6*time.Second), 10)

	bot := &Bot{
		session:        session,
		config:         config,
		fileManager:    fileManager,
		serviceMonitor: serviceMonitor,
		rateLimiter:    rateLimiter,
	}

	// Set up event handlers
	bot.setupEventHandlers()

	return bot, nil
}

// setupEventHandlers configures Discord event handlers
func (b *Bot) setupEventHandlers() {
	b.session.AddHandler(b.onReady)
	b.session.AddHandler(b.onInteractionCreate)
}

// onReady handles the ready event
func (b *Bot) onReady(s *discordgo.Session, event *discordgo.Ready) {
	log.Info().
		Str("username", event.User.Username).
		Str("discriminator", event.User.Discriminator).
		Msg("Discord bot is ready")

	// Register slash commands
	if err := registerCommands(s, b.config.Discord.GuildID); err != nil {
		log.Error().Err(err).Msg("Failed to register commands")
	}
}

// onInteractionCreate handles interaction events
func (b *Bot) onInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Rate limiting
	if !b.rateLimiter.Allow() {
		log.Warn().Msg("Rate limit exceeded for interaction")
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "⚠️ Rate limit exceeded. Please wait before sending another command.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	// Handle slash commands
	if i.ApplicationCommandData().Name != "" {
		b.handleCommands(s, i)
	}
}

// Start starts the Discord bot
func (b *Bot) Start(ctx context.Context) error {
	// Set bot status
	err := b.session.Open()
	if err != nil {
		return fmt.Errorf("failed to open Discord session: %w", err)
	}

	log.Info().Msg("Discord bot started successfully")

	// Set status
	err = b.session.UpdateGameStatus(0, "MonsterInc Manager")
	if err != nil {
		log.Warn().Err(err).Msg("Failed to set bot status")
	}

	// Wait for context cancellation
	<-ctx.Done()

	log.Info().Msg("Shutting down Discord bot...")

	// Clean up commands
	cleanupCommands(b.session, b.config.Discord.GuildID)

	// Close session
	return b.session.Close()
}

// handleCommands routes command interactions to appropriate handlers
func (b *Bot) handleCommands(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.ApplicationCommandData().Name == "" {
		return
	}

	// Defer response to avoid timeout
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to defer interaction response")
		return
	}

	var response string
	var err2 error

	switch i.ApplicationCommandData().Name {
	case "add-url":
		response, err2 = b.handleAddURL(i.ApplicationCommandData().Options)
	case "add-js-html":
		response, err2 = b.handleAddJSHTML(i.ApplicationCommandData().Options)
	case "list-urls":
		response, err2 = b.handleListURLs(i.ApplicationCommandData().Options)
	case "list-js-html":
		response, err2 = b.handleListJSHTML(i.ApplicationCommandData().Options)
	case "remove-url":
		response, err2 = b.handleRemoveURL(i.ApplicationCommandData().Options)
	case "remove-js-html":
		response, err2 = b.handleRemoveJSHTML(i.ApplicationCommandData().Options)
	case "update-url":
		response, err2 = b.handleUpdateURL(i.ApplicationCommandData().Options)
	case "update-js-html":
		response, err2 = b.handleUpdateJSHTML(i.ApplicationCommandData().Options)
	case "scan-onetime":
		response, err2 = b.handleScanOnetime()
	case "status":
		response, err2 = b.handleStatus()
	default:
		response = "Unknown command"
	}

	if err2 != nil {
		response = fmt.Sprintf("Error: %v", err2)
		log.Error().Err(err2).Str("command", i.ApplicationCommandData().Name).Msg("Command execution failed")
	}

	// Send follow-up response
	_, err = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: response,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to send follow-up message")
	}
}

// Command handler methods
func (b *Bot) handleAddURL(options []*discordgo.ApplicationCommandInteractionDataOption) (string, error) {
	if len(options) == 0 {
		return "", fmt.Errorf("URL parameter is required")
	}

	url := options[0].StringValue()
	if url == "" {
		return "", fmt.Errorf("URL cannot be empty")
	}

	err := b.fileManager.AddURL("urls", url)
	if err != nil {
		return "", fmt.Errorf("failed to add URL: %v", err)
	}

	return fmt.Sprintf("✅ Successfully added URL to urls.txt: `%s`", url), nil
}

func (b *Bot) handleAddJSHTML(options []*discordgo.ApplicationCommandInteractionDataOption) (string, error) {
	if len(options) == 0 {
		return "", fmt.Errorf("URL parameter is required")
	}

	url := options[0].StringValue()
	if url == "" {
		return "", fmt.Errorf("URL cannot be empty")
	}

	err := b.fileManager.AddURL("js_html", url)
	if err != nil {
		return "", fmt.Errorf("failed to add URL: %v", err)
	}

	return fmt.Sprintf("✅ Successfully added URL to js_html.txt: `%s`", url), nil
}

func (b *Bot) handleListURLs(options []*discordgo.ApplicationCommandInteractionDataOption) (string, error) {
	page := 1
	if len(options) > 0 && options[0].Name == "page" {
		page = int(options[0].IntValue())
		if page < 1 {
			page = 1
		}
	}

	content, totalPages, err := b.fileManager.FormatURLList("urls", page, 10)
	if err != nil {
		return "", fmt.Errorf("failed to read urls.txt: %v", err)
	}

	if totalPages > 1 {
		content += fmt.Sprintf("\n*Use `/list-urls page:%d` for next page*", page+1)
	}

	return content, nil
}

func (b *Bot) handleListJSHTML(options []*discordgo.ApplicationCommandInteractionDataOption) (string, error) {
	page := 1
	if len(options) > 0 && options[0].Name == "page" {
		page = int(options[0].IntValue())
		if page < 1 {
			page = 1
		}
	}

	content, totalPages, err := b.fileManager.FormatURLList("js_html", page, 10)
	if err != nil {
		return "", fmt.Errorf("failed to read js_html.txt: %v", err)
	}

	if totalPages > 1 {
		content += fmt.Sprintf("\n*Use `/list-js-html page:%d` for next page*", page+1)
	}

	return content, nil
}

func (b *Bot) handleRemoveURL(options []*discordgo.ApplicationCommandInteractionDataOption) (string, error) {
	if len(options) == 0 {
		return "", fmt.Errorf("line parameter is required")
	}

	line := int(options[0].IntValue())
	if line < 1 {
		return "", fmt.Errorf("line number must be greater than 0")
	}

	removedURL, err := b.fileManager.RemoveURL("urls", line)
	if err != nil {
		return "", fmt.Errorf("failed to remove URL: %v", err)
	}

	return fmt.Sprintf("🗑️ Successfully removed URL from line %d in urls.txt: `%s`", line, removedURL), nil
}

func (b *Bot) handleRemoveJSHTML(options []*discordgo.ApplicationCommandInteractionDataOption) (string, error) {
	if len(options) == 0 {
		return "", fmt.Errorf("line parameter is required")
	}

	line := int(options[0].IntValue())
	if line < 1 {
		return "", fmt.Errorf("line number must be greater than 0")
	}

	removedURL, err := b.fileManager.RemoveURL("js_html", line)
	if err != nil {
		return "", fmt.Errorf("failed to remove URL: %v", err)
	}

	return fmt.Sprintf("🗑️ Successfully removed URL from line %d in js_html.txt: `%s`", line, removedURL), nil
}

func (b *Bot) handleUpdateURL(options []*discordgo.ApplicationCommandInteractionDataOption) (string, error) {
	if len(options) < 2 {
		return "", fmt.Errorf("line and url parameters are required")
	}

	line := int(options[0].IntValue())
	newURL := options[1].StringValue()

	if line < 1 {
		return "", fmt.Errorf("line number must be greater than 0")
	}
	if newURL == "" {
		return "", fmt.Errorf("URL cannot be empty")
	}

	oldURL, err := b.fileManager.UpdateURL("urls", line, newURL)
	if err != nil {
		return "", fmt.Errorf("failed to update URL: %v", err)
	}

	return fmt.Sprintf("✏️ Successfully updated line %d in urls.txt:\n`%s` → `%s`", line, oldURL, newURL), nil
}

func (b *Bot) handleUpdateJSHTML(options []*discordgo.ApplicationCommandInteractionDataOption) (string, error) {
	if len(options) < 2 {
		return "", fmt.Errorf("line and url parameters are required")
	}

	line := int(options[0].IntValue())
	newURL := options[1].StringValue()

	if line < 1 {
		return "", fmt.Errorf("line number must be greater than 0")
	}
	if newURL == "" {
		return "", fmt.Errorf("URL cannot be empty")
	}

	oldURL, err := b.fileManager.UpdateURL("js_html", line, newURL)
	if err != nil {
		return "", fmt.Errorf("failed to update URL: %v", err)
	}

	return fmt.Sprintf("✏️ Successfully updated line %d in js_html.txt:\n`%s` → `%s`", line, oldURL, newURL), nil
}

func (b *Bot) handleScanOnetime() (string, error) {
	err := b.serviceMonitor.TriggerScan()
	if err != nil {
		return "", fmt.Errorf("failed to trigger scan: %v", err)
	}

	return "🚀 One-time scan triggered successfully! MonsterInc is now scanning URLs from urls.txt. Check logs for progress.", nil
}

func (b *Bot) handleStatus() (string, error) {
	status, err := b.serviceMonitor.CheckServiceStatus()
	if err != nil {
		return "", fmt.Errorf("failed to check service status: %v", err)
	}

	if !status.IsRunning {
		return "📊 **MonsterInc Service Status**\n```\nStatus: ❌ Not Running\nNo MonsterInc process found\n```", nil
	}

	uptimeStr := "Unknown"
	if status.Uptime > 0 {
		uptimeStr = formatDuration(status.Uptime)
	}

	response := fmt.Sprintf("📊 **MonsterInc Service Status**\n```\nStatus: ✅ Running\nPID: %d\nUptime: %s\nMemory: %s\n```",
		status.PID, uptimeStr, status.Memory)

	return response, nil
}

// formatDuration formats duration in human readable format
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	return fmt.Sprintf("%dd %dh", days, hours)
}

// Command definitions
var commands = []*discordgo.ApplicationCommand{
	{
		Name:        "add-url",
		Description: "Add URL to urls.txt file",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "url",
				Description: "URL to add",
				Required:    true,
			},
		},
	},
	{
		Name:        "add-js-html",
		Description: "Add URL to js_html.txt file",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "url",
				Description: "URL to add",
				Required:    true,
			},
		},
	},
	{
		Name:        "list-urls",
		Description: "List URLs from urls.txt file",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "page",
				Description: "Page number (default: 1)",
				Required:    false,
			},
		},
	},
	{
		Name:        "list-js-html",
		Description: "List URLs from js_html.txt file",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "page",
				Description: "Page number (default: 1)",
				Required:    false,
			},
		},
	},
	{
		Name:        "remove-url",
		Description: "Remove URL from urls.txt file",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "line",
				Description: "Line number to remove",
				Required:    true,
			},
		},
	},
	{
		Name:        "remove-js-html",
		Description: "Remove URL from js_html.txt file",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "line",
				Description: "Line number to remove",
				Required:    true,
			},
		},
	},
	{
		Name:        "update-url",
		Description: "Update URL in urls.txt file",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "line",
				Description: "Line number to update",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "url",
				Description: "New URL",
				Required:    true,
			},
		},
	},
	{
		Name:        "update-js-html",
		Description: "Update URL in js_html.txt file",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "line",
				Description: "Line number to update",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "url",
				Description: "New URL",
				Required:    true,
			},
		},
	},
	{
		Name:        "scan-onetime",
		Description: "Trigger one-time scan with urls.txt",
	},
	{
		Name:        "status",
		Description: "Check MonsterInc service status",
	},
}

// Register commands with Discord
func registerCommands(s *discordgo.Session, guildID string) error {
	log.Info().Msg("Registering slash commands...")

	for _, cmd := range commands {
		_, err := s.ApplicationCommandCreate(s.State.User.ID, guildID, cmd)
		if err != nil {
			return fmt.Errorf("failed to create command %s: %v", cmd.Name, err)
		}
		log.Debug().Str("command", cmd.Name).Msg("Registered command")
	}

	log.Info().Int("count", len(commands)).Msg("Successfully registered all commands")
	return nil
}

// Clean up commands on shutdown
func cleanupCommands(s *discordgo.Session, guildID string) {
	log.Info().Msg("Cleaning up slash commands...")

	commands, err := s.ApplicationCommands(s.State.User.ID, guildID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to fetch commands for cleanup")
		return
	}

	for _, cmd := range commands {
		err := s.ApplicationCommandDelete(s.State.User.ID, guildID, cmd.ID)
		if err != nil {
			log.Error().Err(err).Str("command", cmd.Name).Msg("Failed to delete command")
		} else {
			log.Debug().Str("command", cmd.Name).Msg("Deleted command")
		}
	}

	log.Info().Msg("Command cleanup completed")
}
