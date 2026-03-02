package service

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/kha/foods-drinks/internal/config"
	"github.com/kha/foods-drinks/internal/dto"
	"github.com/kha/foods-drinks/internal/repository"
	"gorm.io/gorm"
)

var (
	ErrNoAvatar        = errors.New("no avatar to delete")
	ErrFileTooLarge    = errors.New("file too large")
	ErrInvalidFileType = errors.New("invalid file type")
)

// ProfileService handles user profile operations (avatar upload/delete)
type ProfileService struct {
	userRepo        *repository.UserRepository
	uploadConfig    *config.UploadConfig
	publicURLPrefix string // e.g. "/uploads" – the route prefix under which files are served
}

// NewProfileService creates a new ProfileService.
// publicURLPrefix is the URL path prefix used when serving static files (e.g. "/uploads").
func NewProfileService(userRepo *repository.UserRepository, uploadConfig *config.UploadConfig, publicURLPrefix string) *ProfileService {
	return &ProfileService{
		userRepo:        userRepo,
		uploadConfig:    uploadConfig,
		publicURLPrefix: publicURLPrefix,
	}
}

// UploadAvatar uploads an avatar image for a user
func (s *ProfileService) UploadAvatar(userID uint, file *multipart.FileHeader) (*dto.AvatarResponse, error) {
	// Validate file size
	if file.Size > s.uploadConfig.MaxSize {
		return nil, ErrFileTooLarge
	}

	// Validate file extension (first-pass, cheap check)
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !s.isAllowedType(ext) {
		return nil, ErrInvalidFileType
	}

	// Open uploaded file early so we can sniff the MIME type from magic bytes
	src, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer func() { _ = src.Close() }()

	// Read first 512 bytes for content-type detection (http.DetectContentType only needs up to 512)
	buf := make([]byte, 512)
	n, err := src.Read(buf)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("failed to read uploaded file: %w", err)
	}
	detectedMIME := http.DetectContentType(buf[:n])

	// Validate that the detected MIME type is an allowed image type
	if !s.isAllowedMIME(detectedMIME) {
		return nil, ErrInvalidFileType
	}

	// Seek back to beginning so we can copy the full file
	if seeker, ok := src.(io.ReadSeeker); ok {
		if _, err := seeker.Seek(0, io.SeekStart); err != nil {
			return nil, fmt.Errorf("failed to seek uploaded file: %w", err)
		}
	} else {
		// Fallback: re-open the file since it doesn't support seeking
		_ = src.Close()
		src, err = file.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to re-open uploaded file: %w", err)
		}
	}

	// Find user
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to find user: %w", err)
	}

	// Remember the old avatar URL *before* making any changes.
	// We will only remove the old file after the DB update succeeds,
	// so a failed update never leaves the user without an avatar.
	oldAvatarURL := ""
	if user.AvatarURL != nil {
		oldAvatarURL = *user.AvatarURL
	}

	// Generate unique filename
	filename := fmt.Sprintf("%s%s", uuid.New().String(), ext)
	savePath := filepath.Join(s.uploadConfig.Path, filename)

	// Build the public URL from the configured prefix and the filename only.
	// This keeps the URL independent of the filesystem path, avoiding issues
	// where savePath already contains directory segments or uses OS path separators.
	avatarURL := s.publicURLPrefix + "/" + filename

	// Ensure upload directory exists
	if err := os.MkdirAll(s.uploadConfig.Path, 0755); err != nil {
		return nil, fmt.Errorf("failed to create upload directory: %w", err)
	}

	// Create destination file
	dst, err := os.Create(savePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create destination file: %w", err)
	}
	defer func() { _ = dst.Close() }()

	// Copy file content
	if _, err := io.Copy(dst, src); err != nil {
		_ = os.Remove(savePath) // clean up new file on copy failure
		return nil, fmt.Errorf("failed to save file: %w", err)
	}

	// Update user avatar URL in DB
	user.AvatarURL = &avatarURL
	if err := s.userRepo.Update(user); err != nil {
		_ = os.Remove(savePath) // clean up new file; old file is untouched
		return nil, fmt.Errorf("failed to update user avatar: %w", err)
	}

	// DB update succeeded – now it is safe to remove the old avatar file
	if oldAvatarURL != "" {
		s.safeRemoveAvatar(oldAvatarURL)
	}

	return &dto.AvatarResponse{
		AvatarURL: avatarURL,
	}, nil
}

