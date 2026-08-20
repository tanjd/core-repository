// Package repotest provides in-memory fake implementations of the
// repository interfaces in internal/repository, for use in handler and
// service unit tests that don't need real SQL semantics. Save methods mimic
// GORM's upsert behavior (insert when the primary key is zero, update
// otherwise) so callers can seed fixtures the same way production code
// would create records.
package repotest

import (
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

// UserRepository is an in-memory fake of repository.UserRepository.
type UserRepository struct {
	mu     sync.Mutex
	nextID uint
	byID   map[uint]*models.User
}

// NewUserRepository creates an empty fake UserRepository.
func NewUserRepository() *UserRepository {
	return &UserRepository{byID: map[uint]*models.User{}}
}

// Create inserts user, assigning it a new ID and defaulting Role to "user".
// Returns repository.ErrConflict if the email is already taken.
func (r *UserRepository) Create(user *models.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, u := range r.byID {
		if u.Email == user.Email {
			return repository.ErrConflict
		}
	}
	r.nextID++
	user.ID = r.nextID
	if user.Role == "" {
		user.Role = "user"
	}
	cp := *user
	r.byID[user.ID] = &cp
	return nil
}

// FindByEmail returns the user with the given email, or repository.ErrNotFound.
func (r *UserRepository) FindByEmail(email string) (*models.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, u := range r.byID {
		if u.Email == email {
			cp := *u
			return &cp, nil
		}
	}
	return nil, repository.ErrNotFound
}

// FindByID returns the user with the given ID, or repository.ErrNotFound.
func (r *UserRepository) FindByID(id uint) (*models.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.byID[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	cp := *u
	return &cp, nil
}

// Save inserts user (assigning a new ID) if user.ID is zero, else overwrites
// the existing record — mirroring GORM's Save semantics.
func (r *UserRepository) Save(user *models.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if user.ID == 0 {
		r.nextID++
		user.ID = r.nextID
	}
	cp := *user
	r.byID[user.ID] = &cp
	return nil
}

// HasAdmin reports whether any stored user has the "admin" role.
func (r *UserRepository) HasAdmin() (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, u := range r.byID {
		if u.Role == "admin" {
			return true, nil
		}
	}
	return false, nil
}

// CreateAdminIfNoneExists inserts user (assigning a new ID, forcing Role to
// "admin") if no stored user currently has the "admin" role, holding the
// lock across the check and insert to mirror the real transaction's TOCTOU
// guard. Returns repository.ErrConflict if an admin already exists or if
// the email is already taken, matching the GORM impl's unique-email index.
func (r *UserRepository) CreateAdminIfNoneExists(user *models.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, u := range r.byID {
		if u.Role == "admin" {
			return repository.ErrConflict
		}
		if u.Email == user.Email {
			return repository.ErrConflict
		}
	}
	r.nextID++
	user.ID = r.nextID
	user.Role = "admin"
	cp := *user
	r.byID[user.ID] = &cp
	return nil
}

// RegistrationVerificationRepository is an in-memory fake of
// repository.RegistrationVerificationRepository.
type RegistrationVerificationRepository struct {
	mu    sync.Mutex
	byKey map[string]*models.RegistrationVerification
}

// NewRegistrationVerificationRepository creates an empty fake
// RegistrationVerificationRepository.
func NewRegistrationVerificationRepository() *RegistrationVerificationRepository {
	return &RegistrationVerificationRepository{byKey: map[string]*models.RegistrationVerification{}}
}

func regVerificationKey(channel, identifier string) string {
	return channel + ":" + identifier
}

// Upsert replaces any existing code for (channel, identifier).
func (r *RegistrationVerificationRepository) Upsert(channel, identifier, code string, expiresAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byKey[regVerificationKey(channel, identifier)] = &models.RegistrationVerification{
		Channel: channel, Identifier: identifier, Code: code, ExpiresAt: expiresAt,
	}
	return nil
}

// Find returns the stored verification for (channel, identifier), or repository.ErrNotFound.
func (r *RegistrationVerificationRepository) Find(channel, identifier string) (*models.RegistrationVerification, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.byKey[regVerificationKey(channel, identifier)]
	if !ok {
		return nil, repository.ErrNotFound
	}
	cp := *v
	return &cp, nil
}

// Delete removes the stored verification for (channel, identifier), if present.
func (r *RegistrationVerificationRepository) Delete(channel, identifier string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.byKey, regVerificationKey(channel, identifier))
	return nil
}

// AdminRepository is an in-memory fake of repository.AdminRepository.
type AdminRepository struct {
	mu       sync.Mutex
	users    map[uint]*models.User
	settings map[string]string
	stats    *repository.DashboardStats
}

// NewAdminRepository creates an empty fake AdminRepository.
func NewAdminRepository() *AdminRepository {
	return &AdminRepository{users: map[uint]*models.User{}, settings: map[string]string{}}
}

// SetDashboardStats overrides the value GetDashboardStats returns; useful
// for tests that only exercise the handler's admin-gating logic.
func (r *AdminRepository) SetDashboardStats(s *repository.DashboardStats) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stats = s
}

