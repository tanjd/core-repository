// Package models defines the GORM database models for the bookshelf app.
package models

import "time"

// User represents a registered member of the church community.
type User struct {
	ID                uint       `gorm:"primarykey" json:"id"`
	Name              string     `gorm:"not null" json:"name"`
	Email             string     `gorm:"uniqueIndex;not null" json:"email"`
	Phone             string     `json:"phone"`
	Password          string     `gorm:"not null" json:"-"`
	Verified          bool       `gorm:"default:false" json:"verified"`
	PhoneVerified     bool       `gorm:"column:phone_verified;default:false" json:"phone_verified"`
	Suspended         bool       `gorm:"default:false" json:"suspended"`
	PendingApproval   bool       `gorm:"column:pending_approval;default:false" json:"pending_approval"`
	Role              string     `gorm:"default:'user';not null" json:"role"`
	CreatedAt         time.Time  `json:"created_at"`
	OTPCode           string     `gorm:"column:otp_code" json:"-"`
	OTPExpiry         *time.Time `gorm:"column:otp_expiry" json:"-"`
	GoogleBooksAPIKey string     `gorm:"column:google_books_api_key" json:"-"`

	PendingEmail          string     `gorm:"column:pending_email" json:"pending_email,omitempty"`
	PendingEmailOTPCode   string     `gorm:"column:pending_email_otp_code" json:"-"`
	PendingEmailOTPExpiry *time.Time `gorm:"column:pending_email_otp_expiry" json:"-"`

	ResetPasswordOTPCode   string     `gorm:"column:reset_password_otp_code" json:"-"`
	ResetPasswordOTPExpiry *time.Time `gorm:"column:reset_password_otp_expiry" json:"-"`

	// No GORM "default:true" tag here deliberately — same reasoning as
	// Announcement.Active below: GORM's Create() treats a zero-valued bool
	// field carrying a "default" tag as unset and substitutes the DB
	// default, silently turning an explicit false into true. register/setup
	// below explicitly set these true; the migration's SQL-level DEFAULT 1
	// is a safety net for direct inserts that bypass application code.
	EmailNotificationsEnabled bool   `gorm:"column:email_notifications_enabled;not null" json:"email_notifications_enabled"`
	MonthlyDigestEnabled      bool   `gorm:"column:monthly_digest_enabled;not null" json:"monthly_digest_enabled"`
	TelegramUsername          string `gorm:"column:telegram_username" json:"telegram_username,omitempty"`
	WhatsAppUsername          string `gorm:"column:whatsapp_username" json:"whatsapp_username,omitempty"`
	ContactNote               string `gorm:"column:contact_note" json:"contact_note,omitempty"`

	// TelegramChatID is the bot-linked Telegram chat used for push
	// notification delivery (see docs/telegram-bot-integration-spec.md) —
	// not to be confused with TelegramUsername above, a free-text field
	// shown to the other party for manually arranging pickup. Nil means
	// Telegram isn't linked. TelegramNotificationsEnabled has no GORM
	// "default" tag, for the same reason EmailNotificationsEnabled/
	// MonthlyDigestEnabled above don't: the link/unlink handlers explicitly
	// set it, and the migration's SQL-level DEFAULT is the safety net for
	// direct inserts that bypass application code.
	TelegramChatID               *int64     `gorm:"column:telegram_chat_id;uniqueIndex" json:"-"`
	TelegramLinkedAt             *time.Time `gorm:"column:telegram_linked_at" json:"telegram_linked_at,omitempty"`
	TelegramNotificationsEnabled bool       `gorm:"column:telegram_notifications_enabled;not null" json:"telegram_notifications_enabled"`

	// InvitedByID is a permanent record of which member's invite link this
	// account registered through — survives regeneration or revocation of
	// that link (see docs/invite-code-spec.md). Nil for accounts that
	// registered without an invite code.
	InvitedByID *uint `gorm:"column:invited_by_id" json:"invited_by_id,omitempty"`
	InvitedBy   *User `gorm:"foreignKey:InvitedByID" json:"invited_by,omitempty"`
}

// RegistrationVerification holds a short-lived OTP code proving control of an
// email address or phone number before a User row exists to attach it to
// (unlike User.OTPCode, which verifies an already-registered account). One
// row per (channel, identifier) pair — a resend overwrites the existing row
// rather than accumulating history.
//
// For the email channel it also parks the account details typed on the
// registration form's first step (PendingRegistrationData below), so
// verifying the code — from any device, including one that never saw the
// form — is enough to create the account outright.
type RegistrationVerification struct {
	ID         uint      `gorm:"primarykey" json:"-"`
	Channel    string    `gorm:"not null;uniqueIndex:idx_registration_verifications_channel_identifier" json:"-"`
	Identifier string    `gorm:"not null;uniqueIndex:idx_registration_verifications_channel_identifier" json:"-"`
	Code       string    `gorm:"not null" json:"-"`
	ExpiresAt  time.Time `gorm:"column:expires_at;not null" json:"-"`

	PendingRegistrationData `gorm:"embedded"`
}

