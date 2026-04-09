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

// 创建令牌Manager相关实例，组装运行所需依赖并返回给上层流程复用。
func NewTokenManager(secret string, expirationMinutes int) *TokenManager {
	return &TokenManager{
		secret:     []byte(secret),
		expiration: time.Duration(expirationMinutes) * time.Minute,
	}
}

// 判断是否存在hPassword，供当前链路选择后续处理策略。
func (m *TokenManager) HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// 处理VerifyPassword相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func (m *TokenManager) VerifyPassword(password string, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// 签发令牌，为后续请求或设备交互生成临时凭证或动作。
func (m *TokenManager) IssueToken(userID string) (string, error) {
	return m.IssueTokenWithDuration(userID, m.expiration)
}

// 签发令牌时长，为后续请求或设备交互生成临时凭证或动作。
func (m *TokenManager) IssueTokenWithDuration(userID string, duration time.Duration) (string, error) {
	if duration <= 0 {
		duration = m.expiration
	}
	now := time.Now().UTC()
	claims := jwt.MapClaims{
		"sub": userID,
		"iat": now.Unix(),
		"exp": now.Add(duration).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// 解析令牌，为安全令牌提供结构化输入。
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
