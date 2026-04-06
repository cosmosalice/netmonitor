package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/netmonitor/backend/auth"
)

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	Token     string     `json:"token"`
	ExpiresAt int64      `json:"expires_at"`
	User      *auth.User `json:"user"`
}

// POST /api/v1/auth/login - 登录（无需认证）
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// 验证用户
	user, err := s.authManager.Authenticate(req.Username, req.Password)
	if err != nil {
		writeError(w, "invalid username or password", http.StatusUnauthorized)
		return
	}

	// 生成 token
	token, err := s.jwtManager.GenerateToken(user)
	if err != nil {
		writeError(w, "failed to generate token", http.StatusInternalServerError)
		return
	}

	// 返回响应（不包含密码）
	user.Password = ""
	writeJSON(w, LoginResponse{
		Token:     token.Token,
		ExpiresAt: token.ExpiresAt.Unix(),
		User:      user,
	})
}

// POST /api/v1/auth/logout - 登出
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	// JWT 是无状态的，客户端只需删除 token
	// 这里可以添加 token 黑名单逻辑（如果需要）
	writeJSON(w, map[string]string{"status": "logged out"})
}

// GET /api/v1/auth/me - 获取当前用户信息
func (s *Server) handleGetMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		writeError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := s.authManager.GetUser(userID)
	if err != nil {
		writeError(w, "user not found", http.StatusNotFound)
		return
	}

	// 不包含密码
	user.Password = ""
	writeJSON(w, user)
}

// PUT /api/v1/auth/password - 修改密码
func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok {
		writeError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req auth.ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := s.authManager.ChangePassword(userID, req); err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, map[string]string{"status": "password changed"})
}

// GET /api/v1/users - 用户列表（admin only）
func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.authManager.ListUsers()
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 清除密码
	for _, u := range users {
		u.Password = ""
	}

	writeJSON(w, map[string]interface{}{"users": users})
}

// POST /api/v1/users - 创建用户（admin only）
func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req auth.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	user, err := s.authManager.CreateUser(req)
	if err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 不包含密码
	user.Password = ""
	writeJSON(w, user)
}

// PUT /api/v1/users/{id} - 更新用户（admin only）
func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, "invalid user id", http.StatusBadRequest)
		return
	}

	var req auth.UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	user, err := s.authManager.UpdateUser(id, req)
	if err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 不包含密码
	user.Password = ""
	writeJSON(w, user)
}

// DELETE /api/v1/users/{id} - 删除用户（admin only）
func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, "invalid user id", http.StatusBadRequest)
		return
	}

	// 检查是否删除自己
	currentUserID, _ := auth.GetUserID(r.Context())
	if id == currentUserID {
		writeError(w, "cannot delete yourself", http.StatusBadRequest)
		return
	}

	if err := s.authManager.DeleteUser(id); err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, map[string]string{"status": "user deleted"})
}

// POST /api/v1/users/{id}/reset-password - 重置密码（admin only）
func (s *Server) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, "invalid user id", http.StatusBadRequest)
		return
	}

	var req struct {
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.NewPassword == "" {
		writeError(w, "new_password is required", http.StatusBadRequest)
		return
	}

	if err := s.authManager.ResetPassword(id, req.NewPassword); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"status": "password reset"})
}
