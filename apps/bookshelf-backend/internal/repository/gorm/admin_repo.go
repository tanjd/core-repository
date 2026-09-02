package gorm

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

// dashboardRankingLimit caps how many rows are returned for each ranked list
// (most-borrowed books, active lenders) on the admin dashboard.
const dashboardRankingLimit = 5

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

func (r *AdminRepository) ListUsersPaginated(page, pageSize int, filter repository.UserListFilter) (*repository.PaginatedResult[models.User], error) {
	query := r.db.Model(&models.User{})
	// SQLite's LIKE is case-insensitive for ASCII by default (no COLLATE
	// NOCASE needed), matching what an admin typing a name/email expects.
	if s := strings.TrimSpace(filter.Search); s != "" {
		like := "%" + s + "%"
		query = query.Where("name LIKE ? OR email LIKE ?", like, like)
	}
	if filter.Role != "" {
		query = query.Where("role = ?", filter.Role)
	}
	switch filter.Status {
	case "verified":
		query = query.Where("verified = ?", true)
	case "unverified":
		query = query.Where("verified = ?", false)
	case "pending_approval":
		query = query.Where("pending_approval = ?", true)
	case "suspended":
		query = query.Where("suspended = ?", true)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var users []models.User
	offset := (page - 1) * pageSize
	switch filter.Sort {
	case "newest":
		query = query.Order("created_at desc")
	case "name":
		query = query.Order("name asc")
	case "email":
		query = query.Order("email asc")
	case "role":
		query = query.Order("role asc, name asc")
	default:
		query = query.Order("created_at asc")
	}
	// Preload InvitedBy so the admin Users table can show an "Invited by"
	// column without a second request per row — see docs/invite-code-spec.md.
	if err := query.Preload("InvitedBy").Offset(offset).Limit(pageSize).Find(&users).Error; err != nil {
		return nil, err
	}
	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
	return &repository.PaginatedResult[models.User]{
		Items: users, Total: total, Page: page, PageSize: pageSize, TotalPages: totalPages,
	}, nil
}

func (r *AdminRepository) ListByRole(role string) ([]models.User, error) {
	var users []models.User
	if err := r.db.Where("role = ?", role).Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
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

func (r *AdminRepository) GetDashboardStats() (*repository.DashboardStats, error) {
	var stats repository.DashboardStats

	if err := r.db.Model(&models.Book{}).Count(&stats.TotalBooks).Error; err != nil {
		return nil, err
	}
	if err := r.db.Model(&models.Copy{}).Count(&stats.TotalCopies).Error; err != nil {
		return nil, err
	}
	if err := r.db.Model(&models.Copy{}).Where("status = ?", "available").Count(&stats.AvailableCopies).Error; err != nil {
		return nil, err
	}
	if err := r.db.Model(&models.Copy{}).Where("status = ?", "loaned").Count(&stats.LoanedCopies).Error; err != nil {
		return nil, err
	}
	if err := r.db.Model(&models.User{}).Count(&stats.TotalUsers).Error; err != nil {
		return nil, err
	}
	if err := r.db.Model(&models.User{}).
		Where("created_at >= ?", time.Now().AddDate(0, 0, -7)).
		Count(&stats.SignupsThisWeek).Error; err != nil {
		return nil, err
	}
	if err := r.db.Model(&models.LoanRequest{}).
		Where("status = ? AND expected_return_date < ?", "accepted", time.Now()).
		Count(&stats.OverdueCount).Error; err != nil {
		return nil, err
	}
	if err := r.db.Model(&models.User{}).
		Where("pending_approval = ?", true).
		Count(&stats.PendingApprovalCount).Error; err != nil {
		return nil, err
	}
	if err := r.db.Model(&models.LoanRequest{}).
		Where("status = ?", "accepted").
		Count(&stats.ActiveLoansCount).Error; err != nil {
		return nil, err
	}
	if err := r.db.Model(&models.LoanRequest{}).
		Where("status = ?", "returned").
		Count(&stats.CompletedLoansCount).Error; err != nil {
		return nil, err
	}

	stats.MostBorrowedBooks = []repository.BookBorrowStat{}
	if err := r.db.Table("loan_requests").
		Select("books.id AS book_id, books.title AS title, books.author AS author, COUNT(*) AS borrow_count").
		Joins("JOIN copies ON copies.id = loan_requests.copy_id").
		Joins("JOIN books ON books.id = copies.book_id").
		Where("loan_requests.status IN ?", []string{"accepted", "returned"}).
		Group("books.id, books.title, books.author").
		Order("borrow_count DESC").
		Limit(dashboardRankingLimit).
		Scan(&stats.MostBorrowedBooks).Error; err != nil {
		return nil, err
	}

	stats.ActiveLenders = []repository.LenderStat{}
	if err := r.db.Table("loan_requests").
		Select("users.id AS user_id, users.name AS name, COUNT(*) AS active_loans").
		Joins("JOIN copies ON copies.id = loan_requests.copy_id").
		Joins("JOIN users ON users.id = copies.owner_id").
		Where("loan_requests.status = ?", "accepted").
		Group("users.id, users.name").
		Order("active_loans DESC").
		Limit(dashboardRankingLimit).
		Scan(&stats.ActiveLenders).Error; err != nil {
		return nil, err
	}

	return &stats, nil
}
