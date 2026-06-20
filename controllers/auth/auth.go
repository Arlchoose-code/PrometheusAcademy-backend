package auth

import (
	"errors"
	"net/http"
	"strings"

	"academyprometheus/backend/config"
	"academyprometheus/backend/models"
	"academyprometheus/backend/services"
	"academyprometheus/backend/structs"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Controller struct {
	db            *gorm.DB
	authService   *services.AuthService
	uploadService *services.UploadService
	secureCookie  bool
}

func NewController(db *gorm.DB, cfg config.Config, uploadService *services.UploadService) *Controller {
	return &Controller{
		db:            db,
		authService:   services.NewAuthService(db, cfg),
		uploadService: uploadService,
		secureCookie:  cfg.AppEnv == "production",
	}
}

func (h *Controller) CreateUser(c *gin.Context) {
	var req structs.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Code: "invalid_registration_data", Message: "Please provide a valid name, email, and password of at least 8 characters"})
		return
	}

	_, tokens, challenge, err := h.authService.Register(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, services.ErrEmailAlreadyRegistered) {
			c.JSON(http.StatusConflict, structs.Response{Success: false, Code: "email_already_registered", Message: "This email is already registered"})
			return
		}
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Code: "registration_failed", Message: "Registration could not be completed"})
		return
	}
	if challenge != nil && challenge.RequiresOTP {
		c.JSON(http.StatusAccepted, structs.Response{Success: true, Message: challenge.Message, Data: structs.AuthChallengeResponse{
			RequiresOTP: true,
			Purpose:     challenge.Purpose,
			Email:       challenge.Email,
			Message:     challenge.Message,
		}})
		return
	}

	http.SetCookie(c.Writer, services.AccessTokenCookie(tokens.AccessToken, tokens.AccessExpiresAt, h.secureCookie))
	http.SetCookie(c.Writer, services.RefreshTokenCookie(tokens.RefreshToken, tokens.RefreshExpiresAt, h.secureCookie))
	c.JSON(http.StatusCreated, structs.Response{Success: true, Message: "Registration successful"})
}

func (h *Controller) Login(c *gin.Context) {
	var req structs.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Code: "invalid_login_data", Message: "Please provide a valid email and password"})
		return
	}

	user, tokens, challenge, err := h.authService.Login(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, services.ErrInvalidCredentials) {
			c.JSON(http.StatusUnauthorized, structs.Response{Success: false, Code: "invalid_credentials", Message: "Invalid email or password"})
			return
		}
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Code: "otp_delivery_failed", Message: "Failed to send verification code"})
		return
	}
	if challenge != nil && challenge.RequiresOTP {
		c.JSON(http.StatusAccepted, structs.Response{Success: true, Message: challenge.Message, Data: structs.AuthChallengeResponse{
			RequiresOTP: true,
			Purpose:     challenge.Purpose,
			Email:       challenge.Email,
			Message:     challenge.Message,
		}})
		return
	}

	http.SetCookie(c.Writer, services.AccessTokenCookie(tokens.AccessToken, tokens.AccessExpiresAt, h.secureCookie))
	http.SetCookie(c.Writer, services.RefreshTokenCookie(tokens.RefreshToken, tokens.RefreshExpiresAt, h.secureCookie))
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Login successful", Data: h.authService.UserResponse(user)})
}

func (h *Controller) VerifyAuthOTP(c *gin.Context) {
	var req structs.VerifyAuthOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Code: "invalid_otp_data", Message: "Enter the 6-digit verification code"})
		return
	}
	user, tokens, err := h.authService.VerifyAuthOTP(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, services.ErrInvalidOrExpiredOTP) {
			c.JSON(http.StatusBadRequest, structs.Response{Success: false, Code: "invalid_or_expired_otp", Message: "Invalid or expired verification code"})
			return
		}
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Code: "otp_verification_failed", Message: "Verification could not be completed"})
		return
	}
	http.SetCookie(c.Writer, services.AccessTokenCookie(tokens.AccessToken, tokens.AccessExpiresAt, h.secureCookie))
	http.SetCookie(c.Writer, services.RefreshTokenCookie(tokens.RefreshToken, tokens.RefreshExpiresAt, h.secureCookie))
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Verification successful", Data: h.authService.UserResponse(user)})
}

func (h *Controller) ResendAuthOTP(c *gin.Context) {
	var req structs.ResendAuthOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Code: "invalid_otp_request", Message: "Invalid verification request"})
		return
	}
	challenge, err := h.authService.ResendAuthOTP(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, services.ErrEmailAlreadyVerified) {
			c.JSON(http.StatusConflict, structs.Response{Success: false, Code: "email_already_verified", Message: "This email is already verified. Please log in."})
			return
		}
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Code: "otp_delivery_failed", Message: "Failed to send verification code"})
		return
	}
	c.JSON(http.StatusAccepted, structs.Response{Success: true, Message: challenge.Message, Data: structs.AuthChallengeResponse{
		RequiresOTP: true,
		Purpose:     challenge.Purpose,
		Email:       challenge.Email,
		Message:     challenge.Message,
	}})
}

