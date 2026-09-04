package gorm

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

// UserRepository is the GORM implementation of repository.UserRepository.
type UserRepository struct {
	db *gorm.DB
}

// NewUserRepository creates a new UserRepository.
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(user *models.User) error {
	return r.db.Create(user).Error
}

// FindByEmail looks up a user by email, case-insensitively: an address is
// one identity whatever casing it's typed in, so "Ada@example.com" and
// "ada@example.com" must not resolve to different accounts (or to none,
// when the row was stored with the other casing). Backed by
// idx_users_email_nocase (migration 000011), which both makes this an index
// lookup rather than a scan and stops two case-variant rows existing in the
// first place.
//
// COLLATE NOCASE folds ASCII only, which is all an email address is in
// practice — domains are punycode and local parts are effectively ASCII.
func (r *UserRepository) FindByEmail(email string) (*models.User, error) {
	var user models.User
	if err := r.db.Where("email = ? COLLATE NOCASE", email).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) FindByID(id uint) (*models.User, error) {
	var user models.User
	if err := r.db.First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &user, nil
}

// Save persists user, mapping a unique-constraint violation (e.g. a
// TelegramChatID already linked to a different account) to
// repository.ErrConflict so callers can distinguish it from a generic
// failure.
func (r *UserRepository) Save(user *models.User) error {
	if err := r.db.Save(user).Error; err != nil {
		if isUniqueViolation(err) {
			return repository.ErrConflict
		}
		return err
	}
	return nil
}

func (r *UserRepository) HasAdmin() (bool, error) {
	var count int64
	err := r.db.Model(&models.User{}).Where("role = ?", "admin").Count(&count).Error
	return count > 0, err
}

// CreateAdminIfNoneExists atomically inserts user (with role "admin") only
// if no admin currently exists, preventing two concurrent /auth/setup calls
// from both succeeding.
func (r *UserRepository) CreateAdminIfNoneExists(user *models.User) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&models.User{}).Where("role = ?", "admin").Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return repository.ErrConflict
		}
		return tx.Create(user).Error
	})
}

// ListDigestRecipients returns members who should receive the monthly digest:
// verified, not suspended, not pending approval, and opted in.
func (r *UserRepository) ListDigestRecipients(ctx context.Context) ([]models.User, error) {
	var users []models.User
	err := r.db.WithContext(ctx).
		Where("verified = ? AND suspended = ? AND pending_approval = ? AND monthly_digest_enabled = ?",
			true, false, false, true).
		Find(&users).Error
	return users, err
}
