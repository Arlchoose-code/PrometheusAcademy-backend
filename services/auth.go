package services

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/url"
	"strings"
	"time"

	"academyprometheus/backend/config"
	"academyprometheus/backend/models"
	"academyprometheus/backend/structs"

	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

const (
	tokenTypeAccess  = "access"
	tokenTypeRefresh = "refresh"
	accessTokenTTL   = 15 * time.Minute
	refreshTokenTTL  = 7 * 24 * time.Hour
	passwordResetTTL = 30 * time.Minute
	authOTPTTL       = 10 * time.Minute
	loginOTPInterval = 30 * 24 * time.Hour
)

var (
	ErrEmailAlreadyRegistered = errors.New("email is already registered")
	ErrInvalidCredentials     = errors.New("invalid credentials")
	ErrInvalidOrExpiredOTP    = errors.New("invalid or expired code")
	ErrEmailAlreadyVerified   = errors.New("email is already verified")
)

type TokenClaims struct {
	UserID       uint   `json:"user_id"`
	TokenType    string `json:"token_type"`
	TokenVersion int    `json:"token_version"`
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
}

type AuthOTPChallenge struct {
	RequiresOTP bool
	Purpose     string
	Email       string
	Message     string
}

type AuthService struct {
	db  *gorm.DB
	cfg config.Config
}

func NewAuthService(db *gorm.DB, cfg config.Config) *AuthService {
	return &AuthService{db: db, cfg: cfg}
}

