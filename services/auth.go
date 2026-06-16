package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
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

type AuthService struct {
	db  *gorm.DB
	cfg config.Config
}

func NewAuthService(db *gorm.DB, cfg config.Config) *AuthService {
	return &AuthService{db: db, cfg: cfg}
}

func (s *AuthService) Register(ctx context.Context, req structs.RegisterRequest) (models.User, TokenPair, error) {
	if s.db == nil {
		return models.User{}, TokenPair{}, errors.New("database is not configured")
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	var existing models.User
	if err := s.db.WithContext(ctx).Where("email = ?", email).First(&existing).Error; err == nil {
		return models.User{}, TokenPair{}, errors.New("email is already registered")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.User{}, TokenPair{}, fmt.Errorf("auth register lookup: %w", err)
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		return models.User{}, TokenPair{}, err
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
		return models.User{}, TokenPair{}, fmt.Errorf("auth register create user: %w", err)
	}

	tokens, err := s.IssueTokenPair(user.ID, user.TokenVersion)
	if err != nil {
		return models.User{}, TokenPair{}, err
	}

	return user, tokens, nil
}

func (s *AuthService) Login(ctx context.Context, req structs.LoginRequest) (models.User, TokenPair, error) {
	if s.db == nil {
		return models.User{}, TokenPair{}, errors.New("database is not configured")
	}

	var user models.User
	if err := s.db.WithContext(ctx).Where("email = ?", strings.ToLower(strings.TrimSpace(req.Email))).First(&user).Error; err != nil {
		return models.User{}, TokenPair{}, errors.New("invalid credentials")
	}
	if err := CheckPassword(req.Password, user.Password); err != nil {
		return models.User{}, TokenPair{}, errors.New("invalid credentials")
	}

	tokens, err := s.IssueTokenPair(user.ID, user.TokenVersion)
	if err != nil {
		return models.User{}, TokenPair{}, err
	}

	return user, tokens, nil
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

func (s *AuthService) UserResponse(user models.User) structs.UserResponse {
	var profile models.UserProfile
	if s.db != nil {
		_ = s.db.Where("user_id = ?", user.ID).First(&profile).Error
	}

	return structs.UserResponse{
		ID:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		Avatar:    user.Avatar,
		Phone:     user.Phone,
		IsStudent: user.IsStudent,
		IsAdmin:   user.IsAdmin,
		Language:  user.Language,
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
