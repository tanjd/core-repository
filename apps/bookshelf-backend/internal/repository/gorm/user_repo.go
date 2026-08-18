package gorm

import (
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

func (r *UserRepository) FindByEmail(email string) (*models.User, error) {
	var user models.User
	if err := r.db.Where("email = ?", email).First(&user).Error; err != nil {
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

func (r *UserRepository) Save(user *models.User) error {
	return r.db.Save(user).Error
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
