package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	// Setup logging compatible with MonsterInc
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	// Configure console output with colors
	consoleWriter := zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: "2006-01-02 15:04:05",
	}
	log.Logger = log.Output(consoleWriter)

	log.Info().Msg("Starting MonsterInc Discord Bot...")

	// Load configuration
	config, err := LoadConfig("../../configs/discord-bot-config.yaml")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	// Setup logging level from config
	level, err := zerolog.ParseLevel(config.Bot.Logging.Level)
	if err != nil {
		log.Warn().Str("level", config.Bot.Logging.Level).Msg("Invalid log level, using info")
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	// Setup file logging if specified
	if config.Bot.Logging.File != "" {
		file, err := os.OpenFile(config.Bot.Logging.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			log.Error().Err(err).Str("file", config.Bot.Logging.File).Msg("Failed to open log file, continuing with console only")
		} else {
			log.Logger = log.Output(zerolog.MultiLevelWriter(consoleWriter, file))
			log.Info().Str("file", config.Bot.Logging.File).Msg("File logging enabled")
		}
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize Discord bot
	bot, err := NewBot(config)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create Discord bot")
	}

	// Start the bot in a goroutine
	go func() {
		if err := bot.Start(ctx); err != nil {
			log.Error().Err(err).Msg("Bot stopped with error")
		}
	}()

	log.Info().Msg("Discord bot started successfully")

	// Wait for interrupt signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Info().Msg("Shutting down Discord bot...")

	// Cancel context to stop bot
	cancel()

	log.Info().Msg("Discord bot stopped")
}