func (s *AuthService) Register(ctx context.Context, req structs.RegisterRequest) (models.User, TokenPair, *AuthOTPChallenge, error) {
	if s.db == nil {
		return models.User{}, TokenPair{}, nil, errors.New("database is not configured")
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	var existing models.User
	if err := s.db.WithContext(ctx).Where("email = ?", email).First(&existing).Error; err == nil {
		if existing.EmailVerifiedAt != nil {
			return models.User{}, TokenPair{}, nil, ErrEmailAlreadyRegistered
		}
		hash, err := HashPassword(req.Password)
		if err != nil {
			return models.User{}, TokenPair{}, nil, err
		}
		language := req.Language
		if language == "" {
			language = "en"
		}
		existing.Name = strings.TrimSpace(req.Name)
		existing.Password = hash
		existing.Language = language
		if err := s.db.WithContext(ctx).Save(&existing).Error; err != nil {
			return models.User{}, TokenPair{}, nil, fmt.Errorf("auth register update pending user: %w", err)
		}
		configured, err := s.transactionalEmailConfigured(ctx)
		if err != nil {
			return models.User{}, TokenPair{}, nil, err
		}
		if !configured {
			return s.completeAuthWithoutOTP(ctx, existing)
		}
		challenge, err := s.sendAuthOTP(ctx, existing, "register")
		return existing, TokenPair{}, challenge, err
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.User{}, TokenPair{}, nil, fmt.Errorf("auth register lookup: %w", err)
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		return models.User{}, TokenPair{}, nil, err
	}

	language := req.Language
	if language == "" {
		language = "en"
	}

	user := models.User{
		Name:      strings.TrimSpace(req.Name),
		Email:     email,
		Password:  hash,
		IsStudent: true,
		IsAdmin:   false,
		Language:  language,
	}
	if err := s.db.WithContext(ctx).Create(&user).Error; err != nil {
		return models.User{}, TokenPair{}, nil, fmt.Errorf("auth register create user: %w", err)
	}
	configured, err := s.transactionalEmailConfigured(ctx)
	if err != nil {
		return models.User{}, TokenPair{}, nil, err
	}
	if !configured {
		return s.completeAuthWithoutOTP(ctx, user)
	}

	challenge, err := s.sendAuthOTP(ctx, user, "register")
	return user, TokenPair{}, challenge, err
}

func (s *AuthService) Login(ctx context.Context, req structs.LoginRequest) (models.User, TokenPair, *AuthOTPChallenge, error) {
	if s.db == nil {
		return models.User{}, TokenPair{}, nil, errors.New("database is not configured")
	}

	var user models.User
	if err := s.db.WithContext(ctx).Where("email = ?", strings.ToLower(strings.TrimSpace(req.Email))).First(&user).Error; err != nil {
		return models.User{}, TokenPair{}, nil, ErrInvalidCredentials
	}
	if err := CheckPassword(req.Password, user.Password); err != nil {
		return models.User{}, TokenPair{}, nil, ErrInvalidCredentials
	}
	configured, err := s.transactionalEmailConfigured(ctx)
	if err != nil {
		return models.User{}, TokenPair{}, nil, err
	}
	if user.EmailVerifiedAt == nil && configured {
		challenge, err := s.sendAuthOTP(ctx, user, "register")
		return user, TokenPair{}, challenge, err
	}
	if user.EmailVerifiedAt == nil {
		return s.completeAuthWithoutOTP(ctx, user)
	}
	if configured && s.needsLoginOTP(user) {
		challenge, err := s.sendAuthOTP(ctx, user, "login")
		return user, TokenPair{}, challenge, err
	}

	tokens, err := s.IssueTokenPair(user.ID, user.TokenVersion)
	if err != nil {
		return models.User{}, TokenPair{}, nil, err
	}

	return user, tokens, nil, nil
}

func (s *AuthService) transactionalEmailConfigured(ctx context.Context) (bool, error) {
	settings, err := LoadMailerSettings(ctx, s.db)
	if err != nil {
		return false, fmt.Errorf("load auth mailer settings: %w", err)
	}
	return MailerDeliveryConfigured(settings), nil
}

func (s *AuthService) completeAuthWithoutOTP(ctx context.Context, user models.User) (models.User, TokenPair, *AuthOTPChallenge, error) {
	now := time.Now()
	if user.EmailVerifiedAt == nil {
		if err := s.db.WithContext(ctx).Model(&user).Updates(map[string]any{
			"email_verified_at": now,
			"last_login_otp_at": now,
		}).Error; err != nil {
			return models.User{}, TokenPair{}, nil, fmt.Errorf("complete auth without otp: %w", err)
		}
		user.EmailVerifiedAt = &now
		user.LastLoginOTPAt = &now
	}
	tokens, err := s.IssueTokenPair(user.ID, user.TokenVersion)
	return user, tokens, nil, err
}

func (s *AuthService) IssueTokenPair(userID uint, tokenVersion int) (TokenPair, error) {
	if s.cfg.JWTSecret == "" || s.cfg.JWTRefreshSecret == "" {
		return TokenPair{}, errors.New("jwt secrets are not configured")
	}

	now := time.Now()
	accessExpiresAt := now.Add(accessTokenTTL)
	refreshExpiresAt := now.Add(refreshTokenTTL)

	accessToken, err := s.signToken(userID, tokenVersion, tokenTypeAccess, accessExpiresAt)
	if err != nil {
		return TokenPair{}, err
	}
	refreshToken, err := s.signToken(userID, tokenVersion, tokenTypeRefresh, refreshExpiresAt)
	if err != nil {
		return TokenPair{}, err
	}

	return TokenPair{
		AccessToken:      accessToken,
		AccessExpiresAt:  accessExpiresAt,
		RefreshToken:     refreshToken,
		RefreshExpiresAt: refreshExpiresAt,
	}, nil
}

func (s *AuthService) ParseAccessToken(token string) (*TokenClaims, error) {
	return s.parseToken(token, s.cfg.JWTSecret, tokenTypeAccess)
}

func (s *AuthService) ParseRefreshToken(token string) (*TokenClaims, error) {
	return s.parseToken(token, s.cfg.JWTRefreshSecret, tokenTypeRefresh)
}

func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (models.User, TokenPair, error) {
	claims, err := s.ParseRefreshToken(refreshToken)
	if err != nil {
		return models.User{}, TokenPair{}, err
	}
	if err := s.EnsureTokenAllowed(ctx, claims.ID); err != nil {
		return models.User{}, TokenPair{}, err
	}
	if err := s.BlacklistToken(ctx, claims.ID, claims.ExpiresAt.Time); err != nil {
		return models.User{}, TokenPair{}, err
	}

	var user models.User
	if err := s.db.WithContext(ctx).First(&user, claims.UserID).Error; err != nil {
		return models.User{}, TokenPair{}, fmt.Errorf("auth refresh user: %w", err)
	}

	if claims.TokenVersion != user.TokenVersion {
		return models.User{}, TokenPair{}, errors.New("token session is revoked")
	}

	tokens, err := s.IssueTokenPair(user.ID, user.TokenVersion)
	if err != nil {
		return models.User{}, TokenPair{}, err
	}

	return user, tokens, nil
}

func (s *AuthService) BlacklistToken(ctx context.Context, jti string, expiredAt time.Time) error {
	if jti == "" {
		return nil
	}

	item := models.JWTBlacklist{TokenJTI: jti, ExpiredAt: expiredAt}
	if err := s.db.WithContext(ctx).Where(models.JWTBlacklist{TokenJTI: jti}).Attrs(item).FirstOrCreate(&item).Error; err != nil {
		return fmt.Errorf("blacklist jwt: %w", err)
	}
	return nil
}

func (s *AuthService) EnsureTokenAllowed(ctx context.Context, jti string) error {
	if jti == "" {
		return errors.New("missing token id")
	}

	var count int64
	if err := s.db.WithContext(ctx).Model(&models.JWTBlacklist{}).Where("token_jti = ?", jti).Count(&count).Error; err != nil {
		return fmt.Errorf("check jwt blacklist: %w", err)
	}
	if count > 0 {
		return errors.New("token is revoked")
	}
	return nil
}

func (s *AuthService) EnsureTokenVersion(user models.User, claims *TokenClaims) error {
	if claims == nil || claims.TokenVersion != user.TokenVersion {
		return errors.New("token session is revoked")
	}
	return nil
}

func (s *AuthService) RevokeUserSessions(ctx context.Context, userID uint) error {
	if userID == 0 {
		return nil
	}
	if err := s.db.WithContext(ctx).
		Model(&models.User{}).
		Where("id = ?", userID).
		Update("token_version", gorm.Expr("token_version + 1")).Error; err != nil {
		return fmt.Errorf("revoke user sessions: %w", err)
	}
	return nil
}

func (s *AuthService) VerifyAuthOTP(ctx context.Context, req structs.VerifyAuthOTPRequest) (models.User, TokenPair, error) {
	if s.db == nil {
		return models.User{}, TokenPair{}, errors.New("database is not configured")
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	purpose := strings.ToLower(strings.TrimSpace(req.Purpose))
	var user models.User
	if err := s.db.WithContext(ctx).Where("email = ?", email).First(&user).Error; err != nil {
		return models.User{}, TokenPair{}, ErrInvalidOrExpiredOTP
	}
	var otp models.AuthEmailOTP
	if err := s.db.WithContext(ctx).
		Where("email = ? AND purpose = ? AND used_at IS NULL AND expires_at > ?", email, purpose, time.Now()).
		Order("id DESC").
		First(&otp).Error; err != nil {
		return models.User{}, TokenPair{}, ErrInvalidOrExpiredOTP
	}
	expectedHash := authOTPHash(s.cfg.AuthOTPSecret, email, purpose, req.Code)
	if subtle.ConstantTimeCompare([]byte(otp.CodeHash), []byte(expectedHash)) != 1 {
		attempts := otp.Attempts + 1
		maxAttempts := s.cfg.AuthOTPMaxAttempts
		if maxAttempts <= 0 {
			maxAttempts = 5
		}
		updates := map[string]any{"attempts": attempts}
		if attempts >= maxAttempts {
			now := time.Now()
			updates["used_at"] = &now
		}
		if err := s.db.WithContext(ctx).Model(&otp).Updates(updates).Error; err != nil {
			return models.User{}, TokenPair{}, fmt.Errorf("record auth otp attempt: %w", err)
		}
		return models.User{}, TokenPair{}, ErrInvalidOrExpiredOTP
	}
	now := time.Now()
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&otp).Update("used_at", &now).Error; err != nil {
			return err
		}
		updates := map[string]any{}
		if purpose == "register" {
			updates["email_verified_at"] = now
			updates["last_login_otp_at"] = now
		}
		if purpose == "login" {
			updates["last_login_otp_at"] = now
		}
		if len(updates) > 0 {
			if err := tx.Model(&user).Updates(updates).Error; err != nil {
				return err
			}
		}
		return tx.Where("email = ? AND purpose = ? AND used_at IS NULL", email, purpose).Delete(&models.AuthEmailOTP{}).Error
	}); err != nil {
		return models.User{}, TokenPair{}, fmt.Errorf("verify auth otp: %w", err)
	}
	if err := s.db.WithContext(ctx).First(&user, user.ID).Error; err != nil {
		return models.User{}, TokenPair{}, err
	}
	if purpose == "register" {
		if err := SendTransactionalTemplateEmail(ctx, s.db, EmailTemplateRegister, "welcome", user, map[string]string{
			"dashboard_url": localizedFrontendURL(s.cfg, user.Language, "/dashboard"),
		}); err != nil {
			log.Printf("register welcome email failed: user_id=%d email=%s error=%v", user.ID, user.Email, err)
		}
	}
	tokens, err := s.IssueTokenPair(user.ID, user.TokenVersion)
	if err != nil {
		return models.User{}, TokenPair{}, err
	}
	return user, tokens, nil
}

