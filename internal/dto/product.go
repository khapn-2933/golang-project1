package dto

import "time"

type ProductImageResponse struct {
	ID        uint    `json:"id"`
	ImageURL  string  `json:"image_url"`
	AltText   *string `json:"alt_text,omitempty"`
	SortOrder int     `json:"sort_order"`
	IsPrimary bool    `json:"is_primary"`
}

type ProductResponse struct {
	ID            uint                   `json:"id"`
	CategoryID    uint                   `json:"category_id"`
	CategoryName  string                 `json:"category_name,omitempty"`
	Name          string                 `json:"name"`
	Slug          string                 `json:"slug"`
	Description   *string                `json:"description,omitempty"`
	Classify      string                 `json:"classify"`
	Price         float64                `json:"price"`
	Stock         int                    `json:"stock"`
	RatingAverage float64                `json:"rating_average"`
	RatingCount   int                    `json:"rating_count"`
	Status        string                 `json:"status"`
	Images        []ProductImageResponse `json:"images,omitempty"`
	PrimaryImage  *ProductImageResponse  `json:"primary_image,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
}

type CreateProductRequest struct {
	CategoryID  uint    `form:"category_id" json:"category_id" binding:"required"`
	Name        string  `form:"name"        json:"name"        binding:"required,min=2,max=255"`
	Slug        string  `form:"slug"        json:"slug"        binding:"omitempty,max=255"`
	Description string  `form:"description" json:"description" binding:"omitempty,max=5000"`
	Classify    string  `form:"classify"    json:"classify"    binding:"required,oneof=food drink"`
	Price       float64 `form:"price"       json:"price"       binding:"required,min=0"`
	Stock       int     `form:"stock"       json:"stock"       binding:"min=0"`
	Status      string  `form:"status"      json:"status"      binding:"omitempty,oneof=active inactive out_of_stock"`
}

type UpdateProductRequest struct {
	CategoryID  *uint    `form:"category_id" json:"category_id" binding:"omitempty"`
	Name        *string  `form:"name"        json:"name"        binding:"omitempty,min=2,max=255"`
	Slug        *string  `form:"slug"        json:"slug"        binding:"omitempty,max=255"`
	Description *string  `form:"description" json:"description" binding:"omitempty,max=5000"`
	Classify    *string  `form:"classify"    json:"classify"    binding:"omitempty,oneof=food drink"`
	Price       *float64 `form:"price"       json:"price"       binding:"omitempty,min=0"`
	Stock       *int     `form:"stock"       json:"stock"       binding:"omitempty,min=0"`
	Status      *string  `form:"status"      json:"status"      binding:"omitempty,oneof=active inactive out_of_stock"`
}

type ProductListRequest struct {
	Page      int     `form:"page,default=1"        binding:"min=1"`
	PageSize  int     `form:"page_size,default=20"  binding:"min=1,max=100"`
	Classify  string  `form:"classify"              binding:"omitempty,oneof=food drink"`
	Category  uint    `form:"category_id"           binding:"omitempty"`
	MinPrice  float64 `form:"min_price"             binding:"omitempty,min=0"`
	MaxPrice  float64 `form:"max_price"             binding:"omitempty,min=0"`
	MinRating float64 `form:"min_rating"            binding:"omitempty,min=0,max=5"`
	Status    string  `form:"status"                binding:"omitempty,oneof=active inactive out_of_stock"`
	Search    string  `form:"search"                binding:"omitempty,max=255"`
	SortBy    string  `form:"sort_by,default=created_at" binding:"omitempty,oneof=price rating_average name created_at"`
	SortDir   string  `form:"sort_dir,default=desc"      binding:"omitempty,oneof=asc desc"`
}
