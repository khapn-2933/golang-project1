package handler

import (
	"errors"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/kha/foods-drinks/internal/dto"
	"github.com/kha/foods-drinks/internal/middleware"
	"github.com/kha/foods-drinks/internal/service"
)

// ProfileHandler handles user profile-specific endpoints (avatar upload/delete)
type ProfileHandler struct {
	profileService *service.ProfileService
}

// NewProfileHandler creates a new ProfileHandler
func NewProfileHandler(profileService *service.ProfileService) *ProfileHandler {
	return &ProfileHandler{
		profileService: profileService,
	}
}

// UploadAvatar godoc
// @Summary Upload user avatar
// @Description Upload an avatar image for the currently authenticated user
// @Tags profile
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param avatar formData file true "Avatar image file"
// @Success 200 {object} dto.AvatarResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/v1/profile/avatar [post]
func (h *ProfileHandler) UploadAvatar(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, dto.ErrorResponse{
			Error:   "unauthorized",
			Message: "User not authenticated",
		})
		return
	}

	file, err := c.FormFile("avatar")
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "validation_error",
			Message: "Avatar file is required",
		})
		return
	}

	// Early size check – gives the UI a clear message without hitting the service layer
	if file.Size > h.profileService.MaxFileSize() {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "file_too_large",
			Message: "File size exceeds the maximum allowed size (" + h.profileService.MaxSizeHuman() + ")",
		})
		return
	}

	// Early extension check – gives the UI a clear message without hitting the service layer
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !h.profileService.IsAllowedExtension(ext) {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "invalid_file_type",
			Message: "Only " + h.profileService.AllowedTypesHuman() + " files are allowed",
		})
		return
	}

	resp, err := h.profileService.UploadAvatar(userID, file)
	if err != nil {
		h.handleProfileError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// DeleteAvatar godoc
// @Summary Delete user avatar
// @Description Remove the avatar of the currently authenticated user
// @Tags profile
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]string
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 404 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/v1/profile/avatar [delete]
func (h *ProfileHandler) DeleteAvatar(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, dto.ErrorResponse{
			Error:   "unauthorized",
			Message: "User not authenticated",
		})
		return
	}

	if err := h.profileService.DeleteAvatar(userID); err != nil {
		h.handleProfileError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Avatar deleted successfully"})
}

// handleProfileError handles profile service errors
func (h *ProfileHandler) handleProfileError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrUserNotFound):
		c.JSON(http.StatusNotFound, dto.ErrorResponse{
			Error:   "user_not_found",
			Message: "User not found",
		})
	case errors.Is(err, service.ErrNoAvatar):
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "no_avatar",
			Message: "No avatar to delete",
		})
	case errors.Is(err, service.ErrFileTooLarge):
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "file_too_large",
			Message: "File size exceeds the maximum allowed size (" + h.profileService.MaxSizeHuman() + ")",
		})
	case errors.Is(err, service.ErrInvalidFileType):
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "invalid_file_type",
			Message: "Only " + h.profileService.AllowedTypesHuman() + " files are allowed",
		})
	default:
		log.Printf("Profile error: %v", err)
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error:   "internal_error",
			Message: "An unexpected error occurred",
		})
	}
}
