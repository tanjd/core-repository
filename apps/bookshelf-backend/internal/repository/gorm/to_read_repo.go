package gorm

import (
	"gorm.io/gorm"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

// ToReadRepository is the GORM implementation of repository.ToReadRepository.
type ToReadRepository struct {
	db *gorm.DB
}

// NewToReadRepository creates a new ToReadRepository.
func NewToReadRepository(db *gorm.DB) *ToReadRepository {
	return &ToReadRepository{db: db}
}

func (r *ToReadRepository) Add(bookID, userID uint) error {
	entry := models.ToReadEntry{BookID: bookID, UserID: userID}
	if err := r.db.Create(&entry).Error; err != nil {
		if isUniqueViolation(err) {
			return repository.ErrConflict
		}
		return err
	}
	return nil
}

func (r *ToReadRepository) Remove(bookID, userID uint) error {
	result := r.db.Where("book_id = ? AND user_id = ?", bookID, userID).Delete(&models.ToReadEntry{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *ToReadRepository) IsToRead(bookID, userID uint) (bool, error) {
	var count int64
	err := r.db.Model(&models.ToReadEntry{}).
		Where("book_id = ? AND user_id = ?", bookID, userID).Count(&count).Error
	return count > 0, err
}

func (r *ToReadRepository) HasToReadBatch(userID uint, bookIDs []uint) (map[uint]bool, error) {
	if userID == 0 || len(bookIDs) == 0 {
		return map[uint]bool{}, nil
	}
	var entries []models.ToReadEntry
	err := r.db.Where("user_id = ? AND book_id IN ?", userID, bookIDs).Find(&entries).Error
	if err != nil {
		return nil, err
	}
	out := make(map[uint]bool, len(entries))
	for _, entry := range entries {
		out[entry.BookID] = true
	}
	return out, nil
}

func (r *ToReadRepository) ListByUserPaginated(userID uint, page, pageSize int) (*repository.PaginatedResult[models.Book], error) {
	base := r.db.Table("to_read_entries").
		Joins("JOIN books ON books.id = to_read_entries.book_id").
		Where("to_read_entries.user_id = ?", userID)

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, err
	}

	var books []models.Book
	offset := (page - 1) * pageSize
	err := base.Select("books.*").
		Order("to_read_entries.created_at DESC").
		Offset(offset).Limit(pageSize).
		Scan(&books).Error
	if err != nil {
		return nil, err
	}

	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
	return &repository.PaginatedResult[models.Book]{
		Items: books, Total: total, Page: page, PageSize: pageSize, TotalPages: totalPages,
	}, nil
}