// ListUsers returns all stored users ordered by ID.
func (r *AdminRepository) ListUsers() ([]models.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]models.User, 0, len(r.users))
	for _, u := range r.users {
		out = append(out, *u)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// ListUsersPaginated returns a page of users ordered by ID.
func (r *AdminRepository) ListUsersPaginated(page, pageSize int) (*repository.PaginatedResult[models.User], error) {
	all, _ := r.ListUsers()
	start, end := paginationBounds(len(all), page, pageSize)
	items := append([]models.User{}, all[start:end]...)
	return &repository.PaginatedResult[models.User]{
		Items: items, Total: int64(len(all)), Page: page, PageSize: pageSize,
		TotalPages: totalPages(len(all), pageSize),
	}, nil
}

// ListByRole returns all stored users with the given role, ordered by ID.
func (r *AdminRepository) ListByRole(role string) ([]models.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]models.User, 0)
	for _, u := range r.users {
		if u.Role == role {
			out = append(out, *u)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// FindUserByID returns the user with the given ID, or repository.ErrNotFound.
func (r *AdminRepository) FindUserByID(id uint) (*models.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.users[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	cp := *u
	return &cp, nil
}

// SaveUser upserts user by ID — also the way tests seed fixtures.
func (r *AdminRepository) SaveUser(user *models.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *user
	r.users[user.ID] = &cp
	return nil
}

// DeleteUser removes the user with the given ID, if present.
func (r *AdminRepository) DeleteUser(id uint) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.users, id)
	return nil
}

// GetSettings returns all stored settings ordered by key.
func (r *AdminRepository) GetSettings() ([]models.AppSetting, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]models.AppSetting, 0, len(r.settings))
	for k, v := range r.settings {
		out = append(out, models.AppSetting{Key: k, Value: v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

// GetSetting returns the value for key, or repository.ErrNotFound if unset.
func (r *AdminRepository) GetSetting(key string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.settings[key]
	if !ok {
		return "", repository.ErrNotFound
	}
	return v, nil
}

// UpsertSetting sets key to value, also usable by tests to seed settings.
func (r *AdminRepository) UpsertSetting(key, value string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.settings[key] = value
	return nil
}

// CountByRole counts stored users with the given role.
func (r *AdminRepository) CountByRole(role string) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var count int64
	for _, u := range r.users {
		if u.Role == role {
			count++
		}
	}
	return count, nil
}

// GetDashboardStats returns the value set via SetDashboardStats, or an empty
// (but non-nil-slice) DashboardStats if none was set.
func (r *AdminRepository) GetDashboardStats() (*repository.DashboardStats, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stats != nil {
		return r.stats, nil
	}
	return &repository.DashboardStats{
		MostBorrowedBooks: []repository.BookBorrowStat{},
		ActiveLenders:     []repository.LenderStat{},
	}, nil
}

// CopyRepository is an in-memory fake of repository.CopyRepository.
type CopyRepository struct {
	mu     sync.Mutex
	nextID uint
	byID   map[uint]*models.Copy
}

// NewCopyRepository creates an empty fake CopyRepository.
func NewCopyRepository() *CopyRepository {
	return &CopyRepository{byID: map[uint]*models.Copy{}}
}

// Create inserts bookCopy, assigning it a new ID and defaulting Status to "available".
func (r *CopyRepository) Create(bookCopy *models.Copy) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID++
	bookCopy.ID = r.nextID
	if bookCopy.Status == "" {
		bookCopy.Status = "available"
	}
	cp := *bookCopy
	r.byID[bookCopy.ID] = &cp
	return nil
}

// GetByID returns the copy with the given ID, or repository.ErrNotFound.
func (r *CopyRepository) GetByID(id uint) (*models.Copy, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.byID[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	cp := *c
	return &cp, nil
}

// GetByIDWithAssociations behaves like GetByID — the fake stores Book/Owner
// as plain struct fields rather than simulating GORM preloading, so callers
// must populate them on the models.Copy passed to Create.
func (r *CopyRepository) GetByIDWithAssociations(id uint) (*models.Copy, error) {
	return r.GetByID(id)
}

// GetByIDWithOwner behaves like GetByID; see GetByIDWithAssociations.
func (r *CopyRepository) GetByIDWithOwner(id uint) (*models.Copy, error) {
	return r.GetByID(id)
}

// ListByOwnerID returns all copies owned by ownerID, ordered by ID.
func (r *CopyRepository) ListByOwnerID(ownerID uint) ([]models.Copy, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []models.Copy{}
	for _, c := range r.byID {
		if c.OwnerID == ownerID {
			out = append(out, *c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// ListOwnedBookIDs returns the distinct book IDs ownerID has at least one copy of.
func (r *CopyRepository) ListOwnedBookIDs(ownerID uint) ([]uint, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	seen := map[uint]bool{}
	out := []uint{}
	for _, c := range r.byID {
		if c.OwnerID == ownerID && !seen[c.BookID] {
			seen[c.BookID] = true
			out = append(out, c.BookID)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

// CountByOwnerID counts copies owned by ownerID.
func (r *CopyRepository) CountByOwnerID(ownerID uint) (int64, error) {
	items, _ := r.ListByOwnerID(ownerID)
	return int64(len(items)), nil
}

// Save inserts bookCopy (assigning a new ID) if its ID is zero, else
// overwrites the existing record.
func (r *CopyRepository) Save(bookCopy *models.Copy) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if bookCopy.ID == 0 {
		r.nextID++
		bookCopy.ID = r.nextID
	}
	cp := *bookCopy
	r.byID[bookCopy.ID] = &cp
	return nil
}

// Delete removes bookCopy from the store.
func (r *CopyRepository) Delete(bookCopy *models.Copy) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.byID, bookCopy.ID)
	return nil
}

// UpdateStatus sets the status of the copy with the given ID.
func (r *CopyRepository) UpdateStatus(id uint, status string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.byID[id]
	if !ok {
		return repository.ErrNotFound
	}
	c.Status = status
	return nil
}

// NotificationRepository is an in-memory fake of repository.NotificationRepository.
type NotificationRepository struct {
	mu     sync.Mutex
	nextID uint
	byID   map[uint]*models.Notification
}

// NewNotificationRepository creates an empty fake NotificationRepository.
func NewNotificationRepository() *NotificationRepository {
	return &NotificationRepository{byID: map[uint]*models.Notification{}}
}

// Create inserts n, assigning it a new ID.
func (r *NotificationRepository) Create(n *models.Notification) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID++
	n.ID = r.nextID
	cp := *n
	r.byID[n.ID] = &cp
	return nil
}

// FindByRecipient returns notifications for recipientID, newest first,
// optionally filtered to unread only.
func (r *NotificationRepository) FindByRecipient(recipientID uint, unreadOnly bool) ([]models.Notification, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []models.Notification{}
	for _, n := range r.byID {
		if n.RecipientID != recipientID || (unreadOnly && n.Read) {
			continue
		}
		out = append(out, *n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out, nil
}

// FindByRecipientPaginated returns a page of notifications for recipientID.
func (r *NotificationRepository) FindByRecipientPaginated(recipientID uint, unreadOnly bool, page, pageSize int) (*repository.PaginatedResult[models.Notification], error) {
	all, _ := r.FindByRecipient(recipientID, unreadOnly)
	start, end := paginationBounds(len(all), page, pageSize)
	items := append([]models.Notification{}, all[start:end]...)
	return &repository.PaginatedResult[models.Notification]{
		Items: items, Total: int64(len(all)), Page: page, PageSize: pageSize,
		TotalPages: totalPages(len(all), pageSize),
	}, nil
}

// GetByID returns the notification with the given ID, or repository.ErrNotFound.
func (r *NotificationRepository) GetByID(id uint) (*models.Notification, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	n, ok := r.byID[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	cp := *n
	return &cp, nil
}

// Save inserts n (assigning a new ID) if its ID is zero, else overwrites the
// existing record.
func (r *NotificationRepository) Save(n *models.Notification) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if n.ID == 0 {
		r.nextID++
		n.ID = r.nextID
	}
	cp := *n
	r.byID[n.ID] = &cp
	return nil
}

// MarkAllReadForRecipient marks every notification for recipientID as read.
func (r *NotificationRepository) MarkAllReadForRecipient(recipientID uint) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, n := range r.byID {
		if n.RecipientID == recipientID {
			n.Read = true
		}
	}
	return nil
}

// Count returns the number of stored notifications — a test helper, not
// part of the repository.NotificationRepository interface.
func (r *NotificationRepository) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.byID)
}

// AnnouncementRepository is an in-memory fake of repository.AnnouncementRepository.
type AnnouncementRepository struct {
	mu     sync.Mutex
	nextID uint
	byID   map[uint]*models.Announcement
}

// NewAnnouncementRepository creates an empty fake AnnouncementRepository.
func NewAnnouncementRepository() *AnnouncementRepository {
	return &AnnouncementRepository{byID: map[uint]*models.Announcement{}}
}

// Create inserts a, assigning it a new ID.
func (r *AnnouncementRepository) Create(a *models.Announcement) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID++
	a.ID = r.nextID
	cp := *a
	r.byID[a.ID] = &cp
	return nil
}

// GetByID returns the announcement with the given ID, or repository.ErrNotFound.
func (r *AnnouncementRepository) GetByID(id uint) (*models.Announcement, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	a, ok := r.byID[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	cp := *a
	return &cp, nil
}

// Save inserts a (assigning a new ID) if its ID is zero, else overwrites the
// existing record.
func (r *AnnouncementRepository) Save(a *models.Announcement) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if a.ID == 0 {
		r.nextID++
		a.ID = r.nextID
	}
	cp := *a
	r.byID[a.ID] = &cp
	return nil
}

// Delete removes the announcement with the given ID from the store. Deleting
// a missing ID is a no-op, matching GORM's Delete semantics.
func (r *AnnouncementRepository) Delete(id uint) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.byID, id)
	return nil
}

// ListActive returns active announcements, newest first.
func (r *AnnouncementRepository) ListActive() ([]models.Announcement, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []models.Announcement{}
	for _, a := range r.byID {
		if a.Active {
			out = append(out, *a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out, nil
}

// ListPaginated returns a page of every announcement (active and inactive), newest first.
func (r *AnnouncementRepository) ListPaginated(page, pageSize int) (*repository.PaginatedResult[models.Announcement], error) {
	r.mu.Lock()
	all := make([]models.Announcement, 0, len(r.byID))
	for _, a := range r.byID {
		all = append(all, *a)
	}
	r.mu.Unlock()
	sort.Slice(all, func(i, j int) bool { return all[i].ID > all[j].ID })
	start, end := paginationBounds(len(all), page, pageSize)
	items := append([]models.Announcement{}, all[start:end]...)
	return &repository.PaginatedResult[models.Announcement]{
		Items: items, Total: int64(len(all)), Page: page, PageSize: pageSize,
		TotalPages: totalPages(len(all), pageSize),
	}, nil
}

// WaitlistRepository is an in-memory fake of repository.WaitlistRepository.
type WaitlistRepository struct {
	mu      sync.Mutex
	nextID  uint
	entries map[uint]*models.WaitlistEntry
}

// NewWaitlistRepository creates an empty fake WaitlistRepository.
func NewWaitlistRepository() *WaitlistRepository {
	return &WaitlistRepository{entries: map[uint]*models.WaitlistEntry{}}
}

// Add joins userID to copyID's waitlist, or returns repository.ErrConflict
// if already on it.
func (r *WaitlistRepository) Add(copyID, userID uint) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.entries {
		if e.CopyID == copyID && e.UserID == userID {
			return repository.ErrConflict
		}
	}
	r.nextID++
	r.entries[r.nextID] = &models.WaitlistEntry{ID: r.nextID, CopyID: copyID, UserID: userID}
	return nil
}

// Remove takes userID off copyID's waitlist, or returns repository.ErrNotFound.
func (r *WaitlistRepository) Remove(copyID, userID uint) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, e := range r.entries {
		if e.CopyID == copyID && e.UserID == userID {
			delete(r.entries, id)
			return nil
		}
	}
	return repository.ErrNotFound
}

// ListByCopyID returns all waitlist entries for copyID, ordered by ID (join order).
func (r *WaitlistRepository) ListByCopyID(copyID uint) ([]models.WaitlistEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []models.WaitlistEntry{}
	for _, e := range r.entries {
		if e.CopyID == copyID {
			out = append(out, *e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// Count returns the number of entries on copyID's waitlist.
func (r *WaitlistRepository) Count(copyID uint) (int64, error) {
	items, _ := r.ListByCopyID(copyID)
	return int64(len(items)), nil
}

// IsOnWaitlist reports whether userID is on copyID's waitlist.
func (r *WaitlistRepository) IsOnWaitlist(copyID, userID uint) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.entries {
		if e.CopyID == copyID && e.UserID == userID {
			return true, nil
		}
	}
	return false, nil
}

// DeleteByCopyID removes every waitlist entry for copyID.
func (r *WaitlistRepository) DeleteByCopyID(copyID uint) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, e := range r.entries {
		if e.CopyID == copyID {
			delete(r.entries, id)
		}
	}
	return nil
}

// LoanRequestRepository is an in-memory fake of repository.LoanRequestRepository.
// It delegates to a CopyRepository, NotificationRepository, and
// UserRepository to reproduce the cross-table effects of
// CreateAndMarkRequested and RejectCompetingAndUpdateCopy, and to hydrate
// the Copy/Borrower associations the GetByIDWith* methods populate via
// GORM's Preload in production.
type LoanRequestRepository struct {
	mu     sync.Mutex
	nextID uint
	byID   map[uint]*models.LoanRequest
	copies *CopyRepository
	notifs *NotificationRepository
	users  *UserRepository
}

// NewLoanRequestRepository creates an empty fake LoanRequestRepository backed
// by the given copy, notification, and user fakes.
func NewLoanRequestRepository(copies *CopyRepository, notifs *NotificationRepository, users *UserRepository) *LoanRequestRepository {
	return &LoanRequestRepository{byID: map[uint]*models.LoanRequest{}, copies: copies, notifs: notifs, users: users}
}

// hydrate populates lr.Copy and lr.Borrower from the backing repositories,
// mimicking GORM's Preload("Copy")/Preload("Borrower").
func (r *LoanRequestRepository) hydrate(lr *models.LoanRequest) {
	if c, err := r.copies.GetByID(lr.CopyID); err == nil {
		lr.Copy = *c
	}
	if u, err := r.users.FindByID(lr.BorrowerID); err == nil {
		lr.Borrower = *u
	}
}

// Create inserts lr, assigning it a new ID and defaulting Status to "pending".
func (r *LoanRequestRepository) Create(lr *models.LoanRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID++
	lr.ID = r.nextID
	if lr.Status == "" {
		lr.Status = "pending"
	}
	cp := *lr
	r.byID[lr.ID] = &cp
	return nil
}

// CreateAndMarkRequested re-checks the copy's availability, creates lr, and
// marks the copy "requested" — mirroring the real transaction's TOCTOU guard.
func (r *LoanRequestRepository) CreateAndMarkRequested(lr *models.LoanRequest) error {
	copyRec, err := r.copies.GetByID(lr.CopyID)
	if err != nil {
		return err
	}
	if copyRec.Status != "available" {
		return repository.ErrConflict
	}
	if err := r.Create(lr); err != nil {
		return err
	}
	return r.copies.UpdateStatus(lr.CopyID, "requested")
}

// GetByID returns the loan request with the given ID, or repository.ErrNotFound.
func (r *LoanRequestRepository) GetByID(id uint) (*models.LoanRequest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	lr, ok := r.byID[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	cp := *lr
	return &cp, nil
}

// GetByIDWithCopyAndBorrower returns the loan request with its Copy and
// Borrower associations populated (see hydrate).
func (r *LoanRequestRepository) GetByIDWithCopyAndBorrower(id uint) (*models.LoanRequest, error) {
	lr, err := r.GetByID(id)
	if err != nil {
		return nil, err
	}
	r.hydrate(lr)
	return lr, nil
}

// GetByIDWithFullAssociations behaves like GetByIDWithCopyAndBorrower — the
// fake doesn't distinguish depth of nested preloads (e.g. Copy.Book).
func (r *LoanRequestRepository) GetByIDWithFullAssociations(id uint) (*models.LoanRequest, error) {
	return r.GetByIDWithCopyAndBorrower(id)
}

// GetByIDWithCopyOwnerAndBorrower behaves like GetByIDWithCopyAndBorrower.
func (r *LoanRequestRepository) GetByIDWithCopyOwnerAndBorrower(id uint) (*models.LoanRequest, error) {
	return r.GetByIDWithCopyAndBorrower(id)
}

// ListByCopyID returns loan requests for copyID, newest first, with Copy and
// Borrower associations populated (see hydrate).
func (r *LoanRequestRepository) ListByCopyID(copyID uint) ([]models.LoanRequest, error) {
	r.mu.Lock()
	out := []models.LoanRequest{}
	for _, lr := range r.byID {
		if lr.CopyID == copyID {
			out = append(out, *lr)
		}
	}
	r.mu.Unlock()
	for i := range out {
		r.hydrate(&out[i])
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RequestedAt.After(out[j].RequestedAt) })
	return out, nil
}

// ListByBorrowerID returns loan requests made by borrowerID, newest first,
// with Copy and Borrower associations populated (see hydrate).
func (r *LoanRequestRepository) ListByBorrowerID(borrowerID uint) ([]models.LoanRequest, error) {
	r.mu.Lock()
	out := []models.LoanRequest{}
	for _, lr := range r.byID {
		if lr.BorrowerID == borrowerID {
			out = append(out, *lr)
		}
	}
	r.mu.Unlock()
	for i := range out {
		r.hydrate(&out[i])
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RequestedAt.After(out[j].RequestedAt) })
	return out, nil
}

// ListByBorrowerIDPaginated returns a page of loan requests made by
// borrowerID, optionally filtered to the given statuses.
func (r *LoanRequestRepository) ListByBorrowerIDPaginated(borrowerID uint, statuses []string, page, pageSize int) (*repository.PaginatedResult[models.LoanRequest], error) {
	all, _ := r.ListByBorrowerID(borrowerID)
	if len(statuses) > 0 {
		set := map[string]bool{}
		for _, s := range statuses {
			set[s] = true
		}
		filtered := make([]models.LoanRequest, 0, len(all))
		for _, lr := range all {
			if set[lr.Status] {
				filtered = append(filtered, lr)
			}
		}
		all = filtered
	}
	start, end := paginationBounds(len(all), page, pageSize)
	items := append([]models.LoanRequest{}, all[start:end]...)
	return &repository.PaginatedResult[models.LoanRequest]{
		Items: items, Total: int64(len(all)), Page: page, PageSize: pageSize,
		TotalPages: totalPages(len(all), pageSize),
	}, nil
}

// ListActiveByBorrowerID returns borrowerID's accepted loan requests,
// mimicking the GORM implementation's due-date-ascending / NULLs-last order.
func (r *LoanRequestRepository) ListActiveByBorrowerID(borrowerID uint) ([]models.LoanRequest, error) {
	r.mu.Lock()
	out := []models.LoanRequest{}
	for _, lr := range r.byID {
		if lr.BorrowerID == borrowerID && lr.Status == "accepted" {
			out = append(out, *lr)
		}
	}
	r.mu.Unlock()
	for i := range out {
		r.hydrate(&out[i])
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i].ExpectedReturnDate, out[j].ExpectedReturnDate
		if a == nil && b == nil {
			return out[i].RequestedAt.Before(out[j].RequestedAt)
		}
		if a == nil {
			return false
		}
		if b == nil {
			return true
		}
		return a.Before(*b)
	})
	return out, nil
}

// Save inserts lr (assigning a new ID) if its ID is zero, else overwrites
// the existing record.
func (r *LoanRequestRepository) Save(lr *models.LoanRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if lr.ID == 0 {
		r.nextID++
		lr.ID = r.nextID
	}
	cp := *lr
	r.byID[lr.ID] = &cp
	return nil
}

// RejectCompetingAndUpdateCopy rejects every other pending request for
// copyID, creates a rejection notification for each, and marks the copy
// "loaned" — mirroring the real transaction.
func (r *LoanRequestRepository) RejectCompetingAndUpdateCopy(copyID, acceptedLoanID uint) error {
	r.mu.Lock()
	var others []*models.LoanRequest
	for _, lr := range r.byID {
		if lr.CopyID == copyID && lr.ID != acceptedLoanID && lr.Status == "pending" {
			lr.Status = "rejected"
			others = append(others, lr)
		}
	}
	r.mu.Unlock()

	for _, o := range others {
		if err := r.notifs.Create(&models.Notification{
			RecipientID: o.BorrowerID, Type: "request_rejected", LoanRequestID: &o.ID,
		}); err != nil {
			return err
		}
	}
	return r.copies.UpdateStatus(copyID, "loaned")
}

// CountPendingForCopyExcluding counts pending requests for copyID other than excludeID.
func (r *LoanRequestRepository) CountPendingForCopyExcluding(copyID, excludeID uint) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var count int64
	for _, lr := range r.byID {
		if lr.CopyID == copyID && lr.ID != excludeID && lr.Status == "pending" {
			count++
		}
	}
	return count, nil
}

