// Package repository defines the data-access interfaces for the bookshelf app.
// Handlers and services depend on these interfaces; only the implementations
// in repository/gorm/ (and db/db.go) import gorm.io/gorm.
package repository

import (
	"errors"
	"time"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
)

// ErrNotFound is returned by repository methods when a record does not exist.
var ErrNotFound = errors.New("record not found")

// ErrConflict is returned when a unique constraint would be violated (e.g. duplicate waitlist entry).
var ErrConflict = errors.New("conflict")

// PaginatedResult holds a page of items plus total count metadata.
type PaginatedResult[T any] struct {
	Items      []T   `json:"items"`
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalPages int   `json:"total_pages"`
}

// UserRepository handles persistence for User records.
type UserRepository interface {
	Create(user *models.User) error
	FindByEmail(email string) (*models.User, error)
	FindByID(id uint) (*models.User, error)
	Save(user *models.User) error
	HasAdmin() (bool, error)
}

// RegistrationVerificationRepository handles persistence for the short-lived
// pre-registration OTP codes proving control of an email or phone number
// before any User row exists.
type RegistrationVerificationRepository interface {
	// Upsert replaces any existing code for (channel, identifier) with a new
	// one — a resend supersedes rather than accumulates.
	Upsert(channel, identifier, code string, expiresAt time.Time) error
	Find(channel, identifier string) (*models.RegistrationVerification, error)
	Delete(channel, identifier string) error
}

// BookRepository handles persistence for Book records.
type BookRepository interface {
	FindByOLKey(olKey string) (*models.Book, error)
	FindByGoogleBooksID(id string) (*models.Book, error)
	List(search, sort string, availableOnly bool) ([]models.Book, error)
	ListPaginated(search, sort string, availableOnly bool, page, pageSize int) (*PaginatedResult[models.Book], error)
	ListRecent(limit int) ([]models.Book, error)
	GetByIDWithCopies(id uint) (*models.Book, error)
	Create(book *models.Book) error
	Save(book *models.Book) error
	CountAvailableCopies(bookID uint) (int64, error)
	// CountAvailableCopiesBatch returns a map of bookID → available copy count
	// for all requested book IDs in a single query.
	CountAvailableCopiesBatch(bookIDs []uint) (map[uint]int64, error)
}

// CopyRepository handles persistence for Copy records.
type CopyRepository interface {
	Create(bookCopy *models.Copy) error
	GetByID(id uint) (*models.Copy, error)
	GetByIDWithAssociations(id uint) (*models.Copy, error)
	GetByIDWithOwner(id uint) (*models.Copy, error)
	ListByOwnerID(ownerID uint) ([]models.Copy, error)
	// ListOwnedBookIDs returns the distinct book IDs the owner has at least
	// one copy of, without loading full Copy/Book rows — for callers that
	// only need membership (e.g. a "yours" badge), not full records.
	ListOwnedBookIDs(ownerID uint) ([]uint, error)
	CountByOwnerID(ownerID uint) (int64, error)
	Save(bookCopy *models.Copy) error
	Delete(bookCopy *models.Copy) error
	UpdateStatus(id uint, status string) error
}

// LoanRequestRepository handles persistence for LoanRequest records.
type LoanRequestRepository interface {
	Create(lr *models.LoanRequest) error
	// CreateAndMarkRequested atomically creates the loan request and sets the
	// copy status to "requested". Returns ErrConflict if the copy is no longer
	// available (closes the TOCTOU window between check and insert).
	CreateAndMarkRequested(lr *models.LoanRequest) error
	GetByID(id uint) (*models.LoanRequest, error)
	GetByIDWithCopyAndBorrower(id uint) (*models.LoanRequest, error)
	GetByIDWithFullAssociations(id uint) (*models.LoanRequest, error)
	GetByIDWithCopyOwnerAndBorrower(id uint) (*models.LoanRequest, error)
	ListByCopyID(copyID uint) ([]models.LoanRequest, error)
	ListByBorrowerID(borrowerID uint) ([]models.LoanRequest, error)
	ListByBorrowerIDPaginated(borrowerID uint, page, pageSize int) (*PaginatedResult[models.LoanRequest], error)
	Save(lr *models.LoanRequest) error
	// RejectCompetingAndUpdateCopy atomically rejects all other pending requests
	// for copyID, creates rejection notifications for their borrowers, and sets
	// the copy status to "loaned".
	RejectCompetingAndUpdateCopy(copyID, acceptedLoanID uint) error
	CountPendingForCopyExcluding(copyID, excludeID uint) (int64, error)
	CountActiveLoansByBorrower(borrowerID uint) (int64, error)
}

// NotificationRepository handles persistence for Notification records.
type NotificationRepository interface {
	Create(n *models.Notification) error
	FindByRecipient(recipientID uint, unreadOnly bool) ([]models.Notification, error)
	FindByRecipientPaginated(recipientID uint, unreadOnly bool, page, pageSize int) (*PaginatedResult[models.Notification], error)
	GetByID(id uint) (*models.Notification, error)
	Save(n *models.Notification) error
	MarkAllReadForRecipient(recipientID uint) error
}

// AdminRepository handles admin-level data access for user management and app settings.
type AdminRepository interface {
	ListUsers() ([]models.User, error)
	ListUsersPaginated(page, pageSize int) (*PaginatedResult[models.User], error)
	FindUserByID(id uint) (*models.User, error)
	SaveUser(user *models.User) error
	DeleteUser(id uint) error
	GetSettings() ([]models.AppSetting, error)
	GetSetting(key string) (string, error)
	UpsertSetting(key, value string) error
	CountByRole(role string) (int64, error)
	GetDashboardStats() (*DashboardStats, error)
}

// BookBorrowStat pairs a book with how many times it has been loaned out
// (loan requests that reached "accepted" or "returned").
type BookBorrowStat struct {
	BookID      uint   `json:"book_id"`
	Title       string `json:"title"`
	Author      string `json:"author"`
	BorrowCount int64  `json:"borrow_count"`
}

// LenderStat pairs a user with how many copies they currently have out on loan.
type LenderStat struct {
	UserID      uint   `json:"user_id"`
	Name        string `json:"name"`
	ActiveLoans int64  `json:"active_loans"`
}

// DashboardStats aggregates the counts and rankings shown on the admin dashboard.
type DashboardStats struct {
	TotalBooks        int64            `json:"total_books"`
	TotalCopies       int64            `json:"total_copies"`
	AvailableCopies   int64            `json:"available_copies"`
	LoanedCopies      int64            `json:"loaned_copies"`
	TotalUsers        int64            `json:"total_users"`
	SignupsThisWeek   int64            `json:"signups_this_week"`
	OverdueCount      int64            `json:"overdue_count"`
	MostBorrowedBooks []BookBorrowStat `json:"most_borrowed_books"`
	ActiveLenders     []LenderStat     `json:"active_lenders"`
}

// WaitlistRepository handles persistence for WaitlistEntry records.
type WaitlistRepository interface {
	Add(copyID, userID uint) error
	Remove(copyID, userID uint) error
	ListByCopyID(copyID uint) ([]models.WaitlistEntry, error)
	Count(copyID uint) (int64, error)
	IsOnWaitlist(copyID, userID uint) (bool, error)
	DeleteByCopyID(copyID uint) error
}
