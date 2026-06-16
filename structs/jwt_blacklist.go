package structs

import "time"

type JWTBlacklistRequest struct {
	TokenJTI  string    `json:"token_jti"`
	ExpiredAt time.Time `json:"expired_at"`
}

type JWTBlacklistResponse struct {
	ModelResponse
	JWTBlacklistRequest
}
