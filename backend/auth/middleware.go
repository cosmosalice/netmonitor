package auth

import (
	"context"
	"net/http"
	"strings"
)

// contextKey 用于在 context 中存储用户信息
type contextKey string

const (
	// ContextKeyUser 存储用户信息的 context key
	ContextKeyUser contextKey = "user"
)

// UserContext 存储在 context 中的用户信息
type UserContext struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Role     Role   `json:"role"`
}

// AuthMiddleware 认证中间件
func (jm *JWTManager) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 跳过登录接口
		if r.URL.Path == "/api/v1/auth/login" {
			next.ServeHTTP(w, r)
			return
		}

		// 从 Header 中提取 token
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeError(w, "missing authorization header", http.StatusUnauthorized)
			return
		}

		// 检查 Bearer 前缀
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			writeError(w, "invalid authorization header format", http.StatusUnauthorized)
			return
		}

		tokenString := parts[1]

		// 验证 token
		claims, err := jm.ValidateToken(tokenString)
		if err != nil {
			writeError(w, "invalid or expired token", http.StatusUnauthorized)
			return
		}

		// 将用户信息存入 context
		userCtx := UserContext{
			UserID:   claims.UserID,
			Username: claims.Username,
			Role:     claims.Role,
		}
		ctx := context.WithValue(r.Context(), ContextKeyUser, userCtx)

		// 继续处理请求
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AdminOnly admin 权限中间件
func AdminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := GetUserFromContext(r.Context())
		if !ok {
			writeError(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		if user.Role != RoleAdmin {
			writeError(w, "admin access required", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// GetUserFromContext 从 context 中获取用户信息
func GetUserFromContext(ctx context.Context) (*UserContext, bool) {
	user, ok := ctx.Value(ContextKeyUser).(UserContext)
	if !ok {
		return nil, false
	}
	return &user, true
}

// GetUserID 从 context 中获取用户 ID
func GetUserID(ctx context.Context) (int64, bool) {
	user, ok := GetUserFromContext(ctx)
	if !ok {
		return 0, false
	}
	return user.UserID, true
}

// GetUsername 从 context 中获取用户名
func GetUsername(ctx context.Context) (string, bool) {
	user, ok := GetUserFromContext(ctx)
	if !ok {
		return "", false
	}
	return user.Username, true
}

// IsAdmin 检查用户是否为 admin
func IsAdmin(ctx context.Context) bool {
	user, ok := GetUserFromContext(ctx)
	if !ok {
		return false
	}
	return user.Role == RoleAdmin
}

// writeError 写入错误响应
func writeError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write([]byte(`{"error":"` + msg + `"}`))
}
