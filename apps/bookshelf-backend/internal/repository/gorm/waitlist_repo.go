package gorm

import (
	"errors"
	"strings"

	"gorm.io/gorm"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

// WaitlistRepository is the GORM implementation of repository.WaitlistRepository.
type WaitlistRepository struct {
	db *gorm.DB
}

// NewWaitlistRepository creates a new WaitlistRepository.
func NewWaitlistRepository(db *gorm.DB) *WaitlistRepository {
	return &WaitlistRepository{db: db}
}

func (r *WaitlistRepository) Add(copyID, userID uint) error {
	entry := models.WaitlistEntry{CopyID: copyID, UserID: userID}
	if err := r.db.Create(&entry).Error; err != nil {
		// SQLite unique constraint violation
		if isUniqueViolation(err) {
			return repository.ErrConflict
		}
		return err
	}
	return nil
}

func (r *WaitlistRepository) Remove(copyID, userID uint) error {
	result := r.db.Where("copy_id = ? AND user_id = ?", copyID, userID).Delete(&models.WaitlistEntry{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *WaitlistRepository) ListByCopyID(copyID uint) ([]models.WaitlistEntry, error) {
	var entries []models.WaitlistEntry
	err := r.db.Preload("User").
		Where("copy_id = ?", copyID).
		Order("created_at ASC").
		Find(&entries).Error
	return entries, err
}

func (r *WaitlistRepository) Count(copyID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.WaitlistEntry{}).Where("copy_id = ?", copyID).Count(&count).Error
	return count, err
}

func (r *WaitlistRepository) IsOnWaitlist(copyID, userID uint) (bool, error) {
	var count int64
	err := r.db.Model(&models.WaitlistEntry{}).
		Where("copy_id = ? AND user_id = ?", copyID, userID).
		Count(&count).Error
	return count > 0, err
}

func (r *WaitlistRepository) DeleteByCopyID(copyID uint) error {
	return r.db.Where("copy_id = ?", copyID).Delete(&models.WaitlistEntry{}).Error
}

// isUniqueViolation checks if an error is a SQLite unique constraint violation.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") || strings.Contains(msg, "unique constraint")
}
