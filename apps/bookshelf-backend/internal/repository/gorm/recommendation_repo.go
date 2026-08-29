package gorm

import (
	"errors"

	"gorm.io/gorm"

	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/models"
	"github.com/tanjd/core-repository/apps/bookshelf-backend/internal/repository"
)

// RecommendationRepository is the GORM implementation of
// repository.RecommendationRepository.
type RecommendationRepository struct {
	db *gorm.DB
}

// NewRecommendationRepository creates a new RecommendationRepository.
func NewRecommendationRepository(db *gorm.DB) *RecommendationRepository {
	return &RecommendationRepository{db: db}
}

func (r *RecommendationRepository) Create(bookID, recommenderID uint) error {
	rec := models.Recommendation{BookID: bookID, RecommenderID: recommenderID}
	if err := r.db.Create(&rec).Error; err != nil {
		if isUniqueViolation(err) {
			return repository.ErrConflict
		}
		return err
	}
	return nil
}

func (r *RecommendationRepository) Delete(bookID, recommenderID uint) error {
	return r.db.Where("book_id = ? AND recommender_id = ?", bookID, recommenderID).
		Delete(&models.Recommendation{}).Error
}

func (r *RecommendationRepository) FindByBookAndRecommender(bookID, recommenderID uint) (*models.Recommendation, error) {
	var rec models.Recommendation
	err := r.db.Where("book_id = ? AND recommender_id = ?", bookID, recommenderID).First(&rec).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	return &rec, nil
}

func (r *RecommendationRepository) ListByBookID(bookID uint) ([]models.Recommendation, error) {
	var recs []models.Recommendation
	err := r.db.Preload("Recommender").
		Where("book_id = ?", bookID).
		Order("created_at DESC").
		Find(&recs).Error
	return recs, err
}

func (r *RecommendationRepository) CountByBookBatch(bookIDs []uint) (map[uint]int64, error) {
	if len(bookIDs) == 0 {
		return map[uint]int64{}, nil
	}
	type row struct {
		BookID uint
		Count  int64
	}
	var rows []row
	err := r.db.Model(&models.Recommendation{}).
		Select("book_id, count(*) as count").
		Where("book_id IN ?", bookIDs).
		Group("book_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[uint]int64, len(rows))
	for _, row := range rows {
		out[row.BookID] = row.Count
	}
	return out, nil
}

func (r *RecommendationRepository) HasRecommendedBatch(userID uint, bookIDs []uint) (map[uint]bool, error) {
	if userID == 0 || len(bookIDs) == 0 {
		return map[uint]bool{}, nil
	}
	var recs []models.Recommendation
	err := r.db.Where("recommender_id = ? AND book_id IN ?", userID, bookIDs).Find(&recs).Error
	if err != nil {
		return nil, err
	}
	out := make(map[uint]bool, len(recs))
	for _, rec := range recs {
		out[rec.BookID] = true
	}
	return out, nil
}

func (r *RecommendationRepository) DeleteByBookID(bookID uint) error {
	return r.db.Where("book_id = ?", bookID).Delete(&models.Recommendation{}).Error
}

func (r *RecommendationRepository) DeleteByRecommenderID(recommenderID uint) error {
	return r.db.Where("recommender_id = ?", recommenderID).Delete(&models.Recommendation{}).Error
}

// ListTopBooks returns the top-recommended books ordered by recommendation
// count descending, title ascending for ties. Books with zero recommendations
// are excluded. Up to limit books are returned.
func (r *RecommendationRepository) ListTopBooks(limit int) ([]repository.TopRecommendedBook, error) {
	type row struct {
		models.Book
		Count int64
	}
	var rows []row
	err := r.db.Table("recommendations").
		Select("books.*, count(*) as count").
		Joins("JOIN books ON books.id = recommendations.book_id").
		Group("recommendations.book_id").
		Having("count(*) > 0").
		Order("count DESC, books.title ASC").
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make([]repository.TopRecommendedBook, len(rows))
	for i, row := range rows {
		result[i] = repository.TopRecommendedBook{Book: row.Book, Count: row.Count}
	}
	return result, nil
}
