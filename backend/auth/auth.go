package auth

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Role 定义用户角色
type Role string

const (
	RoleAdmin  Role = "admin"
	RoleViewer Role = "viewer"
)

// User 用户结构体
type User struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Password  string    `json:"-"` // 不序列化密码
	Role      Role      `json:"role"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	LastLogin time.Time `json:"last_login"`
}

// CreateUserRequest 创建用户请求
type CreateUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     Role   `json:"role"`
	Email    string `json:"email"`
}

// UpdateUserRequest 更新用户请求
type UpdateUserRequest struct {
	Username string `json:"username,omitempty"`
	Role     Role   `json:"role,omitempty"`
	Email    string `json:"email,omitempty"`
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// AuthManager 认证管理器
type AuthManager struct {
	db *sql.DB
}

// NewAuthManager 创建认证管理器
func NewAuthManager(db *sql.DB) *AuthManager {
	am := &AuthManager{db: db}
	// 确保默认 admin 用户存在
	if err := am.ensureDefaultAdmin(); err != nil {
		log.Printf("Warning: failed to ensure default admin: %v", err)
	}
	return am
}

// ensureDefaultAdmin 确保默认 admin 用户存在
func (am *AuthManager) ensureDefaultAdmin() error {
	// 检查是否已有用户
	var count int
	err := am.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to count users: %w", err)
	}

	// 如果没有用户，创建默认 admin
	if count == 0 {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("failed to hash password: %w", err)
		}

		_, err = am.db.Exec(
			"INSERT INTO users (username, password, role, email) VALUES (?, ?, ?, ?)",
			"admin", string(hashedPassword), RoleAdmin, "admin@netmonitor.local",
		)
		if err != nil {
			return fmt.Errorf("failed to create default admin: %w", err)
		}
		log.Println("Default admin user created (username: admin, password: admin)")
	}

	return nil
}

// CreateUser 创建用户
func (am *AuthManager) CreateUser(req CreateUserRequest) (*User, error) {
	// 验证角色
	if req.Role != RoleAdmin && req.Role != RoleViewer {
		return nil, fmt.Errorf("invalid role: %s", req.Role)
	}

	// 检查用户名是否已存在
	var exists int
	err := am.db.QueryRow("SELECT COUNT(*) FROM users WHERE username = ?", req.Username).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("failed to check username: %w", err)
	}
	if exists > 0 {
		return nil, fmt.Errorf("username already exists")
	}

	// 哈希密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// 插入用户
	result, err := am.db.Exec(
		"INSERT INTO users (username, password, role, email) VALUES (?, ?, ?, ?)",
		req.Username, string(hashedPassword), req.Role, req.Email,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	id, _ := result.LastInsertId()

	return am.GetUser(id)
}

// GetUser 获取用户
func (am *AuthManager) GetUser(id int64) (*User, error) {
	user := &User{}
	var lastLogin sql.NullTime

	err := am.db.QueryRow(
		"SELECT id, username, password, role, email, created_at, last_login FROM users WHERE id = ?",
		id,
	).Scan(&user.ID, &user.Username, &user.Password, &user.Role, &user.Email, &user.CreatedAt, &lastLogin)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if lastLogin.Valid {
		user.LastLogin = lastLogin.Time
	}

	return user, nil
}

// GetUserByUsername 通过用户名获取用户
func (am *AuthManager) GetUserByUsername(username string) (*User, error) {
	user := &User{}
	var lastLogin sql.NullTime

	err := am.db.QueryRow(
		"SELECT id, username, password, role, email, created_at, last_login FROM users WHERE username = ?",
		username,
	).Scan(&user.ID, &user.Username, &user.Password, &user.Role, &user.Email, &user.CreatedAt, &lastLogin)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if lastLogin.Valid {
		user.LastLogin = lastLogin.Time
	}

	return user, nil
}

// ListUsers 获取所有用户
func (am *AuthManager) ListUsers() ([]*User, error) {
	rows, err := am.db.Query(
		"SELECT id, username, password, role, email, created_at, last_login FROM users ORDER BY created_at DESC",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		user := &User{}
		var lastLogin sql.NullTime

		err := rows.Scan(&user.ID, &user.Username, &user.Password, &user.Role, &user.Email, &user.CreatedAt, &lastLogin)
		if err != nil {
			continue
		}

		if lastLogin.Valid {
			user.LastLogin = lastLogin.Time
		}

		users = append(users, user)
	}

	return users, nil
}

// UpdateUser 更新用户
func (am *AuthManager) UpdateUser(id int64, req UpdateUserRequest) (*User, error) {
	// 检查用户是否存在
	user, err := am.GetUser(id)
	if err != nil {
		return nil, err
	}

	// 构建更新语句
	updates := []interface{}{}
	setClauses := []string{}

	if req.Username != "" && req.Username != user.Username {
		// 检查新用户名是否已存在
		var exists int
		err := am.db.QueryRow("SELECT COUNT(*) FROM users WHERE username = ? AND id != ?", req.Username, id).Scan(&exists)
		if err != nil {
			return nil, fmt.Errorf("failed to check username: %w", err)
		}
		if exists > 0 {
			return nil, fmt.Errorf("username already exists")
		}
		setClauses = append(setClauses, "username = ?")
		updates = append(updates, req.Username)
	}

	if req.Role != "" {
		if req.Role != RoleAdmin && req.Role != RoleViewer {
			return nil, fmt.Errorf("invalid role: %s", req.Role)
		}
		setClauses = append(setClauses, "role = ?")
		updates = append(updates, req.Role)
	}

	if req.Email != "" {
		setClauses = append(setClauses, "email = ?")
		updates = append(updates, req.Email)
	}

	if len(setClauses) == 0 {
		return user, nil
	}

	// 执行更新
	query := fmt.Sprintf("UPDATE users SET %s WHERE id = ?", joinClauses(setClauses, ", "))
	updates = append(updates, id)

	_, err = am.db.Exec(query, updates...)
	if err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	return am.GetUser(id)
}

// DeleteUser 删除用户
func (am *AuthManager) DeleteUser(id int64) error {
	// 检查是否是最后一个 admin
	var adminCount int
	err := am.db.QueryRow("SELECT COUNT(*) FROM users WHERE role = ?", RoleAdmin).Scan(&adminCount)
	if err != nil {
		return fmt.Errorf("failed to count admins: %w", err)
	}

	// 获取要删除的用户
	user, err := am.GetUser(id)
	if err != nil {
		return err
	}

	// 如果是最后一个 admin，禁止删除
	if user.Role == RoleAdmin && adminCount <= 1 {
		return fmt.Errorf("cannot delete the last admin user")
	}

	_, err = am.db.Exec("DELETE FROM users WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	return nil
}

// Authenticate 验证用户密码
func (am *AuthManager) Authenticate(username, password string) (*User, error) {
	user, err := am.GetUserByUsername(username)
	if err != nil {
		return nil, fmt.Errorf("invalid username or password")
	}

	// 验证密码
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		return nil, fmt.Errorf("invalid username or password")
	}

	// 更新最后登录时间
	_, _ = am.db.Exec("UPDATE users SET last_login = ? WHERE id = ?", time.Now(), user.ID)

	return user, nil
}

// ChangePassword 修改密码
func (am *AuthManager) ChangePassword(userID int64, req ChangePasswordRequest) error {
	user, err := am.GetUser(userID)
	if err != nil {
		return err
	}

	// 验证旧密码
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.OldPassword))
	if err != nil {
		return fmt.Errorf("incorrect old password")
	}

	// 哈希新密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// 更新密码
	_, err = am.db.Exec("UPDATE users SET password = ? WHERE id = ?", string(hashedPassword), userID)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	return nil
}

// ResetPassword 重置密码（admin only）
func (am *AuthManager) ResetPassword(userID int64, newPassword string) error {
	// 哈希新密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// 更新密码
	_, err = am.db.Exec("UPDATE users SET password = ? WHERE id = ?", string(hashedPassword), userID)
	if err != nil {
		return fmt.Errorf("failed to reset password: %w", err)
	}

	return nil
}

// joinClauses 辅助函数：连接 SQL 子句
func joinClauses(clauses []string, sep string) string {
	result := ""
	for i, clause := range clauses {
		if i > 0 {
			result += sep
		}
		result += clause
	}
	return result
}