// PendingRegistrationData is the not-yet-created account behind an
// in-flight email verification: everything /auth/register used to take in a
// second request, held server-side for the same 15 minutes as the code
// itself and discarded with it.
//
// Never serialized — PendingPasswordHash in particular must not leave the
// backend, and the whole point of storing these here rather than embedding
// them in the magic-link JWT is that they never transit email or a URL.
type PendingRegistrationData struct {
	PendingName string `gorm:"column:pending_name" json:"-"`
	// The email as the user typed it. The row's Identifier is normalized
	// (lowercased) so the magic-link token can key off it; the account keeps
	// the original casing purely so it displays back the way it was entered.
	// Lookups are case-insensitive either way (see migration 000011).
	PendingEmail        string `gorm:"column:pending_email" json:"-"`
	PendingPasswordHash string `gorm:"column:pending_password_hash" json:"-"`
	PendingPhone        string `gorm:"column:pending_phone" json:"-"`
	// PendingInviteCode carries a raw invite code across the 15-minute OTP
	// window so verify-email-otp can re-validate it before creating the
	// account — see docs/invite-code-spec.md. Empty for a registration
	// started without one.
	PendingInviteCode string `gorm:"column:pending_invite_code" json:"-"`
}

// AppSetting is a runtime-configurable key-value pair stored in the database.
type AppSetting struct {
	Key       string    `gorm:"primaryKey" json:"key"`
	Value     string    `gorm:"not null" json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Book is a title in the library catalogue.
type Book struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	Title         string    `gorm:"not null" json:"title"`
	Author        string    `gorm:"not null" json:"author"`
	ISBN          string    `json:"isbn"`
	OLKey         string    `json:"ol_key"`
	CoverURL      string    `json:"cover_url"`
	Description   string    `json:"description"`
	Publisher     string    `json:"publisher"`
	PublishedDate string    `json:"published_date"`
	PageCount     int       `json:"page_count"`
	Language      string    `json:"language"`
	GoogleBooksID string    `json:"google_books_id"`
	CreatedAt     time.Time `json:"created_at"`
	// DescriptionEnriched marks Description as backfilled from a sibling
	// edition of the same work by the description-reconciliation job, rather
	// than sourced for this book itself — see
	// internal/services/description_reconciliation.go. Never cleared
	// automatically if Description is later edited directly.
	DescriptionEnriched bool   `gorm:"not null;default:false" json:"description_enriched"`
	Copies              []Copy `json:"copies,omitempty"`
}

// Copy is a physical instance of a Book owned by a church member.
// Status values: available | requested | loaned | unavailable
type Copy struct {
	ID          uint   `gorm:"primarykey" json:"id"`
	BookID      uint   `gorm:"not null" json:"book_id"`
	OwnerID     uint   `gorm:"not null" json:"owner_id"`
	Condition   string `json:"condition"` // good | fair | worn
	Notes       string `json:"notes"`
	Status      string `gorm:"default:'available'" json:"status"`
	AutoApprove bool   `gorm:"default:false" json:"auto_approve"`
	HideOwner   bool   `gorm:"default:false" json:"hide_owner"`
	Book        Book   `json:"book,omitempty"`
	Owner       User   `json:"owner,omitempty"`
}

// LoanRequest tracks a borrower's request to borrow a specific Copy.
// Status values: pending | accepted | rejected | cancelled | returned
type LoanRequest struct {
	ID                 uint       `gorm:"primarykey" json:"id"`
	CopyID             uint       `gorm:"not null" json:"copy_id"`
	BorrowerID         uint       `gorm:"not null" json:"borrower_id"`
	Message            string     `json:"message"`
	Status             string     `gorm:"default:'pending'" json:"status"`
	RequestedAt        time.Time  `json:"requested_at"`
	RespondedAt        *time.Time `json:"responded_at"`
	LoanedAt           *time.Time `json:"loaned_at"`
	ReturnedAt         *time.Time `json:"returned_at"`
	ReturnedBy         *uint      `gorm:"column:returned_by" json:"returned_by,omitempty"`
	ExpectedReturnDate time.Time  `gorm:"not null" json:"expected_return_date"`
	// ExpectedReturnDateChangedBy/At are nil until either party amends the
	// return date after the loan is accepted (see the
	// updateExpectedReturnDate handler) — the original accept-time value is
	// never touched. Last-write-wins, no history kept.
	ExpectedReturnDateChangedBy *uint      `json:"expected_return_date_changed_by,omitempty"`
	ExpectedReturnDateChangedAt *time.Time `json:"expected_return_date_changed_at,omitempty"`
	// DueReminderSentAt records when the due-date-reminder scheduler job
	// (see docs/telegram-bot-integration-spec.md) pushed a reminder for this
	// loan, so a later run doesn't remind the borrower twice. Nil means no
	// reminder has been sent yet.
	DueReminderSentAt *time.Time `gorm:"column:due_reminder_sent_at" json:"due_reminder_sent_at,omitempty"`
	Copy              Copy       `json:"copy,omitempty"`
	Borrower          User       `json:"borrower,omitempty"`
}

// WaitlistEntry tracks users waiting for a loaned copy to become available.
type WaitlistEntry struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CopyID    uint      `gorm:"not null;uniqueIndex:idx_waitlist_copy_user" json:"copy_id"`
	UserID    uint      `gorm:"not null;uniqueIndex:idx_waitlist_copy_user" json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	User      User      `json:"user,omitempty"`
}

