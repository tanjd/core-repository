package gorm

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

// RegistrationVerificationRepository is the GORM implementation of
// repository.RegistrationVerificationRepository.
type RegistrationVerificationRepository struct {
	db *gorm.DB
}

// NewRegistrationVerificationRepository creates a new RegistrationVerificationRepository.
func NewRegistrationVerificationRepository(db *gorm.DB) *RegistrationVerificationRepository {
	return &RegistrationVerificationRepository{db: db}
}

func (r *RegistrationVerificationRepository) Upsert(channel, identifier, code string, expiresAt time.Time) error {
	var existing models.RegistrationVerification
	err := r.db.Where("channel = ? AND identifier = ?", channel, identifier).First(&existing).Error
	if err == nil {
		existing.Code = code
		existing.ExpiresAt = expiresAt
		return r.db.Save(&existing).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return r.db.Create(&models.RegistrationVerification{
		Channel:    channel,
		Identifier: identifier,
		Code:       code,
		ExpiresAt:  expiresAt,
	}).Error
}

func (r *RegistrationVerificationRepository) Find(channel, identifier string) (*models.RegistrationVerification, error) {
	var v models.RegistrationVerification
	if err := r.db.Where("channel = ? AND identifier = ?", channel, identifier).First(&v).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &v, nil
}

func (r *RegistrationVerificationRepository) Delete(channel, identifier string) error {
	return r.db.Where("channel = ? AND identifier = ?", channel, identifier).Delete(&models.RegistrationVerification{}).Error
}