// CountActiveLoansByBorrower counts pending/accepted/loaned requests made by borrowerID.
func (r *LoanRequestRepository) CountActiveLoansByBorrower(borrowerID uint) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	active := map[string]bool{"pending": true, "accepted": true, "loaned": true}
	var count int64
	for _, lr := range r.byID {
		if lr.BorrowerID == borrowerID && active[lr.Status] {
			count++
		}
	}
	return count, nil
}

// BookRepository is an in-memory fake of repository.BookRepository. It only
// implements the querying needed by wishlist's fulfill handler (GetByID
// lookup, Create for test fixtures) — List/ListPaginated/ListRecent and the
// available-copies counters return empty/zero, since nothing that consumes
// this fake exercises them yet.
type BookRepository struct {
	mu     sync.Mutex
	nextID uint
	byID   map[uint]*models.Book
	copies *CopyRepository
}

// NewBookRepository creates an empty fake BookRepository.
func NewBookRepository() *BookRepository {
	return &BookRepository{byID: map[uint]*models.Book{}}
}

// SetCopies wires copies in so CountCopies can answer from its data — a test
// helper, not part of the repository.BookRepository interface. Tests that
// don't call this get a CountCopies that always returns 0.
func (r *BookRepository) SetCopies(copies *CopyRepository) {
	r.copies = copies
}

