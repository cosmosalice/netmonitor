package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// JWTToken JWT Token 结构
type JWTToken struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// JWTClaims JWT Claims 结构
type JWTClaims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Role     Role   `json:"role"`
	Exp      int64  `json:"exp"`
}

// JWTManager JWT 管理器
type JWTManager struct {
	secretKey []byte
	expiry    time.Duration
}

// NewJWTManager 创建 JWT 管理器
func NewJWTManager(secretKey string) *JWTManager {
	return &JWTManager{
		secretKey: []byte(secretKey),
		expiry:    24 * time.Hour,
	}
}

// GenerateToken 生成 JWT Token
func (jm *JWTManager) GenerateToken(user *User) (*JWTToken, error) {
	now := time.Now()
	exp := now.Add(jm.expiry)

	claims := JWTClaims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		Exp:      exp.Unix(),
	}

	// 创建 header
	header := map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal header: %w", err)
	}

	// 创建 payload
	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal claims: %w", err)
	}

	// Base64 URL 编码（去掉 padding）
	encode := func(data []byte) string {
		return base64.RawURLEncoding.EncodeToString(data)
	}

	encodedHeader := encode(headerJSON)
	encodedPayload := encode(payloadJSON)

	// 创建签名
	signature := jm.sign(encodedHeader + "." + encodedPayload)

	// 组合 token
	token := encodedHeader + "." + encodedPayload + "." + signature

	return &JWTToken{
		Token:     token,
		ExpiresAt: exp,
	}, nil
}

// ValidateToken 验证 JWT Token
func (jm *JWTManager) ValidateToken(tokenString string) (*JWTClaims, error) {
	// 分割 token
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid token format")
	}

	encodedHeader := parts[0]
	encodedPayload := parts[1]
	signature := parts[2]

	// 验证签名
	expectedSignature := jm.sign(encodedHeader + "." + encodedPayload)
	if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		return nil, fmt.Errorf("invalid signature")
	}

	// 解码 payload
	payloadJSON, err := base64.RawURLEncoding.DecodeString(encodedPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to decode payload: %w", err)
	}

	// 解析 claims
	var claims JWTClaims
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return nil, fmt.Errorf("failed to unmarshal claims: %w", err)
	}

	// 检查过期时间
	if time.Now().Unix() > claims.Exp {
		return nil, fmt.Errorf("token expired")
	}

	return &claims, nil
}

// sign 使用 HMAC-SHA256 签名
func (jm *JWTManager) sign(data string) string {
	h := hmac.New(sha256.New, jm.secretKey)
	h.Write([]byte(data))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

// RefreshToken 刷新 Token
func (jm *JWTManager) RefreshToken(claims *JWTClaims) (*JWTToken, error) {
	// 创建临时用户对象用于生成新 token
	user := &User{
		ID:       claims.UserID,
		Username: claims.Username,
		Role:     claims.Role,
	}
	return jm.GenerateToken(user)
}

// GetExpiry 获取 Token 过期时间
func (jm *JWTManager) GetExpiry() time.Duration {
	return jm.expiry
}
