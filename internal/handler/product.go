package handler

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/kha/foods-drinks/internal/dto"
	"github.com/kha/foods-drinks/internal/service"
)

type ProductHandler struct {
	productService *service.ProductService
}

func NewProductHandler(productService *service.ProductService) *ProductHandler {
	return &ProductHandler{productService: productService}
}

// List godoc
// @Summary List products
// @Description Public API list products with filter, sort, search and pagination
// @Tags products
// @Produce json
// @Param page       query int    false "Page"          default(1)
// @Param page_size  query int    false "Page size"     default(20)
// @Param classify   query string false "food or drink"
// @Param category_id query uint  false "Category ID"
// @Param min_price  query number false "Min price"
// @Param max_price  query number false "Max price"
// @Param min_rating query number false "Min rating (0-5)"
// @Param search     query string false "Full-text search"
// @Param sort_by    query string false "price|rating_average|name|created_at" default(created_at)
// @Param sort_dir   query string false "asc|desc"                             default(desc)
// @Success 200 {object} dto.PaginatedResponse
// @Failure 400 {object} dto.ErrorResponse
// @Router /api/v1/products [get]
func (h *ProductHandler) List(c *gin.Context) {
	var req dto.ProductListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "invalid_params",
			Message: "Invalid query parameters: " + err.Error(),
		})
		return
	}

	if req.Page == 0 {
		req.Page = 1
	}
	if req.PageSize == 0 {
		req.PageSize = 20
	}
	if req.SortBy == "" {
		req.SortBy = "created_at"
	}
	if req.SortDir == "" {
		req.SortDir = "desc"
	}
	// Public list chỉ hiện active
	req.Status = "active"

	result, err := h.productService.List(&req)
	if err != nil {
		log.Printf("Product list error: %v", err)
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error:   "internal_error",
			Message: "An unexpected error occurred",
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetBySlug godoc
// @Summary Get product detail
// @Description Public API get product detail by slug
// @Tags products
// @Produce json
// @Param slug path string true "Product slug"
// @Success 200 {object} dto.ProductResponse
// @Failure 404 {object} dto.ErrorResponse
// @Router /api/v1/products/{slug} [get]
func (h *ProductHandler) GetBySlug(c *gin.Context) {
	slug := strings.TrimSpace(c.Param("slug"))
	if slug == "" {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "invalid_slug",
			Message: "Slug is required",
		})
		return
	}

	product, err := h.productService.GetBySlug(slug)
	if err != nil {
		if errors.Is(err, service.ErrProductNotFound) {
			c.JSON(http.StatusNotFound, dto.ErrorResponse{
				Error:   "product_not_found",
				Message: "Product not found",
			})
			return
		}
		log.Printf("Product detail error: %v", err)
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error:   "internal_error",
			Message: "An unexpected error occurred",
		})
		return
	}

	if product.Status != "active" {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{
			Error:   "product_not_found",
			Message: "Product not found",
		})
		return
	}

	c.JSON(http.StatusOK, product)
}
