package discord

import (
	"testing"
	"time"
)

func TestDiscordEmbedBuilder_Build(t *testing.T) {
	embed := NewDiscordEmbedBuilder().
		WithTitle("Test").
		WithDescription("Description").
		WithTimestamp(time.Now()).
		WithColor(0x00FF00).
		Build()

	if embed.Title != "Test" {
		t.Errorf("expected title 'Test', got '%s'", embed.Title)
	}
	if embed.Description != "Description" {
		t.Errorf("expected description, got '%s'", embed.Description)
	}
	if embed.Timestamp == "" {
		t.Error("expected timestamp to be set")
	}
}
