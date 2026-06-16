package services

import (
	"net/http"
	"time"
)

func AccessTokenCookie(token string, expiresAt time.Time, secure bool) *http.Cookie {
	return jwtCookie("access_token", token, expiresAt, secure)
}

func RefreshTokenCookie(token string, expiresAt time.Time, secure bool) *http.Cookie {
	return jwtCookie("refresh_token", token, expiresAt, secure)
}

func ExpiredJWTCookie(name string, secure bool) *http.Cookie {
	return jwtCookie(name, "", time.Now().Add(-time.Hour), secure)
}

func jwtCookie(name, value string, expiresAt time.Time, secure bool) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	}
}
