package alerts

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// Notifier 通知器接口
type Notifier interface {
	Name() string
	Type() string // "websocket", "email", "webhook"
	Send(alert *Alert) error
	IsEnabled() bool
}

// NotificationManager 通知管理器
type NotificationManager struct {
	mu        sync.RWMutex
	notifiers []Notifier
	config    *NotificationConfig
	db        *sql.DB
}

// NotificationConfig 通知配置
type NotificationConfig struct {
	Endpoints []NotificationEndpoint `json:"endpoints"`
}

// NotificationEndpoint 通知端点
type NotificationEndpoint struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Type      string            `json:"type"` // "email", "webhook", "websocket"
	Enabled   bool              `json:"enabled"`
	Config    map[string]string `json:"config"` // 配置参数
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// NewNotificationManager 创建通知管理器
func NewNotificationManager(db *sql.DB) *NotificationManager {
	nm := &NotificationManager{
		notifiers: make([]Notifier, 0),
		config:    &NotificationConfig{},
		db:        db,
	}

	// Load endpoints from database
	if err := nm.LoadEndpoints(); err != nil {
		log.Printf("[NotificationManager] failed to load endpoints: %v", err)
	}

	return nm
}

// RegisterNotifier 注册通知器
func (nm *NotificationManager) RegisterNotifier(n Notifier) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	nm.notifiers = append(nm.notifiers, n)
	log.Printf("[NotificationManager] registered notifier: %s (%s)", n.Name(), n.Type())
}

// NotifyAll 向所有启用的通知器发送告警（异步，不阻塞调用方）
func (nm *NotificationManager) NotifyAll(alert *Alert) {
	nm.mu.RLock()
	notifiers := make([]Notifier, len(nm.notifiers))
	copy(notifiers, nm.notifiers)
	nm.mu.RUnlock()

	for _, n := range notifiers {
		if !n.IsEnabled() {
			continue
		}
		// 异步发送，避免阻塞告警写入
		go func(notifier Notifier) {
			if err := notifier.Send(alert); err != nil {
				log.Printf("[NotificationManager] notifier %s failed: %v", notifier.Name(), err)
			}
		}(n)
	}
}

// GetEndpoints 获取配置的端点
func (nm *NotificationManager) GetEndpoints() []NotificationEndpoint {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	eps := make([]NotificationEndpoint, len(nm.config.Endpoints))
	copy(eps, nm.config.Endpoints)
	return eps
}

// SaveEndpoint 保存端点配置到数据库
func (nm *NotificationManager) SaveEndpoint(ep NotificationEndpoint) error {
	configJSON, err := json.Marshal(ep.Config)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	now := time.Now()
	_, err = nm.db.Exec(`
		INSERT INTO notification_endpoints (id, name, type, enabled, config_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			type = excluded.type,
			enabled = excluded.enabled,
			config_json = excluded.config_json,
			updated_at = excluded.updated_at
	`, ep.ID, ep.Name, ep.Type, boolToInt(ep.Enabled), string(configJSON), now, now)
	if err != nil {
		return fmt.Errorf("save endpoint: %w", err)
	}

	// Reload from DB
	return nm.LoadEndpoints()
}

// DeleteEndpoint 删除端点
func (nm *NotificationManager) DeleteEndpoint(id string) error {
	result, err := nm.db.Exec("DELETE FROM notification_endpoints WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete endpoint: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("endpoint %s not found", id)
	}

	return nm.LoadEndpoints()
}

// LoadEndpoints 从数据库加载端点配置
func (nm *NotificationManager) LoadEndpoints() error {
	rows, err := nm.db.Query(`
		SELECT id, name, type, enabled, config_json, created_at, updated_at
		FROM notification_endpoints
		ORDER BY created_at ASC
	`)
	if err != nil {
		return fmt.Errorf("query endpoints: %w", err)
	}
	defer rows.Close()

	var endpoints []NotificationEndpoint
	for rows.Next() {
		var ep NotificationEndpoint
		var enabled int
		var configJSON string
		if err := rows.Scan(&ep.ID, &ep.Name, &ep.Type, &enabled, &configJSON, &ep.CreatedAt, &ep.UpdatedAt); err != nil {
			return fmt.Errorf("scan endpoint: %w", err)
		}
		ep.Enabled = enabled != 0
		ep.Config = make(map[string]string)
		if configJSON != "" {
			if err := json.Unmarshal([]byte(configJSON), &ep.Config); err != nil {
				log.Printf("[NotificationManager] failed to parse config for endpoint %s: %v", ep.ID, err)
			}
		}
		endpoints = append(endpoints, ep)
	}

	nm.mu.Lock()
	nm.config.Endpoints = endpoints
	nm.mu.Unlock()

	log.Printf("[NotificationManager] loaded %d endpoints", len(endpoints))
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