// DeleteAvatar removes the avatar for a user
func (s *ProfileService) DeleteAvatar(userID uint) error {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("failed to find user: %w", err)
	}

	if user.AvatarURL == nil || *user.AvatarURL == "" {
		return ErrNoAvatar
	}

	// Delete the file from disk (path-confined to the upload directory)
	s.safeRemoveAvatar(*user.AvatarURL)

	// Clear avatar URL in DB
	user.AvatarURL = nil
	if err := s.userRepo.Update(user); err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	return nil
}

// isAllowedType checks if the file extension is in the allowed types list.
// It normalises each configured entry by stripping a leading "." so that
// both "jpg" and ".jpg" in config are treated identically.
func (s *ProfileService) isAllowedType(ext string) bool {
	for _, allowed := range s.uploadConfig.AllowedTypes {
		// Normalise: ensure the configured entry always starts with exactly one dot
		normalised := "." + strings.TrimPrefix(allowed, ".")
		if ext == normalised {
			return true
		}
	}
	return false
}

// IsAllowedExtension is the exported counterpart of isAllowedType, used by the
// handler layer for early validation before the file reaches the service.
func (s *ProfileService) IsAllowedExtension(ext string) bool {
	return s.isAllowedType(ext)
}

// MaxFileSize returns the configured maximum file size in bytes.
// Exposed so the handler can perform an early size check before calling the service.
func (s *ProfileService) MaxFileSize() int64 {
	return s.uploadConfig.MaxSize
}

// MaxSizeHuman returns the configured max file size as a human-readable string (e.g. "2MB").
func (s *ProfileService) MaxSizeHuman() string {
	const mb = 1024 * 1024
	const kb = 1024
	size := s.uploadConfig.MaxSize
	switch {
	case size >= mb:
		return fmt.Sprintf("%dMB", size/mb)
	case size >= kb:
		return fmt.Sprintf("%dKB", size/kb)
	default:
		return fmt.Sprintf("%dB", size)
	}
}

// AllowedTypesHuman returns the configured allowed types as a human-readable string (e.g. "jpg, jpeg, png, webp").
func (s *ProfileService) AllowedTypesHuman() string {
	return strings.Join(s.uploadConfig.AllowedTypes, ", ")
}

// allowedMIMETypes maps permitted file extensions to their expected MIME type prefixes.
// http.DetectContentType returns values like "image/jpeg", "image/png", etc.
var allowedMIMETypes = map[string]string{
	"jpg":  "image/jpeg",
	"jpeg": "image/jpeg",
	"png":  "image/png",
	"gif":  "image/gif",
	"webp": "image/webp",
}

// isAllowedMIME validates the detected MIME type against the allowed types configured
// for this service, using magic-byte detection rather than the filename extension.
func (s *ProfileService) isAllowedMIME(detectedMIME string) bool {
	for _, allowed := range s.uploadConfig.AllowedTypes {
		// Normalise: strip any leading dot so the map key lookup is consistent
		key := strings.TrimPrefix(allowed, ".")
		if expected, ok := allowedMIMETypes[key]; ok {
			// detectedMIME may carry parameters (e.g. "image/jpeg; charset=…") – use prefix match
			if strings.HasPrefix(detectedMIME, expected) {
				return true
			}
		}
	}
	return false
}

// safeRemoveAvatar deletes the file referenced by avatarURL, but only if the resolved
// filesystem path is strictly within the configured upload directory. It uses only the
// base filename from the URL, so any path-traversal segments (e.g. "../../etc/passwd")
// embedded in AvatarURL cannot escape the upload directory.
func (s *ProfileService) safeRemoveAvatar(avatarURL string) {
	// Resolve the upload root to an absolute path first
	uploadAbs, err := filepath.Abs(s.uploadConfig.Path)
	if err != nil {
		return
	}

	// Use only the base filename from the stored URL – this defeats any
	// ".." traversal that could exist in a corrupted/tampered AvatarURL.
	base := filepath.Base(filepath.FromSlash(avatarURL))
	if base == "." || base == "/" || base == "" {
		return
	}

	// Build candidate path entirely within the upload directory
	candidateAbs := filepath.Join(uploadAbs, base)

	// Final sanity check: candidate must still be inside the upload root
	if !strings.HasPrefix(candidateAbs, uploadAbs+string(os.PathSeparator)) {
		return
	}

	_ = os.Remove(candidateAbs) // best-effort; ignore error
}
