package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"net/http"
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

// telegramLinkTokenPayloadLen is the fixed-size signed payload: an 8-byte
// big-endian user ID plus a 4-byte big-endian Unix expiry (seconds — good
// until year 2106, plenty for a 10-minute token).
const telegramLinkTokenPayloadLen = 8 + 4

// telegramLinkTokenTagLen truncates the HMAC-SHA256 tag to 128 bits — ample
// for a token that's only ever valid for 10 minutes, and keeps the whole
// encoded token well under Telegram's 64-character deep-link limit.
const telegramLinkTokenTagLen = 16

// TelegramHandler handles linking/unlinking a member's Telegram account
// (apps/bookshelf-bot's counterpart lives in that app, not here). See
// docs/telegram-bot-integration-spec.md.
//
// Link tokens are a stateless HMAC-signed payload (user ID + expiry + tag),
// not a JWT like otp_link_token.go/unsubscribe.go use — a compact JWT's
// `header.payload.signature` shape uses `.` separators, which fall outside
// the alphabet Telegram's deep-link `start` parameter allows
// ([A-Za-z0-9_-], max 64 characters); the bot would receive no payload at
// all and /start would silently have nothing to link. Verification is a
// signature check plus an expiry comparison, no server-side state: no table,
// no cleanup job, and a token survives a backend restart or works correctly
// behind more than one replica for the same reason otp_link_token.go's JWTs
// do. This does mean a token isn't strictly single-use — replaying it
// within its 10-minute window just re-confirms the same (idempotent) link,
// the same tradeoff unsubscribeClaims already accepts for its long-lived
// unsubscribe link.
type TelegramHandler struct {
	users          repository.UserRepository
	telegram       services.TelegramNotifier
	jwtSecret      string
	internalSecret string
	botUsername    string

	// now is overridden in tests to exercise expiry without a real sleep.
	now func() time.Time
}

// NewTelegramHandler creates a new TelegramHandler.
func NewTelegramHandler(
	users repository.UserRepository,
	telegram services.TelegramNotifier,
	jwtSecret, internalSecret, botUsername string,
) *TelegramHandler {
	return &TelegramHandler{
		users:          users,
		telegram:       telegram,
		jwtSecret:      jwtSecret,
		internalSecret: internalSecret,
		botUsername:    botUsername,
		now:            time.Now,
	}
}

// issueLinkToken mints a signed, self-contained token for userID: an 8-byte
// user ID and a 4-byte expiry, HMAC-tagged and base64url-encoded
// (RawURLEncoding, no padding — Telegram's deep-link alphabet has no room
// for `=` padding either).
func (h *TelegramHandler) issueLinkToken(userID uint) string {
	payload := make([]byte, telegramLinkTokenPayloadLen)
	binary.BigEndian.PutUint64(payload[:8], uint64(userID))
	binary.BigEndian.PutUint32(payload[8:], uint32(h.now().Add(telegramLinkTokenTTL).Unix())) //nolint:gosec // fits until year 2106

	tag := h.linkTokenTag(payload)
	return base64.RawURLEncoding.EncodeToString(append(payload, tag...))
}

// verifyLinkToken decodes tokenStr, checks its HMAC tag in constant time,
// and rejects it if the embedded expiry has passed.
func (h *TelegramHandler) verifyLinkToken(tokenStr string) (uint, error) {
	invalid := errors.New("invalid or expired link")

	raw, err := base64.RawURLEncoding.DecodeString(tokenStr)
	if err != nil || len(raw) != telegramLinkTokenPayloadLen+telegramLinkTokenTagLen {
		return 0, invalid
	}
	payload, tag := raw[:telegramLinkTokenPayloadLen], raw[telegramLinkTokenPayloadLen:]
	if !hmac.Equal(tag, h.linkTokenTag(payload)) {
		return 0, invalid
	}

	userID := uint(binary.BigEndian.Uint64(payload[:8]))
	expiresAt := time.Unix(int64(binary.BigEndian.Uint32(payload[8:])), 0)
	if h.now().After(expiresAt) {
		return 0, invalid
	}
	return userID, nil
}

// linkTokenTag computes the truncated HMAC-SHA256 tag authenticating payload.
func (h *TelegramHandler) linkTokenTag(payload []byte) []byte {
	mac := hmac.New(sha256.New, []byte(h.jwtSecret))
	mac.Write(payload) //nolint:errcheck // hash.Hash.Write never returns an error
	return mac.Sum(nil)[:telegramLinkTokenTagLen]
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

	out := &linkTokenOutput{}
	out.Body.Token = h.issueLinkToken(userID)
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
		if errors.Is(err, repository.ErrConflict) {
			return nil, huma.Error409Conflict("this Telegram chat is already linked to another account")
		}
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
