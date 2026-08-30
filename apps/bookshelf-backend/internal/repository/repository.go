// Package repository defines the data-access interfaces for the bookshelf app.
// Handlers and services depend on these interfaces; only the implementations
// in repository/gorm/ (and db/db.go) import gorm.io/gorm.
package repository

import (
	"context"
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
	// CreateAdminIfNoneExists atomically checks whether an admin already
	// exists and, if not, creates user with role "admin". Returns
	// ErrConflict if an admin already exists (closes the TOCTOU window
	// between check and insert).
	CreateAdminIfNoneExists(user *models.User) error
	// ListDigestRecipients returns verified, non-suspended, non-pending-approval,
	// opted-in members — the target audience for the monthly digest email.
	ListDigestRecipients(ctx context.Context) ([]models.User, error)
}

// RegistrationVerificationRepository handles persistence for the short-lived
// pre-registration OTP codes proving control of an email or phone number
// before any User row exists.
type RegistrationVerificationRepository interface {
	// Upsert replaces any existing code for (channel, identifier) with a new
	// one — a resend supersedes rather than accumulates. pending carries the
	// not-yet-created account details for the email channel (zero-valued for
	// the phone channel), and is replaced wholesale alongside the code.
	Upsert(channel, identifier, code string, expiresAt time.Time, pending models.PendingRegistrationData) error
	Find(channel, identifier string) (*models.RegistrationVerification, error)
	Delete(channel, identifier string) error
	// DeleteExpired removes every row that expired before the given time,
	// returning how many were deleted. Rows are normally cleared the moment a
	// code is submitted (right or wrong), so this only sweeps up signups that
	// were started and then abandoned — which for the email channel means
	// rows still holding a bcrypt hash of a password the person very likely
	// uses elsewhere. Keeping those past the 15 minutes they're useful for is
	// retaining a credential for someone who never became a user.
	DeleteExpired(before time.Time) (int64, error)
}

