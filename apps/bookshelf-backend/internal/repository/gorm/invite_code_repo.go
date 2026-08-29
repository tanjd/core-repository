package gorm

import (
	"errors"

	"gorm.io/gorm"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

// InviteCodeRepository is the GORM implementation of
// repository.InviteCodeRepository.
type InviteCodeRepository struct {
	db *gorm.DB
}

// NewInviteCodeRepository creates a new InviteCodeRepository.
func NewInviteCodeRepository(db *gorm.DB) *InviteCodeRepository {
	return &InviteCodeRepository{db: db}
}

// FindOrCreateByInviter returns inviterID's existing code, or inserts one
// with code if none exists yet. Two concurrent first-time callers can both
// reach the Create below — the uniqueIndex on inviter_id lets exactly one
// succeed, and the loser falls back to a plain lookup so both callers end up
// with the same row, rather than one of them erroring.
func (r *InviteCodeRepository) FindOrCreateByInviter(inviterID uint, code string) (*models.InviteCode, error) {
	ic, err := r.findByInviter(inviterID)
	if err == nil {
		return ic, nil
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}

	ic = &models.InviteCode{Code: code, InviterID: inviterID}
	if createErr := r.db.Create(ic).Error; createErr != nil {
		if isUniqueViolation(createErr) {
			return r.findByInviter(inviterID)
		}
		return nil, createErr
	}
	return ic, nil
}

func (r *InviteCodeRepository) findByInviter(inviterID uint) (*models.InviteCode, error) {
	var ic models.InviteCode
	if err := r.db.Where("inviter_id = ?", inviterID).First(&ic).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &ic, nil
}

func (r *InviteCodeRepository) FindByCode(code string) (*models.InviteCode, error) {
	var ic models.InviteCode
	if err := r.db.Where("code = ?", code).First(&ic).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &ic, nil
}

// Regenerate deletes inviterID's existing code and creates a new one in a
// single transaction — the old code is deleted first so the uniqueIndex on
// inviter_id doesn't block the new insert, and the transaction means there
// is never a window where neither code exists.
func (r *InviteCodeRepository) Regenerate(inviterID uint, newCode string) (*models.InviteCode, error) {
	ic := models.InviteCode{Code: newCode, InviterID: inviterID}
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if delErr := tx.Where("inviter_id = ?", inviterID).Delete(&models.InviteCode{}).Error; delErr != nil {
			return delErr
		}
		return tx.Create(&ic).Error
	})
	if err != nil {
		return nil, err
	}
	return &ic, nil
}

func (r *InviteCodeRepository) DeleteByInviter(inviterID uint) error {
	return r.db.Where("inviter_id = ?", inviterID).Delete(&models.InviteCode{}).Error
}

func (r *InviteCodeRepository) DeleteByID(id uint) error {
	res := r.db.Delete(&models.InviteCode{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *InviteCodeRepository) ListAll() ([]models.InviteCode, error) {
	var codes []models.InviteCode
	err := r.db.Preload("Inviter").Order("created_at DESC").Find(&codes).Error
	return codes, err
}