func (s *AuthService) ResendAuthOTP(ctx context.Context, req structs.ResendAuthOTPRequest) (*AuthOTPChallenge, error) {
	if s.db == nil {
		return nil, errors.New("database is not configured")
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	purpose := strings.ToLower(strings.TrimSpace(req.Purpose))
	var user models.User
	if err := s.db.WithContext(ctx).Where("email = ?", email).First(&user).Error; err != nil {
		return &AuthOTPChallenge{RequiresOTP: true, Purpose: purpose, Email: email, Message: "If the account exists, a verification code has been sent."}, nil
	}
	if purpose == "register" && user.EmailVerifiedAt != nil {
		return nil, ErrEmailAlreadyVerified
	}
	return s.sendAuthOTP(ctx, user, purpose)
}

func (s *AuthService) needsLoginOTP(user models.User) bool {
	if user.LastLoginOTPAt == nil {
		return true
	}
	return time.Since(*user.LastLoginOTPAt) >= loginOTPInterval
}

func (s *AuthService) sendAuthOTP(ctx context.Context, user models.User, purpose string) (*AuthOTPChallenge, error) {
	code, err := randomOTPCode()
	if err != nil {
		return nil, err
	}
	purpose = strings.ToLower(strings.TrimSpace(purpose))
	if purpose == "" {
		purpose = "login"
	}
	if err := s.db.WithContext(ctx).Where("email = ? AND purpose = ? AND used_at IS NULL", user.Email, purpose).Delete(&models.AuthEmailOTP{}).Error; err != nil {
		return nil, fmt.Errorf("clear old auth otp: %w", err)
	}
	otp := models.AuthEmailOTP{
		UserID:    user.ID,
		Email:     strings.ToLower(strings.TrimSpace(user.Email)),
		Purpose:   purpose,
		CodeHash:  authOTPHash(s.cfg.AuthOTPSecret, user.Email, purpose, code),
		ExpiresAt: time.Now().Add(authOTPTTL),
	}
	if err := s.db.WithContext(ctx).Create(&otp).Error; err != nil {
		return nil, fmt.Errorf("create auth otp: %w", err)
	}
	settingKey := EmailTemplateLogin
	fallbackKey := "otp_login"
	logLabel := "login otp"
	if purpose == "register" {
		settingKey = EmailTemplateEmailVerification
		fallbackKey = "email_verification"
		logLabel = "register otp"
	}
	if err := SendTransactionalTemplateEmail(ctx, s.db, settingKey, fallbackKey, user, map[string]string{
		"otp": code,
	}); err != nil {
		log.Printf("%s email failed: user_id=%d email=%s error=%v", logLabel, user.ID, user.Email, err)
		return nil, err
	}
	return &AuthOTPChallenge{
		RequiresOTP: true,
		Purpose:     purpose,
		Email:       user.Email,
		Message:     "Verification code sent to email.",
	}, nil
}

func (s *AuthService) RequestPasswordReset(ctx context.Context, req structs.RequestPasswordResetRequest) error {
	if s.db == nil {
		return errors.New("database is not configured")
	}
	var user models.User
	if err := s.db.WithContext(ctx).Where("email = ?", strings.ToLower(strings.TrimSpace(req.Email))).First(&user).Error; err != nil {
		return nil
	}
	token, err := randomResetToken()
	if err != nil {
		return err
	}
	item := models.PasswordResetToken{
		UserID:    user.ID,
		TokenHash: passwordResetTokenHash(token),
		ExpiresAt: time.Now().Add(passwordResetTTL),
	}
	if err := s.db.WithContext(ctx).Create(&item).Error; err != nil {
		return fmt.Errorf("create password reset token: %w", err)
	}
	resetURL := localizedFrontendURL(s.cfg, user.Language, "/login") + "?reset_token=" + url.QueryEscape(token)
	if err := SendTransactionalTemplateEmail(ctx, s.db, EmailTemplatePasswordReset, "password_reset", user, map[string]string{
		"reset_url": resetURL,
	}); err != nil {
		log.Printf("password reset email failed: user_id=%d email=%s error=%v", user.ID, user.Email, err)
	}
	return nil
}

func (s *AuthService) ConfirmPasswordReset(ctx context.Context, req structs.ConfirmPasswordResetRequest) error {
	if s.db == nil {
		return errors.New("database is not configured")
	}
	var token models.PasswordResetToken
	if err := s.db.WithContext(ctx).Where("token_hash = ? AND used_at IS NULL AND expires_at > ?", passwordResetTokenHash(req.Token), time.Now()).First(&token).Error; err != nil {
		return errors.New("invalid or expired reset token")
	}
	hash, err := HashPassword(req.Password)
	if err != nil {
		return err
	}
	now := time.Now()
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.User{}).Where("id = ?", token.UserID).Updates(map[string]any{
			"password":          hash,
			"token_version":     gorm.Expr("token_version + 1"),
			"last_login_otp_at": nil,
		}).Error; err != nil {
			return err
		}
		return tx.Model(&token).Update("used_at", &now).Error
	})
}

