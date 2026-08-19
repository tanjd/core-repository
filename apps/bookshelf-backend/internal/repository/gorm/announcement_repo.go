package gorm

import (
	"errors"

	"gorm.io/gorm"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

// AnnouncementRepository is the GORM implementation of repository.AnnouncementRepository.
type AnnouncementRepository struct {
	db *gorm.DB
}

// NewAnnouncementRepository creates a new AnnouncementRepository.
func NewAnnouncementRepository(db *gorm.DB) *AnnouncementRepository {
	return &AnnouncementRepository{db: db}
}

func (r *AnnouncementRepository) Create(a *models.Announcement) error {
	return r.db.Create(a).Error
}

func (r *AnnouncementRepository) GetByID(id uint) (*models.Announcement, error) {
	var a models.Announcement
	if err := r.db.First(&a, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}

func (r *AnnouncementRepository) Save(a *models.Announcement) error {
	return r.db.Save(a).Error
}

func (r *AnnouncementRepository) Delete(id uint) error {
	return r.db.Delete(&models.Announcement{}, id).Error
}

func (r *AnnouncementRepository) ListActive() ([]models.Announcement, error) {
	var out []models.Announcement
	if err := r.db.Where("active = ?", true).Order("created_at desc").Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func (r *AnnouncementRepository) ListPaginated(page, pageSize int) (*repository.PaginatedResult[models.Announcement], error) {
	var total int64
	if err := r.db.Model(&models.Announcement{}).Count(&total).Error; err != nil {
		return nil, err
	}
	var items []models.Announcement
	offset := (page - 1) * pageSize
	if err := r.db.Order("created_at desc").Offset(offset).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, err
	}
	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
	return &repository.PaginatedResult[models.Announcement]{
		Items: items, Total: total, Page: page, PageSize: pageSize, TotalPages: totalPages,
	}, nil
}
