package notifiers

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/netmonitor/backend/alerts"
)

// WebhookNotifier Webhook 通知
type WebhookNotifier struct {
	config  WebhookConfig
	enabled bool
	client  *http.Client
}

// WebhookConfig Webhook 配置
type WebhookConfig struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"` // "POST" 默认
	Headers map[string]string `json:"headers"`
	Secret  string            `json:"secret,omitempty"` // HMAC 签名密钥
}

// NewWebhookNotifier 创建 Webhook 通知器
func NewWebhookNotifier(config WebhookConfig) *WebhookNotifier {
	if config.Method == "" {
		config.Method = "POST"
	}
	return &WebhookNotifier{
		config:  config,
		enabled: config.URL != "",
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (n *WebhookNotifier) Name() string { return "webhook" }
func (n *WebhookNotifier) Type() string { return "webhook" }

func (n *WebhookNotifier) IsEnabled() bool { return n.enabled }

// SetEnabled 设置启用状态
func (n *WebhookNotifier) SetEnabled(enabled bool) {
	n.enabled = enabled
}

// SetConfig 更新配置
func (n *WebhookNotifier) SetConfig(config WebhookConfig) {
	if config.Method == "" {
		config.Method = "POST"
	}
	n.config = config
	n.enabled = config.URL != ""
}

// Send 通过 Webhook 发送告警
func (n *WebhookNotifier) Send(alert *alerts.Alert) error {
	if n.config.URL == "" {
		return fmt.Errorf("webhook URL not configured")
	}

	payload := map[string]interface{}{
		"event": "alert",
		"alert": map[string]interface{}{
			"id":           alert.ID,
			"type":         alert.Type,
			"severity":     alert.Severity,
			"status":       alert.Status,
			"title":        alert.Title,
			"description":  alert.Description,
			"entity_type":  alert.EntityType,
			"entity_id":    alert.EntityID,
			"triggered_at": alert.TriggeredAt,
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequest(n.config.Method, n.config.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "NetMonitor-Webhook/1.0")

	// Apply custom headers
	for k, v := range n.config.Headers {
		req.Header.Set(k, v)
	}

	// HMAC-SHA256 signature
	if n.config.Secret != "" {
		mac := hmac.New(sha256.New, []byte(n.config.Secret))
		mac.Write(body)
		signature := hex.EncodeToString(mac.Sum(nil))
		req.Header.Set("X-Signature", "sha256="+signature)
	}

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	log.Printf("[WebhookNotifier] alert sent to %s: %s (status %d)", n.config.URL, alert.Title, resp.StatusCode)
	return nil
}
