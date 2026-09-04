package handlers

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/middleware"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/services"
)

// telegramLinkTokenTTL is how long a "Connect Telegram" deep-link token
// stays valid — short, since the member is expected to tap it within the
// same session that minted it (see docs/telegram-bot-integration-spec.md's
// "Linking flow" section), unlike the 1-year digest unsubscribe link.
const telegramLinkTokenTTL = 10 * time.Minute

// telegramLinkTokenBytes is the random token's entropy, before base64url
// encoding. 16 bytes (128 bits) is far more than enough for a 10-minute,
// single-use token.
const telegramLinkTokenBytes = 16

// telegramLinkTokenEntry is what a link token resolves to while it's live.
type telegramLinkTokenEntry struct {
	userID    uint
	expiresAt time.Time
}

// TelegramHandler handles linking/unlinking a member's Telegram account
// (apps/bookshelf-bot's counterpart lives in that app, not here). See
// docs/telegram-bot-integration-spec.md.
//
// Link tokens are opaque random strings held in an in-memory map, not a
// signed JWT like every other magic-link flow in this app (see
// otp_link_token.go/unsubscribe.go). Telegram's deep-link `start` parameter
// only allows [A-Za-z0-9_-], max 64 characters — a compact JWT's `.`
// separators and typical length blow straight through that, so the bot
// receives no payload at all and silently can't complete the link. A random
// token base64url-encoded (RawURLEncoding, no padding) produces exactly
// Telegram's allowed alphabet at a fraction of the length. In-memory (not
// persisted) is an acceptable tradeoff for a 10-minute, single-use,
// single-instance-backend token: a mid-flow restart just means the member
// taps "Connect Telegram" again.
type TelegramHandler struct {
	users          repository.UserRepository
	telegram       services.TelegramNotifier
	internalSecret string
	botUsername    string

	mu         sync.Mutex
	linkTokens map[string]telegramLinkTokenEntry

	// now is overridden in tests to exercise expiry without a real sleep.
	now func() time.Time
}

// NewTelegramHandler creates a new TelegramHandler.
func NewTelegramHandler(users repository.UserRepository, telegram services.TelegramNotifier, internalSecret, botUsername string) *TelegramHandler {
	return &TelegramHandler{
		users:          users,
		telegram:       telegram,
		internalSecret: internalSecret,
		botUsername:    botUsername,
		linkTokens:     make(map[string]telegramLinkTokenEntry),
		now:            time.Now,
	}
}

// issueLinkToken mints a random opaque token for userID and stores it,
// pruning any other expired tokens while it's already holding the lock.
func (h *TelegramHandler) issueLinkToken(userID uint) (string, error) {
	buf := make([]byte, telegramLinkTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(buf)

	h.mu.Lock()
	defer h.mu.Unlock()
	now := h.now()
	for t, entry := range h.linkTokens {
		if now.After(entry.expiresAt) {
			delete(h.linkTokens, t)
		}
	}
	h.linkTokens[token] = telegramLinkTokenEntry{userID: userID, expiresAt: now.Add(telegramLinkTokenTTL)}
	return token, nil
}

// verifyLinkToken looks up tokenStr and, if found, consumes it (deletes it
// regardless of outcome — single-use, and there's no reason to keep an
// expired entry around either).
func (h *TelegramHandler) verifyLinkToken(tokenStr string) (uint, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	entry, ok := h.linkTokens[tokenStr]
	if ok {
		delete(h.linkTokens, tokenStr)
	}
	if !ok || h.now().After(entry.expiresAt) {
		return 0, errors.New("invalid or expired link")
	}
	return entry.userID, nil
}

// --- Input / Output types ---

type linkTokenOutput struct {
	Body struct {
		Token       string `json:"token" doc:"Short-lived token to append to the bot deep link."`
		BotUsername string `json:"bot_username" doc:"Telegram bot username (no leading @), for building https://t.me/<bot_username>?start=<token>. Empty if TELEGRAM_BOT_USERNAME isn't configured."`
	}
}

type confirmLinkInput struct {
	Body struct {
		Token  string `json:"token" doc:"Token minted by POST /profile/telegram/link-token, received via the bot's /start command."`
		ChatID int64  `json:"chat_id" doc:"The Telegram chat ID to link."`
	}
}

type confirmLinkOutput struct {
	Body struct {
		Name string `json:"name" doc:"The linked member's display name, for the bot's confirmation reply."`
	}
}

// --- Handlers ---

// createLinkToken mints a short-lived token for the authenticated member,
// for the frontend to embed in a t.me deep link.
func (h *TelegramHandler) createLinkToken(ctx context.Context, _ *struct{}) (*linkTokenOutput, error) {
	userID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	token, err := h.issueLinkToken(userID)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not mint link token")
	}

	out := &linkTokenOutput{}
	out.Body.Token = token
	out.Body.BotUsername = h.botUsername
	return out, nil
}

