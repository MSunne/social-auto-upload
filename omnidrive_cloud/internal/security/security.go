package security

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type TokenManager struct {
	secret     []byte
	expiration time.Duration
}

func NewTokenManager(secret string, expirationMinutes int) *TokenManager {
	return &TokenManager{
		secret:     []byte(secret),
		expiration: time.Duration(expirationMinutes) * time.Minute,
	}
}

func (m *TokenManager) HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func (m *TokenManager) VerifyPassword(password string, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func (m *TokenManager) IssueToken(userID string) (string, error) {
	now := time.Now().UTC()
	claims := jwt.MapClaims{
		"sub": userID,
		"iat": now.Unix(),
		"exp": now.Add(m.expiration).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

func (m *TokenManager) ParseToken(raw string) (string, error) {
	token, err := jwt.Parse(raw, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected jwt method: %s", token.Method.Alg())
		}
		return m.secret, nil
	})
	if err != nil {
		return "", err
	}
	if !token.Valid {
		return "", errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", errors.New("invalid token claims")
	}

	subject, ok := claims["sub"].(string)
	if !ok || subject == "" {
		return "", errors.New("missing token subject")
	}
	return subject, nil
}