func (s *AuthService) UserResponse(user models.User) structs.UserResponse {
	var profile models.UserProfile
	if s.db != nil {
		_ = s.db.Where("user_id = ?", user.ID).First(&profile).Error
	}

	return structs.UserResponse{
		ID:                  user.ID,
		Name:                user.Name,
		Email:               user.Email,
		Avatar:              user.Avatar,
		Phone:               user.Phone,
		IsStudent:           user.IsStudent,
		IsAdmin:             user.IsAdmin,
		IsInstructor:        user.IsInstructor,
		InstructorGrantedAt: user.InstructorGrantedAt,
		InstructorGrantedBy: user.InstructorGrantedBy,
		Language:            user.Language,
		EmailVerifiedAt:     user.EmailVerifiedAt,
		Profile: structs.UserProfileResponse{
			BioEn:        profile.BioEn,
			BioID:        profile.BioID,
			LinkedinURL:  profile.LinkedinURL,
			PortfolioURL: profile.PortfolioURL,
			Skills:       profile.Skills,
		},
	}
}

func (s *AuthService) signToken(userID uint, tokenVersion int, tokenType string, expiresAt time.Time) (string, error) {
	jti, err := randomJTI()
	if err != nil {
		return "", err
	}

	secret := s.cfg.JWTSecret
	if tokenType == tokenTypeRefresh {
		secret = s.cfg.JWTRefreshSecret
	}

	claims := TokenClaims{
		UserID:       userID,
		TokenType:    tokenType,
		TokenVersion: tokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			Subject:   fmt.Sprintf("%d", userID),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("sign jwt: %w", err)
	}
	return signed, nil
}

