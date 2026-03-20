package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/kha/foods-drinks/internal/dto"
	"github.com/kha/foods-drinks/internal/middleware"
	"github.com/kha/foods-drinks/internal/service"
)

func TestParsePositiveUint64(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		ok   bool
		want uint64
	}{
		{"1", true, 1},
		{" 42 ", true, 42},
		{"0", false, 0},
		{"", false, 0},
		{"-1", false, 0},
		{"abc", false, 0},
	}

	for _, tc := range cases {
		got, ok := parsePositiveUint64(tc.in)
		if ok != tc.ok || got != tc.want {
			t.Fatalf("parsePositiveUint64(%q) = (%d,%v), want (%d,%v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestValidateCreateOrderRequest(t *testing.T) {
	t.Parallel()

	req1 := &dto.CreateOrderRequest{ShippingAddress: "", ShippingPhone: ""}
	d1 := validateCreateOrderRequest(req1)
	if d1["shipping_address"] == "" || d1["shipping_phone"] == "" {
		t.Fatalf("expected required field errors, got %+v", d1)
	}

	req2 := &dto.CreateOrderRequest{ShippingAddress: "HN", ShippingPhone: "09ab"}
	d2 := validateCreateOrderRequest(req2)
	if d2["shipping_phone"] == "" {
		t.Fatalf("expected invalid character error, got %+v", d2)
	}

	req3 := &dto.CreateOrderRequest{ShippingAddress: "HN", ShippingPhone: "1234567"}
	d3 := validateCreateOrderRequest(req3)
	if d3["shipping_phone"] == "" {
		t.Fatalf("expected min digits error, got %+v", d3)
	}

	req4 := &dto.CreateOrderRequest{ShippingAddress: "HN", ShippingPhone: "+84 90123456"}
	d4 := validateCreateOrderRequest(req4)
	if len(d4) != 0 {
		t.Fatalf("expected valid request, got %+v", d4)
	}
}

func TestOrderHandler_CreateValidationAndUnauthorized(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	h := NewOrderHandler(nil)

	r1 := gin.New()
	r1.POST("/orders", h.Create)
	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewBufferString(`{"shipping_address":"HN","shipping_phone":"0901234567"}`))
	req1.Header.Set("Content-Type", "application/json")
	r1.ServeHTTP(w1, req1)
	if w1.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status=%d want=401", w1.Code)
	}

	r2 := gin.New()
	r2.Use(func(c *gin.Context) {
		c.Set(middleware.ContextKeyUserID, uint(1))
		c.Next()
	})
	r2.POST("/orders", h.Create)
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewBufferString(`{}`))
	req2.Header.Set("Content-Type", "application/json")
	r2.ServeHTTP(w2, req2)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("validator error status=%d want=400", w2.Code)
	}

	w3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewBufferString(`not-json`))
	req3.Header.Set("Content-Type", "application/json")
	r2.ServeHTTP(w3, req3)
	if w3.Code != http.StatusBadRequest {
		t.Fatalf("invalid json status=%d want=400", w3.Code)
	}

	w4 := httptest.NewRecorder()
	req4 := httptest.NewRequest(http.MethodPost, "/orders", bytes.NewBufferString(`{"shipping_address":"HN","shipping_phone":"09ab"}`))
	req4.Header.Set("Content-Type", "application/json")
	r2.ServeHTTP(w4, req4)
	if w4.Code != http.StatusBadRequest {
		t.Fatalf("custom validation status=%d want=400", w4.Code)
	}
}

func TestOrderHandler_ListAndGetDetailValidation(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	h := NewOrderHandler(nil)

	r := gin.New()
	r.GET("/orders", h.List)
	r.GET("/orders/:id", h.GetDetail)

	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "/orders", nil)
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusUnauthorized {
		t.Fatalf("list unauthorized status=%d want=401", w1.Code)
	}

	rAuth := gin.New()
	rAuth.Use(func(c *gin.Context) {
		c.Set(middleware.ContextKeyUserID, uint(1))
		c.Next()
	})
	rAuth.GET("/orders", h.List)
	rAuth.GET("/orders/:id", h.GetDetail)

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/orders?page=0", nil)
	rAuth.ServeHTTP(w2, req2)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("list invalid params status=%d want=400", w2.Code)
	}

	w3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodGet, "/orders/abc", nil)
	rAuth.ServeHTTP(w3, req3)
	if w3.Code != http.StatusBadRequest {
		t.Fatalf("get detail invalid id status=%d want=400", w3.Code)
	}
}

func TestOrderHandler_HandleOrderError(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	h := NewOrderHandler(nil)

	tests := []struct {
		err  error
		want int
	}{
		{service.ErrCartEmpty, http.StatusBadRequest},
		{service.ErrProductNotFound, http.StatusNotFound},
		{service.ErrInsufficientStock, http.StatusBadRequest},
		{service.ErrOrderNotFound, http.StatusNotFound},
		{service.ErrInvalidDateFilter, http.StatusBadRequest},
		{service.ErrInvalidOrderInput, http.StatusBadRequest},
		{errors.New("unknown"), http.StatusInternalServerError},
	}

	for _, tc := range tests {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		h.handleOrderError(c, tc.err)
		if w.Code != tc.want {
			t.Fatalf("handleOrderError(%v) status=%d want=%d", tc.err, w.Code, tc.want)
		}
		var body map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		if body["error"] == nil {
			t.Fatalf("expected error field in response for %v", tc.err)
		}
	}
}
