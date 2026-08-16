package gorm

import (
	"errors"

	"gorm.io/gorm"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

// NotificationRepository is the GORM implementation of repository.NotificationRepository.
type NotificationRepository struct {
	db *gorm.DB
}

// NewNotificationRepository creates a new NotificationRepository.
func NewNotificationRepository(db *gorm.DB) *NotificationRepository {
	return &NotificationRepository{db: db}
}

func (r *NotificationRepository) Create(n *models.Notification) error {
	return r.db.Create(n).Error
}

func (r *NotificationRepository) FindByRecipient(recipientID uint, unreadOnly bool) ([]models.Notification, error) {
	var notifications []models.Notification
	tx := r.db.Where("recipient_id = ?", recipientID).Order("created_at DESC")
	if unreadOnly {
		tx = tx.Where("read = ?", false)
	}
	if err := tx.Find(&notifications).Error; err != nil {
		return nil, err
	}
	return notifications, nil
}

func (r *NotificationRepository) FindByRecipientPaginated(recipientID uint, unreadOnly bool, page, pageSize int) (*repository.PaginatedResult[models.Notification], error) {
	var total int64
	cq := r.db.Model(&models.Notification{}).Where("recipient_id = ?", recipientID)
	if unreadOnly {
		cq = cq.Where("read = ?", false)
	}
	if err := cq.Count(&total).Error; err != nil {
		return nil, err
	}
	var notifications []models.Notification
	offset := (page - 1) * pageSize
	fq := r.db.Where("recipient_id = ?", recipientID).Order("created_at DESC").Offset(offset).Limit(pageSize)
	if unreadOnly {
		fq = fq.Where("read = ?", false)
	}
	if err := fq.Find(&notifications).Error; err != nil {
		return nil, err
	}
	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
	return &repository.PaginatedResult[models.Notification]{
		Items: notifications, Total: total, Page: page, PageSize: pageSize, TotalPages: totalPages,
	}, nil
}

func (r *NotificationRepository) GetByID(id uint) (*models.Notification, error) {
	var n models.Notification
	if err := r.db.First(&n, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &n, nil
}

func (r *NotificationRepository) Save(n *models.Notification) error {
	return r.db.Save(n).Error
}

func (r *NotificationRepository) MarkAllReadForRecipient(recipientID uint) error {
	return r.db.Model(&models.Notification{}).
		Where("recipient_id = ? AND read = ?", recipientID, false).
		Update("read", true).Error
}