func (s *AuthService) parseToken(tokenValue, secret, expectedType string) (*TokenClaims, error) {
	if secret == "" {
		return nil, errors.New("jwt secret is not configured")
	}

	token, err := jwt.ParseWithClaims(tokenValue, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrTokenSignatureInvalid
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse jwt: %w", err)
	}

	claims, ok := token.Claims.(*TokenClaims)
	if !ok || !token.Valid || claims.TokenType != expectedType || claims.UserID == 0 {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}

func randomJTI() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate jwt id: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

func randomResetToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate reset token: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

func passwordResetTokenHash(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func randomOTPCode() (string, error) {
	max := big.NewInt(1000000)
	value, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", fmt.Errorf("generate auth otp: %w", err)
	}
	return fmt.Sprintf("%06d", value.Int64()), nil
}

func authOTPHash(secret string, email string, purpose string, code string) string {
	normalized := strings.ToLower(strings.TrimSpace(email)) + "|" + strings.ToLower(strings.TrimSpace(purpose)) + "|" + strings.TrimSpace(code)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(normalized))
	return hex.EncodeToString(mac.Sum(nil))
}

func localizedFrontendURL(cfg config.Config, language string, path string) string {
	base := cfg.BaseURL
	if len(cfg.CORSOrigins) > 0 && strings.TrimSpace(cfg.CORSOrigins[0]) != "" {
		base = strings.TrimSpace(cfg.CORSOrigins[0])
	}
	base = strings.TrimRight(base, "/")
	locale := normalizeMailerLocale(language)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return base + "/" + locale + path
}