// Create inserts book, assigning it a new ID.
func (r *BookRepository) Create(book *models.Book) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID++
	book.ID = r.nextID
	cp := *book
	r.byID[book.ID] = &cp
	return nil
}

// Save inserts book (assigning a new ID) if its ID is zero, else overwrites
// the existing record.
func (r *BookRepository) Save(book *models.Book) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if book.ID == 0 {
		r.nextID++
		book.ID = r.nextID
	}
	cp := *book
	r.byID[book.ID] = &cp
	return nil
}

// FindByOLKey returns the book with the given OLKey, or repository.ErrNotFound.
func (r *BookRepository) FindByOLKey(olKey string) (*models.Book, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, b := range r.byID {
		if olKey != "" && b.OLKey == olKey {
			cp := *b
			return &cp, nil
		}
	}
	return nil, repository.ErrNotFound
}

// FindByGoogleBooksID returns the book with the given GoogleBooksID, or repository.ErrNotFound.
func (r *BookRepository) FindByGoogleBooksID(id string) (*models.Book, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, b := range r.byID {
		if id != "" && b.GoogleBooksID == id {
			cp := *b
			return &cp, nil
		}
	}
	return nil, repository.ErrNotFound
}

// FindByISBN returns the book with the given ISBN, or repository.ErrNotFound.
// Excludes empty-string ISBNs so two books that both lack one don't collide.
func (r *BookRepository) FindByISBN(isbn string) (*models.Book, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, b := range r.byID {
		if isbn != "" && b.ISBN == isbn {
			cp := *b
			return &cp, nil
		}
	}
	return nil, repository.ErrNotFound
}

