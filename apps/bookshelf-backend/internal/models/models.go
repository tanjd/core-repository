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
}

// RegistrationVerification holds a short-lived OTP code proving control of an
// email address or phone number before a User row exists to attach it to
// (unlike User.OTPCode, which verifies an already-registered account). One
// row per (channel, identifier) pair — a resend overwrites the existing row
// rather than accumulating history.
type RegistrationVerification struct {
	ID         uint      `gorm:"primarykey" json:"-"`
	Channel    string    `gorm:"not null;uniqueIndex:idx_registration_verifications_channel_identifier" json:"-"`
	Identifier string    `gorm:"not null;uniqueIndex:idx_registration_verifications_channel_identifier" json:"-"`
	Code       string    `gorm:"not null" json:"-"`
	ExpiresAt  time.Time `gorm:"column:expires_at;not null" json:"-"`
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
	Copies        []Copy    `json:"copies,omitempty"`
}

// Copy is a physical instance of a Book owned by a church member.
// Status values: available | requested | loaned | unavailable
type Copy struct {
	ID                 uint   `gorm:"primarykey" json:"id"`
	BookID             uint   `gorm:"not null" json:"book_id"`
	OwnerID            uint   `gorm:"not null" json:"owner_id"`
	Condition          string `json:"condition"` // good | fair | worn
	Notes              string `json:"notes"`
	Status             string `gorm:"default:'available'" json:"status"`
	AutoApprove        bool   `gorm:"default:false" json:"auto_approve"`
	ReturnDateRequired bool   `gorm:"default:false" json:"return_date_required"`
	HideOwner          bool   `gorm:"default:false" json:"hide_owner"`
	Book               Book   `json:"book,omitempty"`
	Owner              User   `json:"owner,omitempty"`
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
	ExpectedReturnDate *time.Time `json:"expected_return_date,omitempty"`
	Copy               Copy       `json:"copy,omitempty"`
	Borrower           User       `json:"borrower,omitempty"`
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
	FulfilledBookID *uint      `json:"fulfilled_book_id"`
	FulfilledAt     *time.Time `json:"fulfilled_at"`
	CreatedAt       time.Time  `json:"created_at"`
	Requester       User       `json:"requester,omitempty"`
	FulfilledBook   *Book      `json:"fulfilled_book,omitempty"`
}

// Notification is an in-app alert delivered to a user.
// Type values: request_received | request_accepted | request_rejected |
//
//	marked_loaned | marked_returned | waitlist_available |
//	copy_transferred_in | copy_transferred_out | wishlist_fulfilled
type Notification struct {
	ID                uint      `gorm:"primarykey" json:"id"`
	RecipientID       uint      `gorm:"not null" json:"recipient_id"`
	Type              string    `json:"type"`
	LoanRequestID     *uint     `json:"loan_request_id"`
	WishlistRequestID *uint     `json:"wishlist_request_id"`
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
