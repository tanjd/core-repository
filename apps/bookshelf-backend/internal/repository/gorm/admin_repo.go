package gorm

import (
	"errors"

	"gorm.io/gorm"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

// AdminRepository is the GORM implementation of repository.AdminRepository.
type AdminRepository struct {
	db *gorm.DB
}

// NewAdminRepository creates a new AdminRepository.
func NewAdminRepository(db *gorm.DB) *AdminRepository {
	return &AdminRepository{db: db}
}

func (r *AdminRepository) ListUsers() ([]models.User, error) {
	var users []models.User
	if err := r.db.Order("created_at asc").Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func (r *AdminRepository) ListUsersPaginated(page, pageSize int) (*repository.PaginatedResult[models.User], error) {
	var total int64
	if err := r.db.Model(&models.User{}).Count(&total).Error; err != nil {
		return nil, err
	}
	var users []models.User
	offset := (page - 1) * pageSize
	if err := r.db.Order("created_at asc").Offset(offset).Limit(pageSize).Find(&users).Error; err != nil {
		return nil, err
	}
	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
	return &repository.PaginatedResult[models.User]{
		Items: users, Total: total, Page: page, PageSize: pageSize, TotalPages: totalPages,
	}, nil
}

func (r *AdminRepository) FindUserByID(id uint) (*models.User, error) {
	var user models.User
	if err := r.db.First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *AdminRepository) SaveUser(user *models.User) error {
	return r.db.Save(user).Error
}

func (r *AdminRepository) DeleteUser(id uint) error {
	return r.db.Delete(&models.User{}, id).Error
}

func (r *AdminRepository) GetSettings() ([]models.AppSetting, error) {
	var settings []models.AppSetting
	if err := r.db.Order("key asc").Find(&settings).Error; err != nil {
		return nil, err
	}
	return settings, nil
}

func (r *AdminRepository) GetSetting(key string) (string, error) {
	var setting models.AppSetting
	if err := r.db.First(&setting, "key = ?", key).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", repository.ErrNotFound
		}
		return "", err
	}
	return setting.Value, nil
}

func (r *AdminRepository) UpsertSetting(key, value string) error {
	return r.db.Where(models.AppSetting{Key: key}).
		Assign(models.AppSetting{Value: value}).
		FirstOrCreate(&models.AppSetting{}).Error
}

func (r *AdminRepository) CountByRole(role string) (int64, error) {
	var count int64
	err := r.db.Model(&models.User{}).Where("role = ?", role).Count(&count).Error
	return count, err
}