func (h *Controller) RequestPasswordReset(c *gin.Context) {
	var req structs.RequestPasswordResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Valid email is required"})
		return
	}
	if err := h.authService.RequestPasswordReset(c.Request.Context(), req); err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to request password reset"})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "If the email exists, reset instructions have been sent"})
}

func (h *Controller) ConfirmPasswordReset(c *gin.Context) {
	var req structs.ConfirmPasswordResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid reset payload"})
		return
	}
	if err := h.authService.ConfirmPasswordReset(c.Request.Context(), req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Password has been reset"})
}

func (h *Controller) Refresh(c *gin.Context) {
	refreshToken, err := c.Cookie("refresh_token")
	if err != nil || refreshToken == "" {
		c.JSON(http.StatusUnauthorized, structs.Response{Success: false, Message: "Refresh token missing"})
		return
	}

	user, tokens, err := h.authService.Refresh(c.Request.Context(), refreshToken)
	if err != nil {
		http.SetCookie(c.Writer, services.ExpiredJWTCookie("access_token", h.secureCookie))
		http.SetCookie(c.Writer, services.ExpiredJWTCookie("refresh_token", h.secureCookie))
		c.JSON(http.StatusUnauthorized, structs.Response{Success: false, Message: "Refresh token invalid"})
		return
	}

	http.SetCookie(c.Writer, services.AccessTokenCookie(tokens.AccessToken, tokens.AccessExpiresAt, h.secureCookie))
	http.SetCookie(c.Writer, services.RefreshTokenCookie(tokens.RefreshToken, tokens.RefreshExpiresAt, h.secureCookie))
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Token refreshed", Data: h.authService.UserResponse(user)})
}

func (h *Controller) Logout(c *gin.Context) {
	if accessToken, err := c.Cookie("access_token"); err == nil && accessToken != "" {
		if claims, err := h.authService.ParseAccessToken(accessToken); err == nil {
			_ = h.authService.BlacklistToken(c.Request.Context(), claims.ID, claims.ExpiresAt.Time)
		}
	}
	if refreshToken, err := c.Cookie("refresh_token"); err == nil && refreshToken != "" {
		if claims, err := h.authService.ParseRefreshToken(refreshToken); err == nil {
			_ = h.authService.BlacklistToken(c.Request.Context(), claims.ID, claims.ExpiresAt.Time)
		}
	}

	http.SetCookie(c.Writer, services.ExpiredJWTCookie("access_token", h.secureCookie))
	http.SetCookie(c.Writer, services.ExpiredJWTCookie("refresh_token", h.secureCookie))
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Logged out"})
}

func (h *Controller) GetCurrentUser(c *gin.Context) {
	user := c.MustGet("user")
	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "User loaded", Data: h.authService.UserResponse(user.(models.User))})
}

func (h *Controller) UpdateProfile(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	var req structs.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid profile data"})
		return
	}

	language := req.Language
	if language == "" {
		language = user.Language
	}
	user.Name = strings.TrimSpace(req.Name)
	user.Phone = strings.TrimSpace(req.Phone)
	user.Language = language
	if !validPhone(user.Phone, false) {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Invalid phone number"})
		return
	}
	if err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&user).Error; err != nil {
			return err
		}

		profile := models.UserProfile{
			UserID:       user.ID,
			BioEn:        strings.TrimSpace(req.BioEn),
			BioID:        strings.TrimSpace(req.BioID),
			LinkedinURL:  strings.TrimSpace(req.LinkedinURL),
			PortfolioURL: strings.TrimSpace(req.PortfolioURL),
			Skills:       strings.TrimSpace(req.Skills),
		}
		if err := tx.Where(models.UserProfile{UserID: user.ID}).Assign(profile).FirstOrCreate(&profile).Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		c.JSON(http.StatusInternalServerError, structs.Response{Success: false, Message: "Failed to update profile"})
		return
	}

	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Profile updated", Data: h.authService.UserResponse(user)})
}

func (h *Controller) CreateAvatar(c *gin.Context) {
	user := c.MustGet("user").(models.User)
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: "Avatar file is required"})
		return
	}

	path, err := h.uploadService.SaveUserAvatar(c.Request.Context(), user.ID, file)
	if err != nil {
		c.JSON(http.StatusBadRequest, structs.Response{Success: false, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, structs.Response{Success: true, Message: "Avatar uploaded", Data: gin.H{"avatar": path}})
}

func validPhone(value string, required bool) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return !required
	}
	digits := 0
	for index, char := range value {
		if char >= '0' && char <= '9' {
			digits++
			continue
		}
		if char == '+' && index == 0 {
			continue
		}
		if char == ' ' || char == '-' || char == '.' || char == '(' || char == ')' {
			continue
		}
		return false
	}
	return digits >= 6 && digits <= 20 && len(value) <= 32
}
