package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type Config struct {
	AppPort                 string
	AppEnv                  string
	DBHost                  string
	DBPort                  string
	DBUser                  string
	DBPass                  string
	DBName                  string
	JWTSecret               string
	JWTRefreshSecret        string
	AuthOTPSecret           string
	MidtransServerKey       string
	MidtransClientKey       string
	MidtransEnv             string
	StoragePath             string
	StorageProvider         string
	R2AccountID             string
	R2AccessKeyID           string
	R2SecretAccessKey       string
	R2Bucket                string
	R2PublicBaseURL         string
	R2SignedURLTTLSeconds   int
	BackupR2Bucket          string
	BackupR2AccessKeyID     string
	BackupR2SecretAccessKey string
	ChromiumPath            string
	PDFRenderTimeoutSeconds int
	PDFRenderConcurrency    int
	CWebPBin                string
	BaseURL                 string
	FrontendURL             string
	CORSOrigins             []string
	RateLimitPerMinute      int
	AuthRateLimitPerMinute  int
	OTPVerifyRatePerMinute  int
	OTPResendRatePerMinute  int
	PasswordRatePerMinute   int
	AuthOTPMaxAttempts      int
	PaymentExpiresMinutes   int
}

func Load() Config {
	loadEnvFile(".env")

	return Config{
		AppPort:                 env("APP_PORT", "8080"),
		AppEnv:                  env("APP_ENV", "development"),
		DBHost:                  env("DB_HOST", "127.0.0.1"),
		DBPort:                  env("DB_PORT", "3306"),
		DBUser:                  env("DB_USER", "root"),
		DBPass:                  env("DB_PASS", ""),
		DBName:                  env("DB_NAME", "academyprometheus"),
		JWTSecret:               env("JWT_SECRET", ""),
		JWTRefreshSecret:        env("JWT_REFRESH_SECRET", ""),
		AuthOTPSecret:           env("AUTH_OTP_SECRET", env("JWT_SECRET", "")),
		MidtransServerKey:       env("MIDTRANS_SERVER_KEY", ""),
		MidtransClientKey:       env("MIDTRANS_CLIENT_KEY", ""),
		MidtransEnv:             env("MIDTRANS_ENV", "sandbox"),
		StoragePath:             env("STORAGE_PATH", "storage"),
		StorageProvider:         strings.ToLower(env("STORAGE_PROVIDER", "local")),
		R2AccountID:             env("R2_ACCOUNT_ID", ""),
		R2AccessKeyID:           env("R2_ACCESS_KEY_ID", ""),
		R2SecretAccessKey:       env("R2_SECRET_ACCESS_KEY", ""),
		R2Bucket:                env("R2_BUCKET", ""),
		R2PublicBaseURL:         strings.TrimRight(env("R2_PUBLIC_BASE_URL", ""), "/"),
		R2SignedURLTTLSeconds:   intEnv("R2_SIGNED_URL_TTL_SECONDS", 300),
		BackupR2Bucket:          env("BACKUP_R2_BUCKET", ""),
		BackupR2AccessKeyID:     env("BACKUP_R2_ACCESS_KEY_ID", ""),
		BackupR2SecretAccessKey: env("BACKUP_R2_SECRET_ACCESS_KEY", ""),
		ChromiumPath:            env("CHROMIUM_PATH", ""),
		PDFRenderTimeoutSeconds: intEnv("PDF_RENDER_TIMEOUT_SECONDS", 30),
		PDFRenderConcurrency:    intEnv("PDF_RENDER_CONCURRENCY", 2),
		CWebPBin:                env("CWEBP_BIN", "cwebp"),
		BaseURL:                 env("BASE_URL", "http://localhost:8080"),
		FrontendURL:             env("FRONTEND_URL", "http://localhost:3000"),
		CORSOrigins:             splitCSV(env("CORS_ORIGINS", "http://localhost:3000,http://127.0.0.1:3000")),
		RateLimitPerMinute:      intEnv("RATE_LIMIT_PER_MINUTE", 600),
		AuthRateLimitPerMinute:  intEnv("AUTH_RATE_LIMIT_PER_MINUTE", 10),
		OTPVerifyRatePerMinute:  intEnv("OTP_VERIFY_RATE_LIMIT_PER_MINUTE", 10),
		OTPResendRatePerMinute:  intEnv("OTP_RESEND_RATE_LIMIT_PER_MINUTE", 3),
		PasswordRatePerMinute:   intEnv("PASSWORD_RESET_RATE_LIMIT_PER_MINUTE", 5),
		AuthOTPMaxAttempts:      intEnv("AUTH_OTP_MAX_ATTEMPTS", 5),
		PaymentExpiresMinutes:   intEnv("PAYMENT_EXPIRES_MINUTES", 30),
	}
}

func SetupLogger(cfg Config) {
	zerolog.TimeFieldFormat = time.RFC3339
	if cfg.AppEnv != "production" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})
	}
}

func ConnectDatabase(cfg Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", cfg.DBUser, cfg.DBPass, cfg.DBHost, cfg.DBPort, cfg.DBName)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("connect database: %w", err)
	}
	return db, nil
}

func loadEnvFile(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key == "" || os.Getenv(key) != "" {
			continue
		}
		_ = os.Setenv(key, value)
	}
}

func env(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func intEnv(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}
