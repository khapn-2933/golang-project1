package dto

import "time"

// CreateCategoryRequest represents the request body for creating a category
type CreateCategoryRequest struct {
	Name        string  `json:"name" binding:"required,min=2,max=255"`
	Slug        *string `json:"slug" binding:"omitempty,min=2,max=255"`
	Description *string `json:"description" binding:"omitempty,max=2000"`
	ImageURL    *string `json:"image_url" binding:"omitempty,url,max=500"`
	SortOrder   *int    `json:"sort_order" binding:"omitempty,min=0"`
	Status      *string `json:"status" binding:"omitempty,oneof=active inactive"`
}

// UpdateCategoryRequest represents the request body for updating a category
type UpdateCategoryRequest struct {
	Name        *string `json:"name" binding:"omitempty,min=2,max=255"`
	Slug        *string `json:"slug" binding:"omitempty,min=2,max=255"`
	Description *string `json:"description" binding:"omitempty,max=2000"`
	ImageURL    *string `json:"image_url" binding:"omitempty,max=500"`
	SortOrder   *int    `json:"sort_order" binding:"omitempty,min=0"`
	Status      *string `json:"status" binding:"omitempty,oneof=active inactive"`
}

// CategoryResponse represents a category in API responses
type CategoryResponse struct {
	ID          uint      `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description *string   `json:"description,omitempty"`
	ImageURL    *string   `json:"image_url,omitempty"`
	SortOrder   int       `json:"sort_order"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CategoryListRequest represents query parameters for listing categories
type CategoryListRequest struct {
	Page     int    `form:"page,default=1" binding:"min=1"`
	PageSize int    `form:"page_size,default=20" binding:"min=1,max=100"`
	Status   string `form:"status" binding:"omitempty,oneof=active inactive"`
	Search   string `form:"search" binding:"omitempty,max=255"`
	SortBy   string `form:"sort_by,default=sort_order" binding:"omitempty,oneof=id name sort_order created_at"`
	SortDir  string `form:"sort_dir,default=asc" binding:"omitempty,oneof=asc desc"`
}

// PaginatedResponse represents a paginated list response
type PaginatedResponse struct {
	Items      interface{} `json:"items"`
	Total      int64       `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalPages int         `json:"total_pages"`
}
