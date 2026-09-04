package services

import (
	"context"
	"net/http"
	"time"
)

// BotHealthChecker reports whether apps/bookshelf-bot's own process is up
// and reachable — distinct from TelegramNotifier, which only confirms
// backend-to-Telegram delivery works, not whether the bot's long-polling
// process (the thing that handles /start) is actually running.
type BotHealthChecker interface {
	// Configured reports whether a health-check URL was set at all — lets
	// the admin UI distinguish "not set up" from "set up but down".
	Configured() bool
	// Online performs a live check against the bot's health endpoint.
	Online(ctx context.Context) bool
}

// TelegramBotHealthChecker checks apps/bookshelf-bot's GET /health endpoint
// (see libs/telegram-bot-shared's health server) over plain HTTP.
type TelegramBotHealthChecker struct {
	url    string
	client *http.Client
}

// NewTelegramBotHealthChecker creates a TelegramBotHealthChecker. An empty
// url means the check is unconfigured — Online always reports false without
// making a request.
func NewTelegramBotHealthChecker(url string) *TelegramBotHealthChecker {
	return &TelegramBotHealthChecker{url: url, client: &http.Client{Timeout: 3 * time.Second}}
}

func (c *TelegramBotHealthChecker) Configured() bool {
	return c.url != ""
}

func (c *TelegramBotHealthChecker) Online(ctx context.Context) bool {
	if c.url == "" {
		return false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return false
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close() //nolint:errcheck
	return resp.StatusCode == http.StatusOK
}