// BookRepository handles persistence for Book records.
type BookRepository interface {
	FindByOLKey(olKey string) (*models.Book, error)
	FindByGoogleBooksID(id string) (*models.Book, error)
	// FindByISBN is a fallback lookup for createBook's upsert when neither a
	// stronger external key (OL key or Google Books ID) is present — see
	// BookHandler.findExistingBook.
	FindByISBN(isbn string) (*models.Book, error)
	List(search, sort string, availableOnly bool) ([]models.Book, error)
	// CountAll returns the total number of unique Book rows in the catalog
	// (distinct titles/editions, not Copy count) — used to give registration
	// emails a live "N books shared" figure.
	CountAll() (int64, error)
	ListPaginated(search, sort string, availableOnly bool, page, pageSize int) (*PaginatedResult[models.Book], error)
	ListRecent(limit int) ([]models.Book, error)
	// ListCreatedBetween returns books whose created_at is in [from, to),
	// ordered newest first, up to limit. Used by the monthly digest to
	// populate the "new this month" section.
	ListCreatedBetween(from, to time.Time, limit int) ([]models.Book, error)
	GetByIDWithCopies(id uint) (*models.Book, error)
	Create(book *models.Book) error
	Save(book *models.Book) error
	// Delete hard-deletes book — there is no soft-delete on Book (no
	// DeletedAt field). Used to clean up an orphaned keyless book once its
	// last Copy is removed — see CopyHandler.maybeDeleteOrphanedBook.
	Delete(book *models.Book) error
	CountAvailableCopies(bookID uint) (int64, error)
	// CountAvailableCopiesBatch returns a map of bookID → available copy count
	// for all requested book IDs in a single query.
	CountAvailableCopiesBatch(bookIDs []uint) (map[uint]int64, error)
	// CountCopies returns the total number of Copy rows for bookID, with no
	// status filter (unlike CountAvailableCopies) — used to detect when a
	// book has just gone copy-less.
	CountCopies(bookID uint) (int64, error)
	// CountBorrowsBatch returns a map of bookID → completed-loan count for
	// all requested book IDs in a single query. "Completed" means a
	// LoanRequest whose status reached "accepted" or "returned" — pending,
	// rejected, and cancelled requests don't count. Books with zero
	// completed loans are absent from the map (callers treat missing as 0),
	// same convention as CountAvailableCopiesBatch.
	CountBorrowsBatch(bookIDs []uint) (map[uint]int64, error)
	// CountWaitlistBatch returns a map of bookID → live waitlist depth
	// across every copy of the book, in a single query. Zero-waitlist books
	// are absent from the map, same convention as the batch counters above.
	CountWaitlistBatch(bookIDs []uint) (map[uint]int64, error)
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
	// ListByBorrowerIDPaginated returns a page of borrowerID's loan requests,
	// optionally filtered to the given statuses (empty/nil = no filter, i.e.
	// every status).
	ListByBorrowerIDPaginated(borrowerID uint, statuses []string, page, pageSize int) (*PaginatedResult[models.LoanRequest], error)
	// ListByOwnerIDPaginated returns a page of loan requests against copies
	// ownerID owns, optionally filtered to the given statuses (empty/nil = no
	// filter). Mirrors ListByBorrowerIDPaginated but joins through Copy since
	// owner_id lives there, not on LoanRequest.
	ListByOwnerIDPaginated(ownerID uint, statuses []string, page, pageSize int) (*PaginatedResult[models.LoanRequest], error)
	// ListActiveByBorrowerID returns borrowerID's currently-held loans
	// (status "accepted"), due-soonest first with no-due-date requests last.
	ListActiveByBorrowerID(borrowerID uint) ([]models.LoanRequest, error)
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

// AnnouncementRepository handles persistence for Announcement records.
type AnnouncementRepository interface {
	Create(a *models.Announcement) error
	GetByID(id uint) (*models.Announcement, error)
	Save(a *models.Announcement) error
	Delete(id uint) error
	// ListActive returns active announcements, newest first — used by the
	// public-facing endpoint regular users fetch once on load.
	ListActive() ([]models.Announcement, error)
	// ListPaginated returns every announcement (active and inactive), newest
	// first — used by the admin list view.
	ListPaginated(page, pageSize int) (*PaginatedResult[models.Announcement], error)
}

// UserListFilter narrows ListUsersPaginated's results. Zero-valued fields
// (empty string) apply no filter on that dimension — Status's four values
// mirror the badges the admin Users table already renders (verified,
// unverified, pending_approval, suspended); Search matches a case-insensitive
// substring of name OR email.
type UserListFilter struct {
	Search string
	Role   string
	Status string
}

// AdminRepository handles admin-level data access for user management and app settings.
type AdminRepository interface {
	ListUsers() ([]models.User, error)
	ListUsersPaginated(page, pageSize int, filter UserListFilter) (*PaginatedResult[models.User], error)
	ListByRole(role string) ([]models.User, error)
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
	TotalBooks           int64            `json:"total_books"`
	TotalCopies          int64            `json:"total_copies"`
	AvailableCopies      int64            `json:"available_copies"`
	LoanedCopies         int64            `json:"loaned_copies"`
	TotalUsers           int64            `json:"total_users"`
	SignupsThisWeek      int64            `json:"signups_this_week"`
	OverdueCount         int64            `json:"overdue_count"`
	PendingApprovalCount int64            `json:"pending_approval_count"`
	MostBorrowedBooks    []BookBorrowStat `json:"most_borrowed_books"`
	ActiveLenders        []LenderStat     `json:"active_lenders"`
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

// WishlistRequestRepository handles persistence for WishlistRequest records.
type WishlistRequestRepository interface {
	Create(r *models.WishlistRequest) error
	GetByID(id uint) (*models.WishlistRequest, error)
	Save(r *models.WishlistRequest) error
	// ListOpenPaginated returns open (status="open") requests, optionally
	// filtered by a title/author search, newest first — powers the browse board.
	ListOpenPaginated(search string, page, pageSize int) (*PaginatedResult[models.WishlistRequest], error)
	ListByRequesterID(requesterID uint) ([]models.WishlistRequest, error)
	// FindOpenByOLKey and FindOpenByGoogleBooksID power the auto-match hook in
	// createBook — they return every open request sharing the key, since
	// multiple members can separately be looking for the same book.
	FindOpenByOLKey(olKey string) ([]models.WishlistRequest, error)
	FindOpenByGoogleBooksID(googleBooksID string) ([]models.WishlistRequest, error)
	// FindOpenMatch returns the earliest open request matching any of the
	// given external keys (ISBN, OL key, Google Books ID), with its
	// Requester preloaded — powers the create-time dedup check so a member
	// can join an existing request instead of posting a duplicate. Returns
	// (nil, nil) if no key is given or none match.
	FindOpenMatch(isbn, olKey, googleBooksID string) (*models.WishlistRequest, error)
	// ClearFulfilledBookID nulls FulfilledBookID on every request pointing at
	// bookID — called before hard-deleting a Book so no row is left pointing
	// at a deleted one.
	ClearFulfilledBookID(bookID uint) error
}

// RecommendationRepository handles persistence for Recommendation records —
// a member's toggleable "highly recommend this" thumbs-up on a book. See
// docs/book-recommendations-spec.md.
type RecommendationRepository interface {
	// Create adds recommenderID's thumbs-up on bookID. Returns ErrConflict on
	// a duplicate (book_id, recommender_id) pair — treating that as an
	// idempotent success is the caller's job, not this method's.
	Create(bookID, recommenderID uint) error
	// Delete removes recommenderID's thumbs-up on bookID. Idempotent: no
	// error when the row doesn't exist.
	Delete(bookID, recommenderID uint) error
	FindByBookAndRecommender(bookID, recommenderID uint) (*models.Recommendation, error)
	// ListByBookID returns bookID's recommenders newest-first, with
	// Recommender preloaded.
	ListByBookID(bookID uint) ([]models.Recommendation, error)
	// CountByBookBatch returns bookID → recommendation count for all
	// requested book IDs in a single query. Books with zero recommendations
	// are absent from the map, same convention as CountAvailableCopiesBatch.
	CountByBookBatch(bookIDs []uint) (map[uint]int64, error)
	// HasRecommendedBatch returns bookID → whether userID has recommended
	// it, for all requested book IDs in a single query. Absent keys default
	// to false.
	HasRecommendedBatch(userID uint, bookIDs []uint) (map[uint]bool, error)
	// DeleteByBookID removes every recommendation for bookID — called before
	// hard-deleting an orphaned keyless Book (see
	// CopyHandler.maybeDeleteOrphanedBook).
	DeleteByBookID(bookID uint) error
	// DeleteByRecommenderID removes every recommendation made by
	// recommenderID — called before deleting a User, so an ex-member's
	// thumbs-ups fall out of every book's count and facepile (see
	// docs/book-recommendations-spec.md's "Live-community signal").
	DeleteByRecommenderID(recommenderID uint) error
	// ListTopBooks returns the top-recommended books ordered by recommendation
	// count descending, title ascending for ties, excluding books with zero
	// recommendations. Up to limit books are returned.
	ListTopBooks(limit int) ([]TopRecommendedBook, error)
}

// TopRecommendedBook pairs a book with its community recommendation count,
// used to build the "top picks" section of the monthly digest.
type TopRecommendedBook struct {
	Book  models.Book
	Count int64
}

// InviteCodeRepository handles persistence for InviteCode records — a
// member's permanent, multi-use invite link. See docs/invite-code-spec.md.
type InviteCodeRepository interface {
	// FindByInviter returns inviterID's existing code, or ErrNotFound if they
	// don't have one yet. Lets a caller distinguish "already has a code"
	// from "about to create one" — needed by GET /invite-code's creation
	// gate (see docs/invite-code-spec.md's allow_invite_codes semantics).
	FindByInviter(inviterID uint) (*models.InviteCode, error)
	// FindOrCreateByInviter returns inviterID's existing code, or inserts one
	// using code if none exists yet. code is only used on the create path —
	// the caller generates it before calling in, so a collision can be
	// retried without this method knowing about generation at all.
	FindOrCreateByInviter(inviterID uint, code string) (*models.InviteCode, error)
	FindByCode(code string) (*models.InviteCode, error)
	// Regenerate deletes inviterID's existing code (if any) and creates a new
	// one with newCode, atomically — there is never a window where neither
	// code exists.
	Regenerate(inviterID uint, newCode string) (*models.InviteCode, error)
	// DeleteByInviter removes inviterID's code. A no-op (nil error) if the
	// member has none.
	DeleteByInviter(inviterID uint) error
	// DeleteByID removes a code by primary key. Returns ErrNotFound if no
	// such row exists.
	DeleteByID(id uint) error
	// ListAll returns every invite code with its Inviter preloaded, newest
	// first. Not paginated — bounded by community size.
	ListAll() ([]models.InviteCode, error)
}
