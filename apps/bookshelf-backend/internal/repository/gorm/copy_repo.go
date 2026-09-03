package gorm

import (
	"errors"

	"gorm.io/gorm"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

// CopyRepository is the GORM implementation of repository.CopyRepository.
type CopyRepository struct {
	db *gorm.DB
}

// NewCopyRepository creates a new CopyRepository.
func NewCopyRepository(db *gorm.DB) *CopyRepository {
	return &CopyRepository{db: db}
}

func (r *CopyRepository) Create(bookCopy *models.Copy) error {
	return r.db.Create(bookCopy).Error
}

func (r *CopyRepository) GetByID(id uint) (*models.Copy, error) {
	var bookCopy models.Copy
	if err := r.db.First(&bookCopy, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &bookCopy, nil
}

func (r *CopyRepository) GetByIDWithAssociations(id uint) (*models.Copy, error) {
	var bookCopy models.Copy
	if err := r.db.Preload("Book").Preload("Owner").First(&bookCopy, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &bookCopy, nil
}

func (r *CopyRepository) ListByOwnerID(ownerID uint) ([]models.Copy, error) {
	var copies []models.Copy
	if err := r.db.Preload("Book").Where("owner_id = ?", ownerID).Find(&copies).Error; err != nil {
		return nil, err
	}
	return copies, nil
}

func (r *CopyRepository) ListOwnedBookIDs(ownerID uint) ([]uint, error) {
	var bookIDs []uint
	if err := r.db.Model(&models.Copy{}).
		Where("owner_id = ?", ownerID).
		Distinct("book_id").
		Pluck("book_id", &bookIDs).Error; err != nil {
		return nil, err
	}
	return bookIDs, nil
}

func (r *CopyRepository) CountByOwnerID(ownerID uint) (int64, error) {
	var count int64
	if err := r.db.Model(&models.Copy{}).Where("owner_id = ?", ownerID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *CopyRepository) Save(bookCopy *models.Copy) error {
	return r.db.Save(bookCopy).Error
}

func (r *CopyRepository) Delete(bookCopy *models.Copy) error {
	return r.db.Delete(bookCopy).Error
}

func (r *CopyRepository) UpdateStatus(id uint, status string) error {
	return r.db.Model(&models.Copy{}).Where("id = ?", id).Update("status", status).Error
}
