package handler

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/kha/foods-drinks/internal/config"
	"github.com/kha/foods-drinks/internal/middleware"
	"github.com/kha/foods-drinks/internal/service"
)

func multipartBody(t *testing.T, fieldName, filename string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	fw, err := writer.CreateFormFile(fieldName, filename)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	return body, writer.FormDataContentType()
}

func TestProfileHandler_UploadAvatarValidation(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	cfg := &config.UploadConfig{Path: t.TempDir(), MaxSize: 10, AllowedTypes: []string{"jpg", "png"}}
	h := NewProfileHandler(service.NewProfileService(nil, cfg, "/uploads"))

	r := gin.New()
	r.POST("/profile/avatar", h.UploadAvatar)

	// Unauthorized
	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodPost, "/profile/avatar", nil)
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status=%d want=401", w1.Code)
	}

	rAuth := gin.New()
	rAuth.Use(func(c *gin.Context) {
		c.Set(middleware.ContextKeyUserID, uint(1))
		c.Next()
	})
	rAuth.POST("/profile/avatar", h.UploadAvatar)

	// Missing file
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/profile/avatar", nil)
	rAuth.ServeHTTP(w2, req2)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("missing file status=%d want=400", w2.Code)
	}

	// File too large
	bigBody, bigCT := multipartBody(t, "avatar", "avatar.jpg", []byte("01234567890"))
	w3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodPost, "/profile/avatar", bigBody)
	req3.Header.Set("Content-Type", bigCT)
	rAuth.ServeHTTP(w3, req3)
	if w3.Code != http.StatusBadRequest {
		t.Fatalf("file too large status=%d want=400", w3.Code)
	}

	// Invalid extension
	smallBody, smallCT := multipartBody(t, "avatar", "avatar.txt", []byte("1234"))
	w4 := httptest.NewRecorder()
	req4 := httptest.NewRequest(http.MethodPost, "/profile/avatar", smallBody)
	req4.Header.Set("Content-Type", smallCT)
	rAuth.ServeHTTP(w4, req4)
	if w4.Code != http.StatusBadRequest {
		t.Fatalf("invalid extension status=%d want=400", w4.Code)
	}
}

func TestProfileHandler_DeleteAvatarUnauthorized(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	cfg := &config.UploadConfig{Path: t.TempDir(), MaxSize: 10, AllowedTypes: []string{"jpg"}}
	h := NewProfileHandler(service.NewProfileService(nil, cfg, "/uploads"))

	r := gin.New()
	r.DELETE("/profile/avatar", h.DeleteAvatar)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/profile/avatar", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized delete status=%d want=401", w.Code)
	}
}
