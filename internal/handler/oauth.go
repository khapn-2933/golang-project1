package handler

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kha/foods-drinks/internal/dto"
	"github.com/kha/foods-drinks/internal/service"
)

// OAuthHandler handles OAuth HTTP requests
type OAuthHandler struct {
	oauthService *service.OAuthService
}

// NewOAuthHandler creates a new OAuthHandler
func NewOAuthHandler(oauthService *service.OAuthService) *OAuthHandler {
	return &OAuthHandler{
		oauthService: oauthService,
	}
}

// GetProviders godoc
// @Summary Get supported OAuth providers
// @Description Get list of supported OAuth providers
// @Tags oauth
// @Produce json
// @Success 200 {object} dto.OAuthProvidersResponse
// @Router /api/v1/auth/oauth/providers [get]
func (h *OAuthHandler) GetProviders(c *gin.Context) {
	providers := h.oauthService.GetSupportedProviders()
	c.JSON(http.StatusOK, dto.OAuthProvidersResponse{
		Providers: providers,
	})
}

// InitiateOAuth godoc
// @Summary Initiate OAuth flow
// @Description Get OAuth authorization URL for the specified provider
// @Tags oauth
// @Produce json
// @Param provider path string true "OAuth provider (google, facebook, twitter)"
// @Success 200 {object} dto.OAuthURLResponse
// @Failure 400 {object} dto.ErrorResponse
// @Router /api/v1/auth/oauth/{provider} [get]
func (h *OAuthHandler) InitiateOAuth(c *gin.Context) {
	provider := c.Param("provider")

	// Validate provider
	if !h.oauthService.IsProviderSupported(provider) {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "invalid_provider",
			Message: "OAuth provider not supported or not configured",
		})
		return
	}

	// Generate random state for CSRF protection
	state, err := generateRandomState()
	if err != nil {
		log.Printf("Failed to generate state: %v", err)
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to initiate OAuth flow",
		})
		return
	}

	// Store state in cookie for verification
	// Secure flag is determined based on the request (HTTPS or behind reverse proxy)
	secure := isSecureRequest(c)
	c.SetCookie("oauth_state", state, 600, "/", "", secure, true)

	// Get authorization URL
	authURL, err := h.oauthService.GetAuthURL(provider, state)
	if err != nil {
		log.Printf("Failed to get auth URL: %v", err)
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to generate OAuth URL",
		})
		return
	}

	c.JSON(http.StatusOK, dto.OAuthURLResponse{
		URL:      authURL,
		Provider: provider,
		State:    state,
	})
}