// GetByIDWithCopies returns the book with the given ID, or repository.ErrNotFound.
// The fake stores no Copies association — callers needing one must populate
// it on the models.Book passed to Create.
func (r *BookRepository) GetByIDWithCopies(id uint) (*models.Book, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	b, ok := r.byID[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	cp := *b
	return &cp, nil
}

// List returns nil — not exercised by any test using this fake yet.
func (r *BookRepository) List(_, _ string, _ bool) ([]models.Book, error) { return nil, nil }

// Count returns the number of stored books — a test helper, not part of the
// repository.BookRepository interface.
func (r *BookRepository) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.byID)
}

// ListPaginated returns an empty page — not exercised by any test using this fake yet.
func (r *BookRepository) ListPaginated(_, _ string, _ bool, page, pageSize int) (*repository.PaginatedResult[models.Book], error) {
	return &repository.PaginatedResult[models.Book]{Page: page, PageSize: pageSize}, nil
}

// ListRecent returns nil — not exercised by any test using this fake yet.
func (r *BookRepository) ListRecent(_ int) ([]models.Book, error) { return nil, nil }

// CountAvailableCopies returns 0 — not exercised by any test using this fake yet.
func (r *BookRepository) CountAvailableCopies(_ uint) (int64, error) { return 0, nil }

