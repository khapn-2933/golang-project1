package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kha/foods-drinks/internal/config"
)

func newProfileServiceForHelperTest(uploadPath string, maxSize int64, allowed []string) *ProfileService {
	return NewProfileService(nil, &config.UploadConfig{
		Path:         uploadPath,
		MaxSize:      maxSize,
		AllowedTypes: allowed,
	}, "/uploads")
}

func TestProfileService_IsAllowedExtension(t *testing.T) {
	t.Parallel()

	svc := newProfileServiceForHelperTest("/tmp", 2*1024*1024, []string{"jpg", ".png", "webp"})

	if !svc.IsAllowedExtension(".jpg") {
		t.Fatal("expected .jpg to be allowed")
	}
	if !svc.IsAllowedExtension(".png") {
		t.Fatal("expected .png to be allowed")
	}
	if svc.IsAllowedExtension(".gif") {
		t.Fatal("expected .gif to be rejected")
	}
}

func TestProfileService_MaxSizeHuman(t *testing.T) {
	t.Parallel()

	svcMB := newProfileServiceForHelperTest("/tmp", 2*1024*1024, []string{"jpg"})
	if got := svcMB.MaxSizeHuman(); got != "2MB" {
		t.Fatalf("MaxSizeHuman MB = %s, want 2MB", got)
	}

	svcKB := newProfileServiceForHelperTest("/tmp", 1536, []string{"jpg"})
	if got := svcKB.MaxSizeHuman(); got != "1KB" {
		t.Fatalf("MaxSizeHuman KB = %s, want 1KB", got)
	}

	svcB := newProfileServiceForHelperTest("/tmp", 512, []string{"jpg"})
	if got := svcB.MaxSizeHuman(); got != "512B" {
		t.Fatalf("MaxSizeHuman B = %s, want 512B", got)
	}
}

func TestProfileService_AllowedTypesHuman(t *testing.T) {
	t.Parallel()

	svc := newProfileServiceForHelperTest("/tmp", 1024, []string{"jpg", "jpeg", "png"})
	if got := svc.AllowedTypesHuman(); got != "jpg, jpeg, png" {
		t.Fatalf("AllowedTypesHuman = %s", got)
	}
}

func TestProfileService_IsAllowedMIME(t *testing.T) {
	t.Parallel()

	svc := newProfileServiceForHelperTest("/tmp", 1024, []string{"jpg", "png"})
	if !svc.isAllowedMIME("image/jpeg") {
		t.Fatal("expected image/jpeg to be allowed")
	}
	if !svc.isAllowedMIME("image/png; charset=binary") {
		t.Fatal("expected image/png with params to be allowed")
	}
	if svc.isAllowedMIME("text/plain") {
		t.Fatal("expected text/plain to be rejected")
	}
}

func TestProfileService_SafeRemoveAvatar(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	svc := newProfileServiceForHelperTest(dir, 1024, []string{"jpg"})

	keepPath := filepath.Join(dir, "keep.jpg")
	if err := os.WriteFile(keepPath, []byte("x"), 0644); err != nil {
		t.Fatalf("write keep file: %v", err)
	}

	removePath := filepath.Join(dir, "remove.jpg")
	if err := os.WriteFile(removePath, []byte("x"), 0644); err != nil {
		t.Fatalf("write remove file: %v", err)
	}

	svc.safeRemoveAvatar("/uploads/remove.jpg")
	if _, err := os.Stat(removePath); !os.IsNotExist(err) {
		t.Fatalf("expected remove.jpg to be deleted, err=%v", err)
	}

	// Traversal-like URL should not delete files outside basename target.
	svc.safeRemoveAvatar("../../etc/passwd")
	if _, err := os.Stat(keepPath); err != nil {
		t.Fatalf("keep.jpg should remain, err=%v", err)
	}
}
