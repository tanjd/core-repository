package gorm

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

// BookRepository is the GORM implementation of repository.BookRepository.
type BookRepository struct {
	db *gorm.DB
}

// NewBookRepository creates a new BookRepository.
func NewBookRepository(db *gorm.DB) *BookRepository {
	return &BookRepository{db: db}
}

func (r *BookRepository) CountAll() (int64, error) {
	var count int64
	err := r.db.Model(&models.Book{}).Count(&count).Error
	return count, err
}

func (r *BookRepository) FindByGoogleBooksID(id string) (*models.Book, error) {
	var book models.Book
	if err := r.db.Where("google_books_id = ?", id).First(&book).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &book, nil
}

func (r *BookRepository) FindByOLKey(olKey string) (*models.Book, error) {
	var book models.Book
	if err := r.db.Where("ol_key = ?", olKey).First(&book).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &book, nil
}

// FindByISBN returns the book with the given ISBN, or repository.ErrNotFound.
// Excludes empty-string ISBNs so two books that both lack one don't collide.
func (r *BookRepository) FindByISBN(isbn string) (*models.Book, error) {
	var book models.Book
	if err := r.db.Where("isbn = ? AND isbn != ''", isbn).First(&book).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &book, nil
}

func (r *BookRepository) buildListQuery(search, sort string, availableOnly bool) *gorm.DB {
	tx := r.db.Model(&models.Book{})
	if search != "" {
		like := "%" + search + "%"
		tx = tx.Where("title LIKE ? OR author LIKE ?", like, like)
	}
	// A book with no copies left (e.g. its last copy was just removed by its
	// owner) shouldn't linger in the catalog — same rule ListRecent already
	// applies to the "recently added" shelf.
	if availableOnly {
		tx = tx.Where("EXISTS (SELECT 1 FROM copies WHERE copies.book_id = books.id AND copies.status = 'available')")
	} else {
		tx = tx.Where("EXISTS (SELECT 1 FROM copies WHERE copies.book_id = books.id)")
	}
	switch sort {
	case "author":
		tx = tx.Order("author ASC, title ASC")
	case "newest":
		tx = tx.Order("books.created_at DESC")
	case "relevance":
		// Only meaningful alongside a search term — a prefix match on
		// title or author ranks above a mid-string substring match, so a
		// query like "harry" surfaces "Harry Potter" before "The Harried
		// Reader". Falls back to title order for an empty query, same as
		// the default case.
		if search == "" {
			tx = tx.Order("title ASC")
		} else {
			prefix := search + "%"
			tx = tx.Clauses(clause.OrderBy{
				Expression: clause.Expr{
					SQL:  "CASE WHEN title LIKE ? THEN 0 WHEN author LIKE ? THEN 1 ELSE 2 END, title ASC",
					Vars: []any{prefix, prefix},
				},
			})
		}
	default:
		tx = tx.Order("title ASC")
	}
	return tx
}

func (r *BookRepository) List(search, sort string, availableOnly bool) ([]models.Book, error) {
	var books []models.Book
	if err := r.buildListQuery(search, sort, availableOnly).Find(&books).Error; err != nil {
		return nil, err
	}
	return books, nil
}

func (r *BookRepository) ListPaginated(search, sort string, availableOnly bool, page, pageSize int) (*repository.PaginatedResult[models.Book], error) {
	var total int64
	if err := r.buildListQuery(search, sort, availableOnly).Count(&total).Error; err != nil {
		return nil, err
	}
	var books []models.Book
	offset := (page - 1) * pageSize
	if err := r.buildListQuery(search, sort, availableOnly).Offset(offset).Limit(pageSize).Find(&books).Error; err != nil {
		return nil, err
	}
	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
	return &repository.PaginatedResult[models.Book]{
		Items: books, Total: total, Page: page, PageSize: pageSize, TotalPages: totalPages,
	}, nil
}

func (r *BookRepository) ListRecent(limit int) ([]models.Book, error) {
	var books []models.Book
	err := r.db.Where("EXISTS (SELECT 1 FROM copies WHERE copies.book_id = books.id)").
		Order("books.created_at DESC").
		Limit(limit).
		Find(&books).Error
	return books, err
}

func (r *BookRepository) GetByIDWithCopies(id uint) (*models.Book, error) {
	var book models.Book
	if err := r.db.Preload("Copies.Owner").First(&book, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &book, nil
}

func (r *BookRepository) Create(book *models.Book) error {
	return r.db.Create(book).Error
}

func (r *BookRepository) Save(book *models.Book) error {
	return r.db.Save(book).Error
}

// Delete hard-deletes book — Book has no DeletedAt field, so this is a real DELETE.
func (r *BookRepository) Delete(book *models.Book) error {
	return r.db.Delete(book).Error
}

// CountCopies returns the total number of Copy rows for bookID, with no status filter.
func (r *BookRepository) CountCopies(bookID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.Copy{}).
		Where("book_id = ?", bookID).
		Count(&count).Error
	return count, err
}

func (r *BookRepository) CountAvailableCopies(bookID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.Copy{}).
		Where("book_id = ? AND status = ?", bookID, "available").
		Count(&count).Error
	return count, err
}

func (r *BookRepository) CountAvailableCopiesBatch(bookIDs []uint) (map[uint]int64, error) {
	if len(bookIDs) == 0 {
		return map[uint]int64{}, nil
	}
	type row struct {
		BookID uint
		Count  int64
	}
	var rows []row
	err := r.db.Model(&models.Copy{}).
		Select("book_id, count(*) as count").
		Where("book_id IN ? AND status = ?", bookIDs, "available").
		Group("book_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[uint]int64, len(rows))
	for _, r := range rows {
		out[r.BookID] = r.Count
	}
	return out, nil
}
