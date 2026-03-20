package service

import (
	"errors"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strings"

	"github.com/kha/foods-drinks/internal/dto"
	"github.com/kha/foods-drinks/internal/models"
	"github.com/kha/foods-drinks/internal/repository"
	"gorm.io/gorm"
)

var (
	ErrProductNotFound   = errors.New("product not found")
	ErrProductSlugExists = errors.New("slug already exists")
	ErrProductEmptySlug  = errors.New("slug cannot be empty after generation")
)

var (
	productSlugNonAlnum  = regexp.MustCompile(`[^a-z0-9\-]+`)
	productSlugMultiHyph = regexp.MustCompile(`-{2,}`)
)

type ProductService struct {
	productRepo  *repository.ProductRepository
	categoryRepo *repository.CategoryRepository
	baseURL      string
}

func NewProductService(productRepo *repository.ProductRepository, categoryRepo *repository.CategoryRepository, baseURL string) *ProductService {
	baseURL = strings.TrimSpace(baseURL)
	baseURL = strings.TrimRight(baseURL, "/")
	return &ProductService{productRepo: productRepo, categoryRepo: categoryRepo, baseURL: baseURL}
}

func (s *ProductService) Create(req *dto.CreateProductRequest, imageURLs []string) (*dto.ProductResponse, error) {
	slugSrc := req.Name
	if strings.TrimSpace(req.Slug) != "" {
		slugSrc = req.Slug
	}
	slug := s.generateSlug(slugSrc)
	if slug == "" {
		return nil, ErrProductEmptySlug
	}
	slug, err := s.ensureUniqueSlug(slug, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to generate unique slug: %w", err)
	}

	status := models.ProductStatusActive
	if req.Status != "" {
		status = req.Status
	}

	product := &models.Product{
		CategoryID: req.CategoryID,
		Name:       strings.TrimSpace(req.Name),
		Slug:       slug,
		Classify:   req.Classify,
		Price:      req.Price,
		Stock:      req.Stock,
		Status:     status,
	}
	if strings.TrimSpace(req.Description) != "" {
		d := strings.TrimSpace(req.Description)
		product.Description = &d
	}

	if err := s.productRepo.Create(product); err != nil {
		return nil, fmt.Errorf("failed to create product: %w", err)
	}

	for i, url := range imageURLs {
		url = strings.TrimSpace(url)
		if url == "" {
			continue
		}
		img := &models.ProductImage{
			ProductID: product.ID,
			ImageURL:  url,
			SortOrder: i,
			IsPrimary: i == 0,
		}
		if err := s.productRepo.AddImage(img); err != nil {
			return nil, fmt.Errorf("failed to save image: %w", err)
		}
	}

	return s.GetByID(product.ID)
}

func (s *ProductService) GetByID(id uint) (*dto.ProductResponse, error) {
	p, err := s.productRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProductNotFound
		}
		return nil, fmt.Errorf("failed to find product: %w", err)
	}
	return s.toResponse(p), nil
}

func (s *ProductService) GetBySlug(slug string) (*dto.ProductResponse, error) {
	p, err := s.productRepo.FindBySlug(slug)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProductNotFound
		}
		return nil, fmt.Errorf("failed to find product: %w", err)
	}
	return s.toResponse(p), nil
}

func (s *ProductService) Update(id uint, req *dto.UpdateProductRequest, imageURLs []string, replaceImages bool) (*dto.ProductResponse, error) {
	p, err := s.productRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProductNotFound
		}
		return nil, fmt.Errorf("failed to find product: %w", err)
	}

	if req.CategoryID != nil {
		p.CategoryID = *req.CategoryID
	}
	if req.Name != nil {
		p.Name = strings.TrimSpace(*req.Name)
	}
	if req.Slug != nil && strings.TrimSpace(*req.Slug) != "" {
		newSlug := s.generateSlug(*req.Slug)
		if newSlug == "" {
			return nil, ErrProductEmptySlug
		}
		if newSlug != p.Slug {
			exists, err := s.productRepo.ExistsBySlug(newSlug, id)
			if err != nil {
				return nil, fmt.Errorf("failed to check slug: %w", err)
			}
			if exists {
				return nil, ErrProductSlugExists
			}
			p.Slug = newSlug
		}
	}
	if req.Description != nil {
		d := strings.TrimSpace(*req.Description)
		if d == "" {
			p.Description = nil
		} else {
			p.Description = &d
		}
	}
	if req.Classify != nil {
		p.Classify = *req.Classify
	}
	if req.Price != nil {
		p.Price = *req.Price
	}
	if req.Stock != nil {
		p.Stock = *req.Stock
	}
	if req.Status != nil {
		p.Status = *req.Status
	}

	if err := s.productRepo.Update(p); err != nil {
		return nil, fmt.Errorf("failed to update product: %w", err)
	}

	if replaceImages && len(imageURLs) > 0 {
		if err := s.productRepo.DeleteImagesByProductID(id); err != nil {
			return nil, fmt.Errorf("failed to delete old images: %w", err)
		}
		for i, url := range imageURLs {
			url = strings.TrimSpace(url)
			if url == "" {
				continue
			}
			img := &models.ProductImage{
				ProductID: id,
				ImageURL:  url,
				SortOrder: i,
				IsPrimary: i == 0,
			}
			if err := s.productRepo.AddImage(img); err != nil {
				return nil, fmt.Errorf("failed to save image: %w", err)
			}
		}
	}

	return s.GetByID(id)
}