// WishlistRequest tracks a member's post for a book not currently in the
// catalog — "does anyone have X" for a title nobody's added yet. Book
// identity comes from the same metadata-search results used by /books
// (createBook), so it always carries a resolvable external key: this lets a
// newly-added Book auto-match and fulfill it. Status values: open |
// fulfilled | cancelled
type WishlistRequest struct {
	ID              uint       `gorm:"primarykey" json:"id"`
	RequesterID     uint       `gorm:"not null" json:"requester_id"`
	Title           string     `gorm:"not null" json:"title"`
	Author          string     `gorm:"not null" json:"author"`
	ISBN            string     `json:"isbn"`
	OLKey           string     `json:"ol_key"`
	GoogleBooksID   string     `json:"google_books_id"`
	CoverURL        string     `json:"cover_url"`
	Notes           string     `json:"notes"`
	Status          string     `gorm:"not null;default:'open'" json:"status"`
	IsAnonymous     bool       `gorm:"column:is_anonymous;not null;default:false" json:"is_anonymous"`
	FulfilledBookID *uint      `json:"fulfilled_book_id"`
	FulfilledAt     *time.Time `json:"fulfilled_at"`
	CreatedAt       time.Time  `json:"created_at"`
	Requester       User       `json:"requester,omitempty"`
	FulfilledBook   *Book      `json:"fulfilled_book,omitempty"`
}

// Recommendation is a member's lightweight "I'd highly recommend this"
// thumbs-up on a book — one per (book, recommender) pair, toggled on/off
// rather than edited. See docs/book-recommendations-spec.md.
type Recommendation struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	BookID        uint      `gorm:"not null;uniqueIndex:idx_recommendations_book_recommender" json:"book_id"`
	RecommenderID uint      `gorm:"not null;uniqueIndex:idx_recommendations_book_recommender" json:"recommender_id"`
	CreatedAt     time.Time `json:"created_at"`
	Recommender   User      `json:"recommender,omitempty"`
}

// InviteCode is a member's permanent, multi-use invite link. See
// docs/invite-code-spec.md. There is no ExpiresAt, UsedAt, or UsedByID —
// who was invited by whom is tracked on User.InvitedByID instead, since that
// survives regeneration or deletion of the code itself. The uniqueIndex on
// InviterID enforces one code per member at the database level.
type InviteCode struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	Code      string    `gorm:"uniqueIndex;not null" json:"code"`
	InviterID uint      `gorm:"uniqueIndex;not null" json:"inviter_id"`
	Inviter   User      `json:"inviter,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Notification is an in-app alert delivered to a user.
// Type values: request_received | request_accepted | request_rejected |
//
//	marked_loaned | marked_returned | return_undone | waitlist_available |
//	copy_transferred_in | copy_transferred_out | wishlist_fulfilled |
//	user_pending_approval | user_approved | expected_return_date_changed
type Notification struct {
	ID                uint      `gorm:"primarykey" json:"id"`
	RecipientID       uint      `gorm:"not null" json:"recipient_id"`
	Type              string    `json:"type"`
	LoanRequestID     *uint     `json:"loan_request_id"`
	WishlistRequestID *uint     `json:"wishlist_request_id"`
	PendingUserID     *uint     `json:"pending_user_id"`
	Read              bool      `gorm:"default:false" json:"read"`
	CreatedAt         time.Time `json:"created_at"`
}

// Announcement is a permanent, admin-authored banner shown to community
// members until an admin manually deactivates it — no scheduled start/end,
// unlike a per-release changelog (deliberately a separate, undesigned
// feature — see TODO.md).
// Type values: info | new_feature | known_issue
type Announcement struct {
	ID    uint   `gorm:"primarykey" json:"id"`
	Title string `gorm:"not null" json:"title"`
	Body  string `gorm:"not null" json:"body"`
	Type  string `gorm:"not null;default:'info'" json:"type"`
	// No GORM "default:true" tag here deliberately: GORM's Create() treats a
	// zero-valued field (false, for bool) with a "default" tag as "unset" and
	// substitutes the DB default instead — silently turning an explicit
	// Active: false into true. The handler layer already applies the
	// default-true behavior when the API caller omits the field; the
	// migration's SQL-level DEFAULT 1 remains as a safety net for direct inserts.
	Active    bool      `gorm:"not null" json:"active"`
	CreatedAt time.Time `json:"created_at"`
}
