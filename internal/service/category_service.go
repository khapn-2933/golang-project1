package service

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/kha/foods-drinks/internal/dto"
	"github.com/kha/foods-drinks/internal/models"
	"github.com/kha/foods-drinks/internal/repository"
	"gorm.io/gorm"
)

var (
	ErrCategoryNotFound  = errors.New("category not found")
	ErrSlugAlreadyExists = errors.New("slug already exists")
)

// CategoryService handles category business logic
type CategoryService struct {
	categoryRepo *repository.CategoryRepository
}

// NewCategoryService creates a new CategoryService
func NewCategoryService(categoryRepo *repository.CategoryRepository) *CategoryService {
	return &CategoryService{
		categoryRepo: categoryRepo,
	}
}

// Create creates a new category
func (s *CategoryService) Create(req *dto.CreateCategoryRequest) (*dto.CategoryResponse, error) {
	// Generate or validate slug
	slug := s.generateSlug(req.Name)
	if req.Slug != nil && strings.TrimSpace(*req.Slug) != "" {
		slug = s.generateSlug(*req.Slug)
	}

	// Ensure slug uniqueness
	slug, err := s.ensureUniqueSlug(slug, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to generate unique slug: %w", err)
	}

	category := &models.Category{
		Name:   strings.TrimSpace(req.Name),
		Slug:   slug,
		Status: models.CategoryStatusActive,
	}

	if req.Description != nil {
		trimmed := strings.TrimSpace(*req.Description)
		if trimmed != "" {
			category.Description = &trimmed
		}
	}
	if req.ImageURL != nil {
		trimmed := strings.TrimSpace(*req.ImageURL)
		if trimmed != "" {
			category.ImageURL = &trimmed
		}
	}
	if req.SortOrder != nil {
		category.SortOrder = *req.SortOrder
	}
	if req.Status != nil {
		category.Status = *req.Status
	}

	if err := s.categoryRepo.Create(category); err != nil {
		return nil, fmt.Errorf("failed to create category: %w", err)
	}

	return s.toCategoryResponse(category), nil
}

// GetByID retrieves a category by ID
func (s *CategoryService) GetByID(id uint) (*dto.CategoryResponse, error) {
	category, err := s.categoryRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCategoryNotFound
		}
		return nil, fmt.Errorf("failed to find category: %w", err)
	}
	return s.toCategoryResponse(category), nil
}

// Update updates an existing category
func (s *CategoryService) Update(id uint, req *dto.UpdateCategoryRequest) (*dto.CategoryResponse, error) {
	category, err := s.categoryRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCategoryNotFound
		}
		return nil, fmt.Errorf("failed to find category: %w", err)
	}

	// Update name
	if req.Name != nil {
		category.Name = strings.TrimSpace(*req.Name)
	}

	// Update slug
	if req.Slug != nil && strings.TrimSpace(*req.Slug) != "" {
		newSlug := s.generateSlug(*req.Slug)
		if newSlug != category.Slug {
			// Check uniqueness excluding current category
			exists, err := s.categoryRepo.ExistsBySlug(newSlug, id)
			if err != nil {
				return nil, fmt.Errorf("failed to check slug: %w", err)
			}
			if exists {
				return nil, ErrSlugAlreadyExists
			}
			category.Slug = newSlug
		}
	}

	// Update description
	if req.Description != nil {
		trimmed := strings.TrimSpace(*req.Description)
		if trimmed == "" {
			category.Description = nil
		} else {
			category.Description = &trimmed
		}
	}

	// Update image URL
	if req.ImageURL != nil {
		trimmed := strings.TrimSpace(*req.ImageURL)
		if trimmed == "" {
			category.ImageURL = nil
		} else {
			category.ImageURL = &trimmed
		}
	}

	// Update sort order
	if req.SortOrder != nil {
		category.SortOrder = *req.SortOrder
	}

	// Update status
	if req.Status != nil {
		category.Status = *req.Status
	}

	if err := s.categoryRepo.Update(category); err != nil {
		return nil, fmt.Errorf("failed to update category: %w", err)
	}

	return s.toCategoryResponse(category), nil
}

// Delete soft deletes a category
func (s *CategoryService) Delete(id uint) error {
	// Check if category exists
	_, err := s.categoryRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCategoryNotFound
		}
		return fmt.Errorf("failed to find category: %w", err)
	}

	if err := s.categoryRepo.Delete(id); err != nil {
		return fmt.Errorf("failed to delete category: %w", err)
	}

	return nil
}

// List returns a paginated list of categories
func (s *CategoryService) List(req *dto.CategoryListRequest) (*dto.PaginatedResponse, error) {
	offset := (req.Page - 1) * req.PageSize

	params := repository.CategoryListParams{
		Offset:  offset,
		Limit:   req.PageSize,
		Status:  req.Status,
		Search:  req.Search,
		SortBy:  req.SortBy,
		SortDir: req.SortDir,
	}

	categories, total, err := s.categoryRepo.List(params)
	if err != nil {
		return nil, fmt.Errorf("failed to list categories: %w", err)
	}

	items := make([]dto.CategoryResponse, len(categories))
	for i, cat := range categories {
		items[i] = *s.toCategoryResponse(&cat)
	}

	totalPages := int(math.Ceil(float64(total) / float64(req.PageSize)))

	return &dto.PaginatedResponse{
		Items:      items,
		Total:      total,
		Page:       req.Page,
		PageSize:   req.PageSize,
		TotalPages: totalPages,
	}, nil
}

// generateSlug converts a string to a URL-friendly slug
func (s *CategoryService) generateSlug(input string) string {
	slug := strings.ToLower(strings.TrimSpace(input))

	// Replace spaces and underscores with hyphens
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = strings.ReplaceAll(slug, "_", "-")

	// Remove non-alphanumeric characters except hyphens
	reg := regexp.MustCompile(`[^a-z0-9\-]+`)
	slug = reg.ReplaceAllString(slug, "")

	// Replace multiple hyphens with single hyphen
	reg = regexp.MustCompile(`-{2,}`)
	slug = reg.ReplaceAllString(slug, "-")

	// Trim hyphens from start and end
	slug = strings.Trim(slug, "-")

	return slug
}

// ensureUniqueSlug generates a unique slug by appending a suffix if needed
func (s *CategoryService) ensureUniqueSlug(slug string, excludeID uint) (string, error) {
	exists, err := s.categoryRepo.ExistsBySlug(slug, excludeID)
	if err != nil {
		return "", err
	}
	if !exists {
		return slug, nil
	}

	// Append numeric suffix to make it unique
	for i := 2; i <= 100; i++ {
		candidate := fmt.Sprintf("%s-%d", slug, i)
		exists, err := s.categoryRepo.ExistsBySlug(candidate, excludeID)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
	}

	return "", errors.New("could not generate unique slug")
}

// toCategoryResponse converts a Category model to CategoryResponse DTO
func (s *CategoryService) toCategoryResponse(category *models.Category) *dto.CategoryResponse {
	return &dto.CategoryResponse{
		ID:          category.ID,
		Name:        category.Name,
		Slug:        category.Slug,
		Description: category.Description,
		ImageURL:    category.ImageURL,
		SortOrder:   category.SortOrder,
		Status:      category.Status,
		CreatedAt:   category.CreatedAt,
		UpdatedAt:   category.UpdatedAt,
	}
}