func (s *ProductService) Delete(id uint) error {
	_, err := s.productRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrProductNotFound
		}
		return fmt.Errorf("failed to find product: %w", err)
	}
	return s.productRepo.Delete(id)
}

func (s *ProductService) List(req *dto.ProductListRequest) (*dto.PaginatedResponse, error) {
	offset := (req.Page - 1) * req.PageSize

	params := repository.ProductListParams{
		Offset:    offset,
		Limit:     req.PageSize,
		Classify:  req.Classify,
		Category:  req.Category,
		MinPrice:  req.MinPrice,
		MaxPrice:  req.MaxPrice,
		MinRating: req.MinRating,
		Status:    req.Status,
		Search:    req.Search,
		SortBy:    req.SortBy,
		SortDir:   req.SortDir,
	}

	products, total, err := s.productRepo.List(params)
	if err != nil {
		return nil, fmt.Errorf("failed to list products: %w", err)
	}

	items := make([]dto.ProductResponse, len(products))
	for i, p := range products {
		items[i] = *s.toResponse(&p)
	}

	totalPages := int(math.Ceil(float64(total) / float64(req.PageSize)))
	if totalPages == 0 {
		totalPages = 1
	}

	return &dto.PaginatedResponse{
		Items:      items,
		Total:      total,
		Page:       req.Page,
		PageSize:   req.PageSize,
		TotalPages: totalPages,
	}, nil
}

func (s *ProductService) generateSlug(input string) string {
	slug := strings.ToLower(strings.TrimSpace(input))
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = strings.ReplaceAll(slug, "_", "-")
	slug = productSlugNonAlnum.ReplaceAllString(slug, "")
	slug = productSlugMultiHyph.ReplaceAllString(slug, "-")
	return strings.Trim(slug, "-")
}

func (s *ProductService) ensureUniqueSlug(slug string, excludeID uint) (string, error) {
	exists, err := s.productRepo.ExistsBySlug(slug, excludeID)
	if err != nil {
		return "", err
	}
	if !exists {
		return slug, nil
	}
	for i := 2; i <= 101; i++ {
		candidate := fmt.Sprintf("%s-%d", slug, i)
		exists, err := s.productRepo.ExistsBySlug(candidate, excludeID)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
	}
	return "", errors.New("could not generate unique slug")
}

func (s *ProductService) toResponse(p *models.Product) *dto.ProductResponse {
	resp := &dto.ProductResponse{
		ID:            p.ID,
		CategoryID:    p.CategoryID,
		Name:          p.Name,
		Slug:          p.Slug,
		Description:   p.Description,
		Classify:      p.Classify,
		Price:         p.Price,
		Stock:         p.Stock,
		RatingAverage: p.RatingAverage,
		RatingCount:   p.RatingCount,
		Status:        p.Status,
		SocialShare:   s.buildSocialShare(p),
		CreatedAt:     p.CreatedAt,
		UpdatedAt:     p.UpdatedAt,
	}

	if p.Category != nil {
		resp.CategoryName = p.Category.Name
	}

	if len(p.Images) > 0 {
		resp.Images = make([]dto.ProductImageResponse, len(p.Images))
		for i, img := range p.Images {
			resp.Images[i] = dto.ProductImageResponse{
				ID:        img.ID,
				ImageURL:  img.ImageURL,
				AltText:   img.AltText,
				SortOrder: img.SortOrder,
				IsPrimary: img.IsPrimary,
			}
			if img.IsPrimary {
				imgResp := resp.Images[i]
				resp.PrimaryImage = &imgResp
			}
		}
		if resp.PrimaryImage == nil {
			resp.PrimaryImage = &resp.Images[0]
		}
	}

	return resp
}

func (s *ProductService) buildSocialShare(p *models.Product) dto.ProductSocialShareResponse {
	productURL := s.buildProductURL(p.Slug)
	shareText := fmt.Sprintf("Khám phá %s tại Foods & Drinks", strings.TrimSpace(p.Name))

	encodedProductURL := url.QueryEscape(productURL)
	encodedShareText := url.QueryEscape(shareText)

	return dto.ProductSocialShareResponse{
		Facebook: "https://www.facebook.com/sharer/sharer.php?u=" + encodedProductURL,
		Twitter:  "https://twitter.com/intent/tweet?url=" + encodedProductURL + "&text=" + encodedShareText,
	}
}

func (s *ProductService) buildProductURL(slug string) string {
	baseURL := s.baseURL
	if baseURL == "" {
		baseURL = "http://localhost:8000"
	}

	slug = strings.TrimSpace(slug)
	if slug == "" {
		return baseURL
	}

	return baseURL + "/products/" + url.PathEscape(slug)
}