// CountAvailableCopiesBatch returns an empty map — not exercised by any test using this fake yet.
func (r *BookRepository) CountAvailableCopiesBatch(_ []uint) (map[uint]int64, error) {
	return map[uint]int64{}, nil
}

// Delete removes book from the store.
func (r *BookRepository) Delete(book *models.Book) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.byID, book.ID)
	return nil
}

// CountCopies returns the number of copies stored against bookID, delegating
// to copies since the fake BookRepository holds no Copy rows of its own.
func (r *BookRepository) CountCopies(bookID uint) (int64, error) {
	if r.copies == nil {
		return 0, nil
	}
	r.copies.mu.Lock()
	defer r.copies.mu.Unlock()
	var count int64
	for _, c := range r.copies.byID {
		if c.BookID == bookID {
			count++
		}
	}
	return count, nil
}

// WishlistRequestRepository is an in-memory fake of
// repository.WishlistRequestRepository.
type WishlistRequestRepository struct {
	mu     sync.Mutex
	nextID uint
	byID   map[uint]*models.WishlistRequest
}

// NewWishlistRequestRepository creates an empty fake WishlistRequestRepository.
func NewWishlistRequestRepository() *WishlistRequestRepository {
	return &WishlistRequestRepository{byID: map[uint]*models.WishlistRequest{}}
}

