package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type tokenType string

const (
	accessToken  tokenType = "access"
	refreshToken tokenType = "refresh"
)

type contextKey string

const UserIDKey contextKey = "user_id"

type Claims struct {
	UserID    string    `json:"user_id"`
	Email     string    `json:"email"`
	TokenType tokenType `json:"token_type"`
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

func GenerateTokenPair(userID, email, secret string) (TokenPair, error) {
	access, err := newToken(userID, email, accessToken, 15*time.Minute, secret)
	if err != nil {
		return TokenPair{}, fmt.Errorf("generate access token: %w", err)
	}

	refresh, err := newToken(userID, email, refreshToken, 7*24*time.Hour, secret)
	if err != nil {
		return TokenPair{}, fmt.Errorf("generate refresh token: %w", err)
	}

	return TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    900, // 15 min in seconds
	}, nil
}

func ValidateAccessToken(tokenStr, secret string) (*Claims, error) {
	return validate(tokenStr, secret, accessToken)
}

func ValidateRefreshToken(tokenStr, secret string) (*Claims, error) {
	return validate(tokenStr, secret, refreshToken)
}

func UserIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(UserIDKey).(string)
	return id, ok
}

func newToken(userID, email string, tt tokenType, ttl time.Duration, secret string) (string, error) {
	claims := Claims{
		UserID:    userID,
		Email:     email,
		TokenType: tt,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

func validate(tokenStr, secret string, expected tokenType) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, fmt.Errorf("token expired: %w", err)
		}
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}
	if claims.TokenType != expected {
		return nil, fmt.Errorf("wrong token type: got %s, want %s", claims.TokenType, expected)
	}
	return claims, nil
}
