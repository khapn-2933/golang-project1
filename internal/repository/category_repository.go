package repository

import (
	"github.com/kha/foods-drinks/internal/models"
	"gorm.io/gorm"
)

// CategoryRepository handles category database operations
type CategoryRepository struct {
	db *gorm.DB
}

// NewCategoryRepository creates a new CategoryRepository
func NewCategoryRepository(db *gorm.DB) *CategoryRepository {
	return &CategoryRepository{db: db}
}

// Create creates a new category
func (r *CategoryRepository) Create(category *models.Category) error {
	return r.db.Create(category).Error
}

// FindByID finds a category by ID (excluding soft-deleted)
func (r *CategoryRepository) FindByID(id uint) (*models.Category, error) {
	var category models.Category
	if err := r.db.First(&category, id).Error; err != nil {
		return nil, err
	}
	return &category, nil
}

// FindBySlug finds a category by slug
func (r *CategoryRepository) FindBySlug(slug string) (*models.Category, error) {
	var category models.Category
	if err := r.db.Where("slug = ?", slug).First(&category).Error; err != nil {
		return nil, err
	}
	return &category, nil
}

// ExistsBySlug checks if a category with the given slug exists (optionally excluding an ID)
func (r *CategoryRepository) ExistsBySlug(slug string, excludeID ...uint) (bool, error) {
	var count int64
	query := r.db.Model(&models.Category{}).Where("slug = ?", slug)
	if len(excludeID) > 0 && excludeID[0] > 0 {
		query = query.Where("id != ?", excludeID[0])
	}
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// Update updates a category
func (r *CategoryRepository) Update(category *models.Category) error {
	return r.db.Save(category).Error
}

// Delete soft deletes a category
func (r *CategoryRepository) Delete(id uint) error {
	return r.db.Delete(&models.Category{}, id).Error
}

// CategoryListParams holds parameters for listing categories
type CategoryListParams struct {
	Offset  int
	Limit   int
	Status  string
	Search  string
	SortBy  string
	SortDir string
}

// List returns a paginated list of categories with optional filters
func (r *CategoryRepository) List(params CategoryListParams) ([]models.Category, int64, error) {
	var categories []models.Category
	var total int64

	query := r.db.Model(&models.Category{})

	// Filter by status
	if params.Status != "" {
		query = query.Where("status = ?", params.Status)
	}

	// Search by name
	if params.Search != "" {
		query = query.Where("name LIKE ?", "%"+params.Search+"%")
	}

	// Get total count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Sort
	sortBy := "sort_order"
	if params.SortBy != "" {
		sortBy = params.SortBy
	}
	sortDir := "asc"
	if params.SortDir != "" {
		sortDir = params.SortDir
	}
	query = query.Order(sortBy + " " + sortDir)

	// Pagination
	if err := query.Offset(params.Offset).Limit(params.Limit).Find(&categories).Error; err != nil {
		return nil, 0, err
	}

	return categories, total, nil
}
