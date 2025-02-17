package models

import (
	"github.com/golang-jwt/jwt/v5"
)

type AccessClaims struct {
	ID          uint   `json:"id"`
	NickName    string `json:"nick_name"`
	AuthorityId uint   `json:"authority_id"`
	jwt.RegisteredClaims
}

type RefreshClaims struct {
	UserID uint `json:"user_id"`
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}