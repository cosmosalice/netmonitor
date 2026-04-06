package notifiers

import (
	"encoding/json"
	"log"

	"github.com/netmonitor/backend/alerts"
)

// Broadcaster 广播接口，用于解耦 WebSocket hub 实现
type Broadcaster interface {
	Broadcast(msg []byte)
}

// WebSocketNotifier 应用内 WebSocket 通知
type WebSocketNotifier struct {
	hub     Broadcaster
	enabled bool
}

// NewWebSocketNotifier 创建 WebSocket 通知器
func NewWebSocketNotifier(hub Broadcaster) *WebSocketNotifier {
	return &WebSocketNotifier{
		hub:     hub,
		enabled: true,
	}
}

func (n *WebSocketNotifier) Name() string { return "websocket" }
func (n *WebSocketNotifier) Type() string { return "websocket" }

func (n *WebSocketNotifier) IsEnabled() bool { return n.enabled }

// SetEnabled 设置启用状态
func (n *WebSocketNotifier) SetEnabled(enabled bool) {
	n.enabled = enabled
}

// Send 通过 WebSocket 推送告警到前端
func (n *WebSocketNotifier) Send(alert *alerts.Alert) error {
	msg := map[string]interface{}{
		"type": "alert",
		"data": map[string]interface{}{
			"id":           alert.ID,
			"severity":     alert.Severity,
			"title":        alert.Title,
			"description":  alert.Description,
			"type":         alert.Type,
			"status":       alert.Status,
			"entity_type":  alert.EntityType,
			"entity_id":    alert.EntityID,
			"triggered_at": alert.TriggeredAt,
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	n.hub.Broadcast(data)
	log.Printf("[WebSocketNotifier] alert pushed: %s", alert.Title)
	return nil
}