// confirmLink is called by apps/bookshelf-bot (not the frontend) once a
// member sends /start <token>. Authenticated by a static shared secret
// rather than a user JWT — the bot has no user session of its own.
func (h *TelegramHandler) confirmLink(_ context.Context, input *confirmLinkInput) (*confirmLinkOutput, error) {
	userID, err := h.verifyLinkToken(input.Body.Token)
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}

	user, err := h.users.FindByID(userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, huma.Error404NotFound("member not found")
		}
		return nil, huma.Error500InternalServerError("could not load member")
	}

	chatID := input.Body.ChatID
	now := time.Now()
	user.TelegramChatID = &chatID
	user.TelegramLinkedAt = &now
	user.TelegramNotificationsEnabled = true
	if err := h.users.Save(user); err != nil {
		return nil, huma.Error500InternalServerError("could not save link")
	}

	out := &confirmLinkOutput{}
	out.Body.Name = user.Name
	return out, nil
}

// unlink clears the authenticated member's Telegram link, resetting
// TelegramNotificationsEnabled to its default (true) so a future re-link
// starts clean rather than carrying forward a stale preference.
func (h *TelegramHandler) unlink(ctx context.Context, _ *struct{}) (*struct{}, error) {
	userID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	user, err := h.users.FindByID(userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, huma.Error404NotFound("member not found")
		}
		return nil, huma.Error500InternalServerError("could not load member")
	}

	user.TelegramChatID = nil
	user.TelegramLinkedAt = nil
	user.TelegramNotificationsEnabled = true
	if err := h.users.Save(user); err != nil {
		return nil, huma.Error500InternalServerError("could not save unlink")
	}
	return nil, nil
}

// sendTestNotification pushes a one-off message to the authenticated
// member's linked Telegram chat, synchronously (unlike every event
// notification, which is fire-and-forget) so this "does it actually work"
// check can report success or failure back to the profile settings page
// that triggered it, rather than silently doing nothing on failure.
func (h *TelegramHandler) sendTestNotification(ctx context.Context, _ *struct{}) (*struct{}, error) {
	userID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	user, err := h.users.FindByID(userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, huma.Error404NotFound("member not found")
		}
		return nil, huma.Error500InternalServerError("could not load member")
	}

	if user.TelegramChatID == nil {
		return nil, huma.Error400BadRequest("link Telegram before sending a test notification")
	}

	const testMessage = "✅ Test notification from bookshelf — if you're seeing this, your Telegram connection is working."
	if err := h.telegram.Notify(ctx, *user.TelegramChatID, testMessage); err != nil {
		return nil, huma.Error502BadGateway("could not reach Telegram — check the bot is still linked and try again")
	}
	return nil, nil
}

// requireInternalSecret guards confirmLink against callers other than
// apps/bookshelf-bot. Constant-time comparison, same as the pending-email
// OTP check in auth_profile.go.
func (h *TelegramHandler) requireInternalSecret(secret string) error {
	if h.internalSecret == "" || subtle.ConstantTimeCompare([]byte(secret), []byte(h.internalSecret)) != 1 {
		return errors.New("invalid internal secret")
	}
	return nil
}

// --- Route registration ---

// RegisterRoutes registers the Telegram linking endpoints on the given huma API.
func (h *TelegramHandler) RegisterRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "create-telegram-link-token",
		Method:      "POST",
		Path:        "/profile/telegram/link-token",
		Summary:     "Mint a Telegram link token",
		Description: "Mints a short-lived token for the frontend to embed in a t.me deep link.",
		Tags:        []string{"profile"},
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.createLinkToken)

	huma.Register(api, huma.Operation{
		OperationID: "confirm-telegram-link",
		Method:      "POST",
		Path:        "/internal/telegram/confirm-link",
		Summary:     "Confirm a Telegram link (bot-only)",
		Description: "Called by apps/bookshelf-bot after a member sends /start <token>. Requires the X-Internal-Secret header, not a user session.",
		Tags:        []string{"internal"},
		Middlewares: huma.Middlewares{h.internalSecretMiddleware(api)},
	}, h.confirmLink)

	huma.Register(api, huma.Operation{
		OperationID: "unlink-telegram",
		Method:      "DELETE",
		Path:        "/profile/telegram/link",
		Summary:     "Unlink Telegram",
		Description: "Clears the authenticated member's Telegram link.",
		Tags:        []string{"profile"},
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.unlink)

	huma.Register(api, huma.Operation{
		OperationID: "send-telegram-test-notification",
		Method:      "POST",
		Path:        "/profile/telegram/test",
		Summary:     "Send a test Telegram notification",
		Description: "Pushes a one-off message to the authenticated member's linked Telegram chat, to confirm the connection actually works.",
		Tags:        []string{"profile"},
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.sendTestNotification)
}

// internalSecretMiddleware returns huma operation middleware that rejects
// any request to confirm-link that doesn't carry a matching
// X-Internal-Secret header, before the handler runs — same shape as
// middleware.RateLimit.
func (h *TelegramHandler) internalSecretMiddleware(api huma.API) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		if err := h.requireInternalSecret(ctx.Header("X-Internal-Secret")); err != nil {
			_ = huma.WriteErr(api, ctx, http.StatusUnauthorized, "invalid internal secret", err)
			return
		}
		next(ctx)
	}
}
