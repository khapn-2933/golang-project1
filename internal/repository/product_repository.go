package repository

import (
	"github.com/kha/foods-drinks/internal/models"
	"gorm.io/gorm"
)

type ProductRepository struct {
	db *gorm.DB
}

func NewProductRepository(db *gorm.DB) *ProductRepository {
	return &ProductRepository{db: db}
}

func (r *ProductRepository) Create(product *models.Product) error {
	return r.db.Create(product).Error
}

func (r *ProductRepository) FindByID(id uint) (*models.Product, error) {
	var p models.Product
	err := r.db.Preload("Images", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC")
	}).Preload("Category").First(&p, id).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *ProductRepository) FindBySlug(slug string) (*models.Product, error) {
	var p models.Product
	err := r.db.Preload("Images", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC")
	}).Preload("Category").Where("slug = ?", slug).First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *ProductRepository) ExistsBySlug(slug string, excludeID ...uint) (bool, error) {
	var count int64
	query := r.db.Model(&models.Product{}).Where("slug = ?", slug)
	if len(excludeID) > 0 && excludeID[0] > 0 {
		query = query.Where("id != ?", excludeID[0])
	}
	err := query.Count(&count).Error
	return count > 0, err
}

func (r *ProductRepository) Update(product *models.Product) error {
	return r.db.Save(product).Error
}

func (r *ProductRepository) Delete(id uint) error {
	return r.db.Delete(&models.Product{}, id).Error
}

func (r *ProductRepository) AddImage(img *models.ProductImage) error {
	return r.db.Create(img).Error
}

func (r *ProductRepository) DeleteImagesByProductID(productID uint) error {
	return r.db.Where("product_id = ?", productID).Delete(&models.ProductImage{}).Error
}

func (r *ProductRepository) SetPrimaryImage(productID, imageID uint) error {
	if err := r.db.Model(&models.ProductImage{}).
		Where("product_id = ?", productID).
		Update("is_primary", false).Error; err != nil {
		return err
	}
	return r.db.Model(&models.ProductImage{}).
		Where("id = ? AND product_id = ?", imageID, productID).
		Update("is_primary", true).Error
}

type ProductListParams struct {
	Offset    int
	Limit     int
	Classify  string
	Category  uint
	MinPrice  float64
	MaxPrice  float64
	MinRating float64
	Status    string
	Search    string
	SortBy    string
	SortDir   string
}

func (r *ProductRepository) List(params ProductListParams) ([]models.Product, int64, error) {
	var products []models.Product
	var total int64

	query := r.db.Model(&models.Product{})

	if params.Classify != "" {
		query = query.Where("classify = ?", params.Classify)
	}
	if params.Category > 0 {
		query = query.Where("category_id = ?", params.Category)
	}
	if params.MinPrice > 0 {
		query = query.Where("price >= ?", params.MinPrice)
	}
	if params.MaxPrice > 0 {
		query = query.Where("price <= ?", params.MaxPrice)
	}
	if params.MinRating > 0 {
		query = query.Where("rating_average >= ?", params.MinRating)
	}
	if params.Status != "" {
		query = query.Where("status = ?", params.Status)
	}
	if params.Search != "" {
		query = query.Where("MATCH(name, description) AGAINST(? IN BOOLEAN MODE)", params.Search+"*")
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	sortBy := "created_at"
	allowed := map[string]bool{"price": true, "rating_average": true, "name": true, "created_at": true, "sort_order": true}
	if allowed[params.SortBy] {
		sortBy = params.SortBy
	}
	sortDir := "desc"
	if params.SortDir == "asc" {
		sortDir = "asc"
	}

	err := query.
		Preload("Images", func(db *gorm.DB) *gorm.DB {
			return db.Where("is_primary = ?", true).Limit(1)
		}).
		Preload("Category").
		Order(sortBy + " " + sortDir).
		Offset(params.Offset).
		Limit(params.Limit).
		Find(&products).Error

	return products, total, err
}
