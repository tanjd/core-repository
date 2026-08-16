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
	Suspended         bool       `gorm:"default:false" json:"suspended"`
	Role              string     `gorm:"default:'user';not null" json:"role"`
	CreatedAt         time.Time  `json:"created_at"`
	OTPCode           string     `gorm:"column:otp_code" json:"-"`
	OTPExpiry         *time.Time `gorm:"column:otp_expiry" json:"-"`
	GoogleBooksAPIKey string     `gorm:"column:google_books_api_key" json:"-"`
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

// Notification is an in-app alert delivered to a user.
// Type values: request_received | request_accepted | request_rejected |
//
//	marked_loaned | marked_returned
type Notification struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	RecipientID   uint      `gorm:"not null" json:"recipient_id"`
	Type          string    `json:"type"`
	LoanRequestID *uint     `json:"loan_request_id"`
	Read          bool      `gorm:"default:false" json:"read"`
	CreatedAt     time.Time `json:"created_at"`
}
