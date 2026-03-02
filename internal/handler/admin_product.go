package handler

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/kha/foods-drinks/internal/dto"
	"github.com/kha/foods-drinks/internal/service"
)

type AdminProductHandler struct {
	productService  *service.ProductService
	categoryService *service.CategoryService
	listTmpl        *template.Template
	formTmpl        *template.Template
}

func NewAdminProductHandler(
	productService *service.ProductService,
	categoryService *service.CategoryService,
	funcMap template.FuncMap,
) *AdminProductHandler {
	layout := "templates/admin/layout.html"
	return &AdminProductHandler{
		productService:  productService,
		categoryService: categoryService,
		listTmpl: template.Must(
			template.New("list").Funcs(funcMap).ParseFiles(layout, "templates/admin/products/list.html"),
		),
		formTmpl: template.Must(
			template.New("form").Funcs(funcMap).ParseFiles(layout, "templates/admin/products/form.html"),
		),
	}
}

func (h *AdminProductHandler) render(c *gin.Context, status int, tmpl *template.Template, data gin.H) {
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "layout", data); err != nil {
		c.String(http.StatusInternalServerError, "Template error: %v", err)
		return
	}
	c.Data(status, "text/html; charset=utf-8", buf.Bytes())
}

func (h *AdminProductHandler) setFlash(c *gin.Context, t, msg string) {
	c.SetCookie("flash_product", t+"|"+msg, 0, "/", "", false, true)
}

func (h *AdminProductHandler) getFlash(c *gin.Context) *flash {
	val, err := c.Cookie("flash_product")
	if err != nil || val == "" {
		return nil
	}
	c.SetCookie("flash_product", "", -1, "/", "", false, true)
	parts := strings.SplitN(val, "|", 2)
	if len(parts) != 2 {
		return nil
	}
	return &flash{Type: parts[0], Message: parts[1]}
}

func (h *AdminProductHandler) loadCategories() []dto.CategoryResponse {
	result, err := h.categoryService.List(&dto.CategoryListRequest{
		Page: 1, PageSize: 200, Status: "active", SortBy: "name", SortDir: "asc",
	})
	if err != nil {
		return nil
	}
	cats, _ := result.Items.([]dto.CategoryResponse)
	return cats
}

func (h *AdminProductHandler) List(c *gin.Context) {
	page := 1
	if p, err := strconv.Atoi(c.Query("page")); err == nil && p > 0 {
		page = p
	}

	req := &dto.ProductListRequest{
		Page:     page,
		PageSize: 15,
		Classify: c.Query("classify"),
		Search:   c.Query("search"),
		Status:   c.Query("status"),
		SortBy:   c.DefaultQuery("sort_by", "created_at"),
		SortDir:  c.DefaultQuery("sort_dir", "desc"),
	}
	if cid, err := strconv.ParseUint(c.Query("category_id"), 10, 32); err == nil {
		req.Category = uint(cid)
	}

	result, err := h.productService.List(req)
	if err != nil {
		h.render(c, http.StatusInternalServerError, h.listTmpl, gin.H{
			"Title":      "Sản phẩm",
			"ActiveMenu": "products",
			"Flash":      &flash{Type: flashTypeErr, Message: "Lỗi tải danh sách: " + err.Error()},
		})
		return
	}

	products, _ := result.Items.([]dto.ProductResponse)

	h.render(c, http.StatusOK, h.listTmpl, gin.H{
		"Title":      "Sản phẩm",
		"ActiveMenu": "products",
		"Flash":      h.getFlash(c),
		"Products":   products,
		"Categories": h.loadCategories(),
		"Query": map[string]interface{}{
			"Search":     req.Search,
			"Classify":   req.Classify,
			"Status":     req.Status,
			"CategoryID": req.Category,
			"SortBy":     req.SortBy,
		},
		"Pagination": paginationData{
			Page:       page,
			TotalPages: result.TotalPages,
			Total:      result.Total,
			Pages:      buildPages(page, result.TotalPages),
		},
	})
}

func (h *AdminProductHandler) New(c *gin.Context) {
	h.render(c, http.StatusOK, h.formTmpl, gin.H{
		"Title":      "Thêm sản phẩm",
		"ActiveMenu": "products",
		"Categories": h.loadCategories(),
	})
}

