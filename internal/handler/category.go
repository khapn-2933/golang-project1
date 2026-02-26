package handler

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/kha/foods-drinks/internal/dto"
	"github.com/kha/foods-drinks/internal/service"
)

// CategoryHandler handles category HTTP requests
type CategoryHandler struct {
	categoryService *service.CategoryService
}

// NewCategoryHandler creates a new CategoryHandler
func NewCategoryHandler(categoryService *service.CategoryService) *CategoryHandler {
	return &CategoryHandler{
		categoryService: categoryService,
	}
}

// Create godoc
// @Summary Create a new category
// @Description Create a new category (admin only)
// @Tags categories
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.CreateCategoryRequest true "Create category request"
// @Success 201 {object} dto.CategoryResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 403 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/v1/admin/categories [post]
func (h *CategoryHandler) Create(c *gin.Context) {
	var req dto.CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.handleValidationError(c, err)
		return
	}

	resp, err := h.categoryService.Create(&req)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// GetByID godoc
// @Summary Get a category by ID
// @Description Get a category by its ID (admin only)
// @Tags categories
// @Produce json
// @Security BearerAuth
// @Param id path int true "Category ID"
// @Success 200 {object} dto.CategoryResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/v1/admin/categories/{id} [get]
func (h *CategoryHandler) GetByID(c *gin.Context) {
	id, err := h.parseID(c)
	if err != nil {
		return
	}

	resp, err := h.categoryService.GetByID(id)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// Update godoc
// @Summary Update a category
// @Description Update an existing category (admin only)
// @Tags categories
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Category ID"
// @Param request body dto.UpdateCategoryRequest true "Update category request"
// @Success 200 {object} dto.CategoryResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 409 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/v1/admin/categories/{id} [put]
func (h *CategoryHandler) Update(c *gin.Context) {
	id, err := h.parseID(c)
	if err != nil {
		return
	}

	var req dto.UpdateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.handleValidationError(c, err)
		return
	}

	// Ensure at least one field is being updated
	if req.Name == nil && req.Slug == nil && req.Description == nil &&
		req.ImageURL == nil && req.SortOrder == nil && req.Status == nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "validation_error",
			Message: "At least one field must be provided for update",
		})
		return
	}

	resp, err := h.categoryService.Update(id, &req)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// Delete godoc
// @Summary Delete a category
// @Description Soft delete a category (admin only)
// @Tags categories
// @Produce json
// @Security BearerAuth
// @Param id path int true "Category ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/v1/admin/categories/{id} [delete]
func (h *CategoryHandler) Delete(c *gin.Context) {
	id, err := h.parseID(c)
	if err != nil {
		return
	}

	if err := h.categoryService.Delete(id); err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Category deleted successfully"})
}

// List godoc
// @Summary List categories
// @Description List categories with pagination and filters (admin only)
// @Tags categories
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(20)
// @Param status query string false "Filter by status" Enums(active, inactive)
// @Param search query string false "Search by name"
// @Param sort_by query string false "Sort field" Enums(id, name, sort_order, created_at) default(sort_order)
// @Param sort_dir query string false "Sort direction" Enums(asc, desc) default(asc)
// @Success 200 {object} dto.PaginatedResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/v1/admin/categories [get]
func (h *CategoryHandler) List(c *gin.Context) {
	var req dto.CategoryListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		h.handleValidationError(c, err)
		return
	}

	// Set defaults if not provided
	if req.Page == 0 {
		req.Page = 1
	}
	if req.PageSize == 0 {
		req.PageSize = 20
	}
	if req.SortBy == "" {
		req.SortBy = "sort_order"
	}
	if req.SortDir == "" {
		req.SortDir = "asc"
	}

	resp, err := h.categoryService.List(&req)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// parseID extracts and validates the ID path parameter
func (h *CategoryHandler) parseID(c *gin.Context) (uint, error) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "invalid_id",
			Message: "Invalid category ID",
		})
		return 0, errors.New("invalid id")
	}
	return uint(id), nil
}

// handleValidationError handles validation errors
func (h *CategoryHandler) handleValidationError(c *gin.Context, err error) {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		details := make(map[string]string)
		for _, fe := range ve {
			field := strings.ToLower(fe.Field())
			switch fe.Tag() {
			case "required":
				details[field] = field + " is required"
			case "min":
				details[field] = field + " must be at least " + fe.Param() + " characters"
			case "max":
				details[field] = field + " must be at most " + fe.Param() + " characters"
			case "url":
				details[field] = field + " must be a valid URL"
			case "oneof":
				details[field] = field + " must be one of: " + fe.Param()
			default:
				details[field] = field + " is invalid"
			}
		}
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "validation_error",
			Message: "Validation failed",
			Details: details,
		})
		return
	}

	c.JSON(http.StatusBadRequest, dto.ErrorResponse{
		Error:   "bad_request",
		Message: "Invalid request body",
	})
}

// handleServiceError handles service layer errors
func (h *CategoryHandler) handleServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrCategoryNotFound):
		c.JSON(http.StatusNotFound, dto.ErrorResponse{
			Error:   "category_not_found",
			Message: "Category not found",
		})
	case errors.Is(err, service.ErrSlugAlreadyExists):
		c.JSON(http.StatusConflict, dto.ErrorResponse{
			Error:   "slug_exists",
			Message: "A category with this slug already exists",
		})
	default:
		log.Printf("Category service error: %v", err)
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error:   "internal_error",
			Message: "An unexpected error occurred",
		})
	}
}
