package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

// TelegramNotifier sends a push notification to a linked Telegram chat.
// Workflows depend on this interface rather than the concrete
// TelegramService type, so tests can inject a fake and assert on sent
// messages — mirrors SMSService's interface-plus-mock shape
// (internal/services/sms.go) rather than EmailService's concrete-struct
// shape, since (unlike email) call sites need their Telegram sends verified
// in tests.
type TelegramNotifier interface {
	NotifyAsync(ctx context.Context, chatID int64, text string)
	// Notify is the synchronous counterpart to NotifyAsync, for the one
	// call site (the "send me a test message" profile action) where the
	// caller needs to know whether delivery actually succeeded, rather
	// than firing best-effort like every event notification does.
	Notify(ctx context.Context, chatID int64, text string) error
}

// TelegramService sends messages directly to the Telegram Bot API
// (api.telegram.org) — the same "call the provider directly from the
// backend" shape EmailService already uses for SMTP, rather than routing
// through apps/bookshelf-bot. See docs/telegram-bot-integration-spec.md's
// "Architecture decision" section for why.
type TelegramService struct {
	token  string
	client *http.Client
}

// NewTelegramService creates a TelegramService. An empty token means
// Telegram notifications are silently skipped (logged) — the same contract
// EmailService has when its SMTP host is unset.
func NewTelegramService(token string) *TelegramService {
	return &TelegramService{
		token:  token,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

type sendMessageRequest struct {
	ChatID    int64  `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode"`
}

// Notify sends text to chatID via the Telegram Bot API's sendMessage
// endpoint, HTML-formatted (Telegram supports a small subset of tags:
// <b>, <i>, <a href>).
func (s *TelegramService) Notify(ctx context.Context, chatID int64, text string) error {
	if s.token == "" {
		zerolog.Ctx(ctx).Warn().Int64("chat_id", chatID).Str("text", text).
			Msg("telegram skipped: TELEGRAM_BOT_TOKEN not set — printing message here so it's still usable for local testing")
		return nil
	}

	body, err := json.Marshal(sendMessageRequest{ChatID: chatID, Text: text, ParseMode: "HTML"})
	if err != nil {
		return fmt.Errorf("telegram: marshal request: %w", err)
	}

	endpoint := "https://api.telegram.org/bot" + s.token + "/sendMessage"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("telegram: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("telegram: send: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram: sendMessage returned %d", resp.StatusCode)
	}

	zerolog.Ctx(ctx).Debug().Int64("chat_id", chatID).Msg("telegram message sent")
	return nil
}

// NotifyAsync sends a message in a background goroutine so callers on the
// HTTP request path don't block on the Telegram API round trip. Errors are
// logged rather than returned — every call site already treats Telegram
// delivery as best-effort, same as EmailService.SendEmailAsync. The
// goroutine runs with context.WithoutCancel(ctx): it keeps ctx's logger (for
// request correlation) but is detached from ctx's cancellation, since the
// originating HTTP handler — and huma's request context along with it — has
// typically already returned by the time this goroutine gets scheduled.
func (s *TelegramService) NotifyAsync(ctx context.Context, chatID int64, text string) {
	detached := context.WithoutCancel(ctx)
	go func() {
		if err := s.Notify(detached, chatID, text); err != nil {
			zerolog.Ctx(detached).Error().Err(err).Int64("chat_id", chatID).Msg("async telegram send failed")
		}
	}()
}
