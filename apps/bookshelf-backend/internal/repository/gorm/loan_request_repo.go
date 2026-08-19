package gorm

import (
	"errors"

	"gorm.io/gorm"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

// LoanRequestRepository is the GORM implementation of repository.LoanRequestRepository.
type LoanRequestRepository struct {
	db *gorm.DB
}

// NewLoanRequestRepository creates a new LoanRequestRepository.
func NewLoanRequestRepository(db *gorm.DB) *LoanRequestRepository {
	return &LoanRequestRepository{db: db}
}

func (r *LoanRequestRepository) Create(lr *models.LoanRequest) error {
	return r.db.Create(lr).Error
}

// CreateAndMarkRequested atomically inserts a loan request and sets the copy
// status to "requested", preventing double-booking via a database transaction.
func (r *LoanRequestRepository) CreateAndMarkRequested(lr *models.LoanRequest) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Re-check availability inside the transaction to close the TOCTOU window.
		var bookCopy models.Copy
		if err := tx.First(&bookCopy, lr.CopyID).Error; err != nil {
			return err
		}
		if bookCopy.Status != "available" {
			return repository.ErrConflict
		}
		if err := tx.Create(lr).Error; err != nil {
			return err
		}
		return tx.Model(&models.Copy{}).Where("id = ?", lr.CopyID).Update("status", "requested").Error
	})
}

func (r *LoanRequestRepository) GetByID(id uint) (*models.LoanRequest, error) {
	var lr models.LoanRequest
	if err := r.db.First(&lr, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &lr, nil
}

func (r *LoanRequestRepository) GetByIDWithCopyAndBorrower(id uint) (*models.LoanRequest, error) {
	var lr models.LoanRequest
	if err := r.db.Preload("Copy").Preload("Borrower").First(&lr, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &lr, nil
}

func (r *LoanRequestRepository) GetByIDWithFullAssociations(id uint) (*models.LoanRequest, error) {
	var lr models.LoanRequest
	if err := r.db.Preload("Copy.Book").Preload("Copy.Owner").Preload("Borrower").First(&lr, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &lr, nil
}

func (r *LoanRequestRepository) GetByIDWithCopyOwnerAndBorrower(id uint) (*models.LoanRequest, error) {
	var lr models.LoanRequest
	if err := r.db.Preload("Copy.Owner").Preload("Borrower").First(&lr, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &lr, nil
}

func (r *LoanRequestRepository) ListByCopyID(copyID uint) ([]models.LoanRequest, error) {
	var requests []models.LoanRequest
	err := r.db.Preload("Copy.Book").Preload("Copy.Owner").Preload("Borrower").
		Where("copy_id = ?", copyID).
		Order("requested_at DESC").
		Find(&requests).Error
	return requests, err
}

func (r *LoanRequestRepository) ListByBorrowerID(borrowerID uint) ([]models.LoanRequest, error) {
	var requests []models.LoanRequest
	err := r.db.Preload("Copy.Book").Preload("Copy.Owner").Preload("Borrower").
		Where("borrower_id = ?", borrowerID).
		Order("requested_at DESC").
		Find(&requests).Error
	return requests, err
}

func (r *LoanRequestRepository) ListByBorrowerIDPaginated(borrowerID uint, statuses []string, page, pageSize int) (*repository.PaginatedResult[models.LoanRequest], error) {
	countQuery := r.db.Model(&models.LoanRequest{}).Where("borrower_id = ?", borrowerID)
	selectQuery := r.db.Preload("Copy.Book").Preload("Copy.Owner").Preload("Borrower").
		Where("borrower_id = ?", borrowerID)
	if len(statuses) > 0 {
		countQuery = countQuery.Where("status IN ?", statuses)
		selectQuery = selectQuery.Where("status IN ?", statuses)
	}

	var total int64
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, err
	}
	var requests []models.LoanRequest
	offset := (page - 1) * pageSize
	if err := selectQuery.Order("requested_at DESC").Offset(offset).Limit(pageSize).Find(&requests).Error; err != nil {
		return nil, err
	}
	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
	return &repository.PaginatedResult[models.LoanRequest]{
		Items: requests, Total: total, Page: page, PageSize: pageSize, TotalPages: totalPages,
	}, nil
}

// ListActiveByBorrowerID returns borrowerID's accepted (currently-held) loan
// requests, due-soonest first, with NULL expected_return_date sorted last.
func (r *LoanRequestRepository) ListActiveByBorrowerID(borrowerID uint) ([]models.LoanRequest, error) {
	var requests []models.LoanRequest
	err := r.db.Preload("Copy.Book").Preload("Copy.Owner").Preload("Borrower").
		Where("borrower_id = ? AND status = ?", borrowerID, "accepted").
		Order("expected_return_date IS NULL, expected_return_date ASC, requested_at ASC").
		Find(&requests).Error
	return requests, err
}

func (r *LoanRequestRepository) Save(lr *models.LoanRequest) error {
	return r.db.Save(lr).Error
}

// RejectCompetingAndUpdateCopy atomically rejects all pending requests for
// copyID other than acceptedLoanID, creates rejection notifications for their
// borrowers, and sets the copy status to "loaned".
func (r *LoanRequestRepository) RejectCompetingAndUpdateCopy(copyID, acceptedLoanID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var others []models.LoanRequest
		if err := tx.Where("copy_id = ? AND id != ? AND status = ?", copyID, acceptedLoanID, "pending").Find(&others).Error; err != nil {
			return err
		}
		for _, other := range others {
			other.Status = "rejected"
			if err := tx.Save(&other).Error; err != nil {
				return err
			}
			n := models.Notification{
				RecipientID:   other.BorrowerID,
				Type:          "request_rejected",
				LoanRequestID: &other.ID,
			}
			if err := tx.Create(&n).Error; err != nil {
				return err
			}
		}
		return tx.Model(&models.Copy{}).Where("id = ?", copyID).Update("status", "loaned").Error
	})
}

func (r *LoanRequestRepository) CountPendingForCopyExcluding(copyID, excludeID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.LoanRequest{}).
		Where("copy_id = ? AND id != ? AND status = ?", copyID, excludeID, "pending").
		Count(&count).Error
	return count, err
}

func (r *LoanRequestRepository) CountActiveLoansByBorrower(borrowerID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.LoanRequest{}).
		Where("borrower_id = ? AND status IN ?", borrowerID, []string{"pending", "accepted", "loaned"}).
		Count(&count).Error
	return count, err
}