// HandleCallback godoc
// @Summary Handle OAuth callback
// @Description Handle OAuth callback from provider
// @Tags oauth
// @Produce json
// @Param provider path string true "OAuth provider (google, facebook, twitter)"
// @Param code query string true "Authorization code"
// @Param state query string true "State for CSRF protection"
// @Success 200 {object} dto.AuthResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/v1/auth/oauth/{provider}/callback [get]
func (h *OAuthHandler) HandleCallback(c *gin.Context) {
	provider := c.Param("provider")

	// Validate provider
	if !h.oauthService.IsProviderSupported(provider) {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "invalid_provider",
			Message: "OAuth provider not supported or not configured",
		})
		return
	}

	// Bind query parameters
	var req dto.OAuthCallbackRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "invalid_request",
			Message: "Missing required parameters (code and state)",
		})
		return
	}

	// Verify state (CSRF protection)
	storedState, err := c.Cookie("oauth_state")
	if err != nil || storedState != req.State {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "invalid_state",
			Message: "Invalid or expired OAuth state",
		})
		return
	}

	// Clear state cookie
	secure := isSecureRequest(c)
	c.SetCookie("oauth_state", "", -1, "/", "", secure, true)

	// Handle callback
	resp, err := h.oauthService.HandleCallback(c.Request.Context(), provider, req.Code)
	if err != nil {
		h.handleOAuthError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// HandleCallbackRedirect handles OAuth callback and redirects to frontend
// Uses HTTP-only secure cookie for token to prevent XSS attacks
// The token is NOT passed in URL to avoid exposure in browser history, logs, and referrer headers
func (h *OAuthHandler) HandleCallbackRedirect(c *gin.Context, frontendURL string, secureCookie bool) {
	provider := c.Param("provider")

	// Validate provider
	if !h.oauthService.IsProviderSupported(provider) {
		c.Redirect(http.StatusTemporaryRedirect, frontendURL+"?error=invalid_provider")
		return
	}

	// Bind query parameters
	var req dto.OAuthCallbackRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.Redirect(http.StatusTemporaryRedirect, frontendURL+"?error=invalid_request")
		return
	}

	// Verify state
	storedState, err := c.Cookie("oauth_state")
	if err != nil || storedState != req.State {
		c.Redirect(http.StatusTemporaryRedirect, frontendURL+"?error=invalid_state")
		return
	}

	// Clear state cookie
	c.SetCookie("oauth_state", "", -1, "/", "", secureCookie, true)

	// Handle callback
	resp, err := h.oauthService.HandleCallback(c.Request.Context(), provider, req.Code)
	if err != nil {
		errCode := "auth_failed"
		if errors.Is(err, service.ErrUserInactive) {
			errCode = "user_inactive"
		} else if errors.Is(err, service.ErrUserBanned) {
			errCode = "user_banned"
		} else if errors.Is(err, service.ErrOAuthEmailRequired) {
			errCode = "email_required"
		}
		c.Redirect(http.StatusTemporaryRedirect, frontendURL+"?error="+errCode)
		return
	}

	// Set token in HTTP-only secure cookie instead of URL parameter
	// This prevents token exposure in:
	// - Browser history
	// - Server logs
	// - Referrer headers
	// Cookie expires in 24 hours (86400 seconds)
	c.SetCookie("access_token", resp.AccessToken, 86400, "/", "", secureCookie, true)

	// Redirect to frontend success page
	c.Redirect(http.StatusTemporaryRedirect, frontendURL+"?auth=success")
}

// handleOAuthError handles OAuth errors and returns appropriate response
func (h *OAuthHandler) handleOAuthError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrOAuthProviderNotSupported):
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "provider_not_supported",
			Message: "OAuth provider not supported",
		})
	case errors.Is(err, service.ErrOAuthStateMismatch):
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "state_mismatch",
			Message: "OAuth state mismatch",
		})
	case errors.Is(err, service.ErrOAuthCodeExchange):
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "code_exchange_failed",
			Message: "Failed to exchange authorization code",
		})
	case errors.Is(err, service.ErrOAuthUserInfo):
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "user_info_failed",
			Message: "Failed to get user information from provider",
		})
	case errors.Is(err, service.ErrOAuthEmailRequired):
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error:   "email_required",
			Message: "OAuth provider did not provide email address",
		})
	case errors.Is(err, service.ErrUserInactive):
		c.JSON(http.StatusForbidden, dto.ErrorResponse{
			Error:   "user_inactive",
			Message: "Your account is inactive",
		})
	case errors.Is(err, service.ErrUserBanned):
		c.JSON(http.StatusForbidden, dto.ErrorResponse{
			Error:   "user_banned",
			Message: "Your account has been banned",
		})
	default:
		log.Printf("OAuth error: %v", err)
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error:   "internal_error",
			Message: "An unexpected error occurred during OAuth authentication",
		})
	}
}

// generateRandomState generates a random state string for CSRF protection
func generateRandomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// isSecureRequest determines if the request is over HTTPS
// Checks both direct TLS and reverse proxy headers
func isSecureRequest(c *gin.Context) bool {
	return c.Request.TLS != nil || c.Request.Header.Get("X-Forwarded-Proto") == "https"
}