func (h *AdminProductHandler) Create(c *gin.Context) {
	categoryID, _ := strconv.ParseUint(c.PostForm("category_id"), 10, 32)
	price, _ := strconv.ParseFloat(c.PostForm("price"), 64)
	stock, _ := strconv.Atoi(c.PostForm("stock"))
	status := c.PostForm("status")
	if status == "" {
		status = "active"
	}

	req := &dto.CreateProductRequest{
		CategoryID:  uint(categoryID),
		Name:        strings.TrimSpace(c.PostForm("name")),
		Slug:        strings.TrimSpace(c.PostForm("slug")),
		Description: strings.TrimSpace(c.PostForm("description")),
		Classify:    c.PostForm("classify"),
		Price:       price,
		Stock:       stock,
		Status:      status,
	}

	imageURLs := h.parseImageURLs(c)

	if req.Name == "" || req.CategoryID == 0 || req.Classify == "" {
		h.render(c, http.StatusUnprocessableEntity, h.formTmpl, gin.H{
			"Title":      "Thêm sản phẩm",
			"ActiveMenu": "products",
			"Categories": h.loadCategories(),
			"Errors":     []string{"Tên, danh mục và phân loại là bắt buộc."},
			"Form":       req,
			"ImageURLs":  imageURLs,
		})
		return
	}

	_, err := h.productService.Create(req, imageURLs)
	if err != nil {
		errs := h.serviceErrMessages(err)
		h.render(c, http.StatusUnprocessableEntity, h.formTmpl, gin.H{
			"Title":      "Thêm sản phẩm",
			"ActiveMenu": "products",
			"Categories": h.loadCategories(),
			"Errors":     errs,
			"Form":       req,
			"ImageURLs":  imageURLs,
		})
		return
	}

	h.setFlash(c, flashTypeOK, fmt.Sprintf("Đã tạo sản phẩm \"%s\" thành công.", req.Name))
	c.Redirect(http.StatusFound, "/admin/products")
}

func (h *AdminProductHandler) Edit(c *gin.Context) {
	id, ok := h.parseIDParam(c)
	if !ok {
		c.Redirect(http.StatusFound, "/admin/products")
		return
	}

	product, err := h.productService.GetByID(id)
	if err != nil {
		h.setFlash(c, flashTypeErr, "Không tìm thấy sản phẩm.")
		c.Redirect(http.StatusFound, "/admin/products")
		return
	}

	h.render(c, http.StatusOK, h.formTmpl, gin.H{
		"Title":      "Sửa sản phẩm",
		"ActiveMenu": "products",
		"Categories": h.loadCategories(),
		"Product":    product,
	})
}

func (h *AdminProductHandler) Update(c *gin.Context) {
	id, ok := h.parseIDParam(c)
	if !ok {
		c.Redirect(http.StatusFound, "/admin/products")
		return
	}

	categoryID, _ := strconv.ParseUint(c.PostForm("category_id"), 10, 32)
	price, _ := strconv.ParseFloat(c.PostForm("price"), 64)
	stock, _ := strconv.Atoi(c.PostForm("stock"))
	catIDUint := uint(categoryID)
	status := c.PostForm("status")
	classify := c.PostForm("classify")
	name := strings.TrimSpace(c.PostForm("name"))
	slug := strings.TrimSpace(c.PostForm("slug"))
	desc := strings.TrimSpace(c.PostForm("description"))

	req := &dto.UpdateProductRequest{
		CategoryID:  &catIDUint,
		Name:        &name,
		Slug:        &slug,
		Description: &desc,
		Classify:    &classify,
		Price:       &price,
		Stock:       &stock,
		Status:      &status,
	}

	imageURLs := h.parseImageURLs(c)
	replaceImages := len(imageURLs) > 0

	product, err := h.productService.GetByID(id)
	if err != nil {
		h.setFlash(c, flashTypeErr, "Không tìm thấy sản phẩm.")
		c.Redirect(http.StatusFound, "/admin/products")
		return
	}

	_, err = h.productService.Update(id, req, imageURLs, replaceImages)
	if err != nil {
		errs := h.serviceErrMessages(err)
		h.render(c, http.StatusUnprocessableEntity, h.formTmpl, gin.H{
			"Title":      "Sửa sản phẩm",
			"ActiveMenu": "products",
			"Categories": h.loadCategories(),
			"Errors":     errs,
			"Product":    product,
		})
		return
	}

	h.setFlash(c, flashTypeOK, fmt.Sprintf("Đã cập nhật sản phẩm \"%s\".", name))
	c.Redirect(http.StatusFound, "/admin/products")
}

func (h *AdminProductHandler) Delete(c *gin.Context) {
	id, ok := h.parseIDParam(c)
	if !ok {
		c.Redirect(http.StatusFound, "/admin/products")
		return
	}

	if err := h.productService.Delete(id); err != nil {
		h.setFlash(c, flashTypeErr, "Không thể xoá sản phẩm: "+err.Error())
	} else {
		h.setFlash(c, flashTypeOK, "Đã xoá sản phẩm.")
	}
	c.Redirect(http.StatusFound, "/admin/products")
}

func (h *AdminProductHandler) parseIDParam(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		return 0, false
	}
	return uint(id), true
}

func (h *AdminProductHandler) parseImageURLs(c *gin.Context) []string {
	raw := c.PostFormArray("image_urls")
	var urls []string
	for _, u := range raw {
		u = strings.TrimSpace(u)
		if u != "" {
			urls = append(urls, u)
		}
	}
	return urls
}

func (h *AdminProductHandler) serviceErrMessages(err error) []string {
	switch {
	case err == service.ErrProductNotFound:
		return []string{"Không tìm thấy sản phẩm."}
	case err == service.ErrProductSlugExists:
		return []string{"Slug này đã được sử dụng."}
	case err == service.ErrProductEmptySlug:
		return []string{"Tên sản phẩm phải chứa ít nhất một ký tự chữ hoặc số."}
	default:
		return []string{"Đã có lỗi xảy ra: " + err.Error()}
	}
}