// Create inserts req, assigning it a new ID.
func (r *WishlistRequestRepository) Create(req *models.WishlistRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID++
	req.ID = r.nextID
	cp := *req
	r.byID[req.ID] = &cp
	return nil
}

// GetByID returns the request with the given ID, or repository.ErrNotFound.
func (r *WishlistRequestRepository) GetByID(id uint) (*models.WishlistRequest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	req, ok := r.byID[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	cp := *req
	return &cp, nil
}

// Save inserts req (assigning a new ID) if its ID is zero, else overwrites
// the existing record.
func (r *WishlistRequestRepository) Save(req *models.WishlistRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if req.ID == 0 {
		r.nextID++
		req.ID = r.nextID
	}
	cp := *req
	r.byID[req.ID] = &cp
	return nil
}

// ListOpenPaginated returns a page of open requests, optionally filtered by
// a case-sensitive title/author substring search, newest first.
func (r *WishlistRequestRepository) ListOpenPaginated(search string, page, pageSize int) (*repository.PaginatedResult[models.WishlistRequest], error) {
	r.mu.Lock()
	all := make([]models.WishlistRequest, 0, len(r.byID))
	for _, req := range r.byID {
		if req.Status != "open" {
			continue
		}
		if search != "" && !strings.Contains(req.Title, search) && !strings.Contains(req.Author, search) {
			continue
		}
		all = append(all, *req)
	}
	r.mu.Unlock()
	sort.Slice(all, func(i, j int) bool { return all[i].CreatedAt.After(all[j].CreatedAt) })
	start, end := paginationBounds(len(all), page, pageSize)
	items := append([]models.WishlistRequest{}, all[start:end]...)
	return &repository.PaginatedResult[models.WishlistRequest]{
		Items: items, Total: int64(len(all)), Page: page, PageSize: pageSize,
		TotalPages: totalPages(len(all), pageSize),
	}, nil
}

// ListByRequesterID returns every request made by requesterID, newest first.
func (r *WishlistRequestRepository) ListByRequesterID(requesterID uint) ([]models.WishlistRequest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []models.WishlistRequest{}
	for _, req := range r.byID {
		if req.RequesterID == requesterID {
			out = append(out, *req)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

// FindOpenByOLKey returns every open request with the given OLKey.
func (r *WishlistRequestRepository) FindOpenByOLKey(olKey string) ([]models.WishlistRequest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []models.WishlistRequest{}
	if olKey == "" {
		return out, nil
	}
	for _, req := range r.byID {
		if req.Status == "open" && req.OLKey == olKey {
			out = append(out, *req)
		}
	}
	return out, nil
}

// FindOpenByGoogleBooksID returns every open request with the given GoogleBooksID.
func (r *WishlistRequestRepository) FindOpenByGoogleBooksID(googleBooksID string) ([]models.WishlistRequest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []models.WishlistRequest{}
	if googleBooksID == "" {
		return out, nil
	}
	for _, req := range r.byID {
		if req.Status == "open" && req.GoogleBooksID == googleBooksID {
			out = append(out, *req)
		}
	}
	return out, nil
}

// wishlistMatchesAnyKey reports whether req shares any of the given
// non-empty external keys.
func wishlistMatchesAnyKey(req *models.WishlistRequest, isbn, olKey, googleBooksID string) bool {
	return (isbn != "" && req.ISBN == isbn) ||
		(olKey != "" && req.OLKey == olKey) ||
		(googleBooksID != "" && req.GoogleBooksID == googleBooksID)
}

// FindOpenMatch returns the earliest open request matching any of the given
// external keys, or nil if no key is given or none match.
func (r *WishlistRequestRepository) FindOpenMatch(isbn, olKey, googleBooksID string) (*models.WishlistRequest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if isbn == "" && olKey == "" && googleBooksID == "" {
		return nil, nil
	}
	var match *models.WishlistRequest
	for _, req := range r.byID {
		if req.Status != "open" || !wishlistMatchesAnyKey(req, isbn, olKey, googleBooksID) {
			continue
		}
		if match == nil || req.CreatedAt.Before(match.CreatedAt) {
			match = req
		}
	}
	if match == nil {
		return nil, nil
	}
	cp := *match
	return &cp, nil
}

// ClearFulfilledBookID nulls FulfilledBookID on every stored request pointing at bookID.
func (r *WishlistRequestRepository) ClearFulfilledBookID(bookID uint) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, req := range r.byID {
		if req.FulfilledBookID != nil && *req.FulfilledBookID == bookID {
			req.FulfilledBookID = nil
		}
	}
	return nil
}

// paginationBounds returns the [start, end) slice bounds for page/pageSize
// over a collection of the given length.
func paginationBounds(length, page, pageSize int) (start, end int) {
	start = (page - 1) * pageSize
	if start > length {
		start = length
	}
	if start < 0 {
		start = 0
	}
	end = start + pageSize
	if end > length {
		end = length
	}
	return start, end
}

// totalPages returns the number of pages of pageSize needed to cover length items.
func totalPages(length, pageSize int) int {
	return (length + pageSize - 1) / pageSize
}

var (
	_ repository.UserRepository                     = (*UserRepository)(nil)
	_ repository.AdminRepository                    = (*AdminRepository)(nil)
	_ repository.CopyRepository                     = (*CopyRepository)(nil)
	_ repository.NotificationRepository             = (*NotificationRepository)(nil)
	_ repository.AnnouncementRepository             = (*AnnouncementRepository)(nil)
	_ repository.WaitlistRepository                 = (*WaitlistRepository)(nil)
	_ repository.LoanRequestRepository              = (*LoanRequestRepository)(nil)
	_ repository.RegistrationVerificationRepository = (*RegistrationVerificationRepository)(nil)
	_ repository.WishlistRequestRepository          = (*WishlistRequestRepository)(nil)
	_ repository.BookRepository                     = (*BookRepository)(nil)
)
