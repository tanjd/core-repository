package gorm

import (
	"errors"
	"strings"

	"gorm.io/gorm"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

// WishlistRequestRepository is the GORM implementation of
// repository.WishlistRequestRepository.
type WishlistRequestRepository struct {
	db *gorm.DB
}

// NewWishlistRequestRepository creates a new WishlistRequestRepository.
func NewWishlistRequestRepository(db *gorm.DB) *WishlistRequestRepository {
	return &WishlistRequestRepository{db: db}
}

func (r *WishlistRequestRepository) Create(req *models.WishlistRequest) error {
	return r.db.Create(req).Error
}

func (r *WishlistRequestRepository) GetByID(id uint) (*models.WishlistRequest, error) {
	var req models.WishlistRequest
	if err := r.db.Preload("Requester").Preload("FulfilledBook").First(&req, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &req, nil
}

func (r *WishlistRequestRepository) Save(req *models.WishlistRequest) error {
	return r.db.Save(req).Error
}

func (r *WishlistRequestRepository) buildOpenQuery(search string) *gorm.DB {
	tx := r.db.Model(&models.WishlistRequest{}).Where("status = ?", "open")
	if search != "" {
		like := "%" + search + "%"
		tx = tx.Where("title LIKE ? OR author LIKE ?", like, like)
	}
	return tx
}

func (r *WishlistRequestRepository) ListOpenPaginated(search string, page, pageSize int) (*repository.PaginatedResult[models.WishlistRequest], error) {
	var total int64
	if err := r.buildOpenQuery(search).Count(&total).Error; err != nil {
		return nil, err
	}
	var items []models.WishlistRequest
	offset := (page - 1) * pageSize
	if err := r.buildOpenQuery(search).Preload("Requester").
		Order("created_at desc").Offset(offset).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, err
	}
	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
	return &repository.PaginatedResult[models.WishlistRequest]{
		Items: items, Total: total, Page: page, PageSize: pageSize, TotalPages: totalPages,
	}, nil
}

func (r *WishlistRequestRepository) ListByRequesterID(requesterID uint) ([]models.WishlistRequest, error) {
	var items []models.WishlistRequest
	err := r.db.Preload("FulfilledBook").
		Where("requester_id = ?", requesterID).
		Order("created_at desc").
		Find(&items).Error
	return items, err
}

func (r *WishlistRequestRepository) FindOpenByOLKey(olKey string) ([]models.WishlistRequest, error) {
	var items []models.WishlistRequest
	err := r.db.Where("status = ? AND ol_key = ? AND ol_key != ''", "open", olKey).Find(&items).Error
	return items, err
}

func (r *WishlistRequestRepository) FindOpenByGoogleBooksID(googleBooksID string) ([]models.WishlistRequest, error) {
	var items []models.WishlistRequest
	err := r.db.Where("status = ? AND google_books_id = ? AND google_books_id != ''", "open", googleBooksID).Find(&items).Error
	return items, err
}

func (r *WishlistRequestRepository) FindOpenMatch(isbn, olKey, googleBooksID string) (*models.WishlistRequest, error) {
	var conds []string
	var args []any
	if isbn != "" {
		conds = append(conds, "isbn = ?")
		args = append(args, isbn)
	}
	if olKey != "" {
		conds = append(conds, "ol_key = ?")
		args = append(args, olKey)
	}
	if googleBooksID != "" {
		conds = append(conds, "google_books_id = ?")
		args = append(args, googleBooksID)
	}
	if len(conds) == 0 {
		return nil, nil
	}

	var req models.WishlistRequest
	err := r.db.Where("status = ?", "open").
		Where(strings.Join(conds, " OR "), args...).
		Preload("Requester").
		Order("created_at asc").
		First(&req).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &req, nil
}

// ClearFulfilledBookID nulls FulfilledBookID on every request pointing at
// bookID, so deleting that Book leaves no dangling reference.
func (r *WishlistRequestRepository) ClearFulfilledBookID(bookID uint) error {
	return r.db.Model(&models.WishlistRequest{}).
		Where("fulfilled_book_id = ?", bookID).
		Update("fulfilled_book_id", nil).Error
}
