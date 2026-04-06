package integrations

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/netmonitor/backend/alerts"
)

// SyslogConfig Syslog 配置
type SyslogConfig struct {
	Target   string `json:"target"`   // 目标地址
	Port     int    `json:"port"`     // 端口
	Protocol string `json:"protocol"` // udp 或 tcp
	Facility int    `json:"facility"` // syslog facility (0-23)
	Severity int    `json:"severity"` // 默认 severity (0-7)
	UseTLS   bool   `json:"use_tls"`  // 是否使用 TLS (仅 TCP)
	Enabled  bool   `json:"enabled"`  // 是否启用
}

// SyslogForwarder Syslog 告警转发器
type SyslogForwarder struct {
	config     SyslogConfig
	conn       net.Conn
	mu         sync.RWMutex
	running    bool
	stopCh     chan struct{}
	retryCount int
	lastError  string
}

// NewSyslogForwarder 创建新的 Syslog 转发器
func NewSyslogForwarder(config SyslogConfig) *SyslogForwarder {
	return &SyslogForwarder{
		config: config,
		stopCh: make(chan struct{}),
	}
}

// Start 启动转发器
func (s *SyslogForwarder) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	if !s.config.Enabled {
		log.Println("Syslog forwarder is disabled")
		return nil
	}

	s.running = true
	s.stopCh = make(chan struct{})

	// 尝试建立连接
	if err := s.connect(); err != nil {
		log.Printf("Syslog initial connection failed: %v, will retry", err)
		s.lastError = err.Error()
	}

	log.Println("Syslog forwarder started")
	return nil
}

// Stop 停止转发器
func (s *SyslogForwarder) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.running = false
	close(s.stopCh)

	if s.conn != nil {
		s.conn.Close()
		s.conn = nil
	}

	log.Println("Syslog forwarder stopped")
	return nil
}

// connect 建立连接
func (s *SyslogForwarder) connect() error {
	if s.conn != nil {
		s.conn.Close()
		s.conn = nil
	}

	addr := fmt.Sprintf("%s:%d", s.config.Target, s.config.Port)

	var conn net.Conn
	var err error

	if s.config.Protocol == "tcp" {
		if s.config.UseTLS {
			conn, err = tls.Dial("tcp", addr, &tls.Config{
				InsecureSkipVerify: true, // 生产环境应该验证证书
			})
		} else {
			conn, err = net.Dial("tcp", addr)
		}
	} else {
		conn, err = net.Dial("udp", addr)
	}

	if err != nil {
		return err
	}

	s.conn = conn
	s.retryCount = 0
	s.lastError = ""
	log.Printf("Syslog connected to %s", addr)
	return nil
}

// reconnect 重新连接（带重试）
func (s *SyslogForwarder) reconnect() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	maxRetries := 5
	if s.retryCount >= maxRetries {
		s.lastError = fmt.Sprintf("Max retries (%d) exceeded", maxRetries)
		return
	}

	s.retryCount++
	delay := time.Duration(s.retryCount) * time.Second

	log.Printf("Syslog reconnecting in %v (attempt %d/%d)", delay, s.retryCount, maxRetries)

	// 异步延迟重连
	go func() {
		time.Sleep(delay)
		s.mu.Lock()
		defer s.mu.Unlock()

		if !s.running {
			return
		}

		if err := s.connect(); err != nil {
			s.lastError = err.Error()
		}
	}()
}

// Forward 转发告警
func (s *SyslogForwarder) Forward(alert *alerts.Alert) error {
	s.mu.RLock()
	if !s.running || !s.config.Enabled {
		s.mu.RUnlock()
		return nil
	}
	conn := s.conn
	s.mu.RUnlock()

	if conn == nil {
		// 尝试重新连接
		s.reconnect()
		return fmt.Errorf("syslog not connected")
	}

	// 构建 RFC5424 syslog 消息
	msg := s.formatRFC5424(alert)

	// 发送消息
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn == nil {
		return fmt.Errorf("syslog connection lost")
	}

	_, err := s.conn.Write([]byte(msg))
	if err != nil {
		s.lastError = err.Error()
		go s.reconnect()
		return err
	}

	return nil
}

// formatRFC5424 格式化为 RFC5424 标准 syslog 消息
func (s *SyslogForwarder) formatRFC5424(alert *alerts.Alert) string {
	// PRI = facility * 8 + severity
	pri := s.config.Facility*8 + s.config.Severity

	timestamp := alert.TriggeredAt.Format(time.RFC3339)
	hostname := "netmonitor"
	appName := "netmonitor"
	procID := "-"
	msgID := alert.RuleID
	if msgID == "" {
		msgID = "-"
	}

	// 结构化数据（可选）
	structuredData := "-"

	// 构建消息内容
	content := fmt.Sprintf("[%s] %s: %s", alert.Severity, alert.Title, alert.Description)

	// 添加元数据
	metadata := map[string]interface{}{
		"type":        alert.Type,
		"entity_type": alert.EntityType,
		"entity_id":   alert.EntityID,
		"status":      alert.Status,
	}
	if alert.Metadata != "" {
		metadata["metadata"] = alert.Metadata
	}

	metaJSON, _ := json.Marshal(metadata)
	content = fmt.Sprintf("%s | %s", content, string(metaJSON))

	// RFC5424 格式: <PRI>VERSION TIMESTAMP HOSTNAME APP-NAME PROCID MSGID STRUCTURED-DATA MSG
	return fmt.Sprintf("<%d>1 %s %s %s %s %s %s %s\n",
		pri, timestamp, hostname, appName, procID, msgID, structuredData, content)
}

// GetStatus 获取转发器状态
func (s *SyslogForwarder) GetStatus() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	connected := s.conn != nil
	return map[string]interface{}{
		"enabled":     s.config.Enabled,
		"running":     s.running,
		"connected":   connected,
		"target":      fmt.Sprintf("%s:%d", s.config.Target, s.config.Port),
		"protocol":    s.config.Protocol,
		"retry_count": s.retryCount,
		"last_error":  s.lastError,
	}
}

// UpdateConfig 更新配置
func (s *SyslogForwarder) UpdateConfig(config SyslogConfig) error {
	wasRunning := s.running

	// 停止当前转发器
	if wasRunning {
		s.Stop()
	}

	// 更新配置
	s.mu.Lock()
	s.config = config
	s.retryCount = 0
	s.mu.Unlock()

	// 如果之前正在运行，重新启动
	if wasRunning && config.Enabled {
		return s.Start()
	}

	return nil
}

// TestConnection 测试连接
func (s *SyslogForwarder) TestConnection() error {
	addr := fmt.Sprintf("%s:%d", s.config.Target, s.config.Port)

	var conn net.Conn
	var err error

	if s.config.Protocol == "tcp" {
		if s.config.UseTLS {
			conn, err = tls.Dial("tcp", addr, &tls.Config{
				InsecureSkipVerify: true,
			})
		} else {
			conn, err = net.Dial("tcp", addr)
		}
	} else {
		conn, err = net.Dial("udp", addr)
	}

	if err != nil {
		return err
	}

	conn.Close()
	return nil
}
