package notifiers

import (
	"fmt"
	"log"
	"net/smtp"
	"strings"

	"github.com/netmonitor/backend/alerts"
)

// EmailNotifier 邮件通知
type EmailNotifier struct {
	config  EmailConfig
	enabled bool
}

// EmailConfig 邮件配置
type EmailConfig struct {
	SMTPHost    string   `json:"smtp_host"`
	SMTPPort    int      `json:"smtp_port"`
	Username    string   `json:"username"`
	Password    string   `json:"password"`
	FromAddress string   `json:"from_address"`
	ToAddresses []string `json:"to_addresses"`
	UseTLS      bool     `json:"use_tls"`
}

// NewEmailNotifier 创建邮件通知器
func NewEmailNotifier(config EmailConfig) *EmailNotifier {
	return &EmailNotifier{
		config:  config,
		enabled: config.SMTPHost != "" && len(config.ToAddresses) > 0,
	}
}

func (n *EmailNotifier) Name() string { return "email" }
func (n *EmailNotifier) Type() string { return "email" }

func (n *EmailNotifier) IsEnabled() bool { return n.enabled }

// SetEnabled 设置启用状态
func (n *EmailNotifier) SetEnabled(enabled bool) {
	n.enabled = enabled
}

// SetConfig 更新配置
func (n *EmailNotifier) SetConfig(config EmailConfig) {
	n.config = config
	n.enabled = config.SMTPHost != "" && len(config.ToAddresses) > 0
}

// Send 发送邮件通知
func (n *EmailNotifier) Send(alert *alerts.Alert) error {
	if n.config.SMTPHost == "" || len(n.config.ToAddresses) == 0 {
		return fmt.Errorf("email notifier not configured")
	}

	subject := fmt.Sprintf("[NetMonitor Alert] [%s] %s", strings.ToUpper(string(alert.Severity)), alert.Title)

	body := buildHTMLBody(alert)

	// Build email message
	to := strings.Join(n.config.ToAddresses, ",")
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		n.config.FromAddress, to, subject, body)

	addr := fmt.Sprintf("%s:%d", n.config.SMTPHost, n.config.SMTPPort)

	var auth smtp.Auth
	if n.config.Username != "" {
		auth = smtp.PlainAuth("", n.config.Username, n.config.Password, n.config.SMTPHost)
	}

	err := smtp.SendMail(addr, auth, n.config.FromAddress, n.config.ToAddresses, []byte(msg))
	if err != nil {
		return fmt.Errorf("send email: %w", err)
	}

	log.Printf("[EmailNotifier] alert email sent: %s", alert.Title)
	return nil
}

func buildHTMLBody(alert *alerts.Alert) string {
	severityColor := "#999"
	switch alert.Severity {
	case alerts.SeverityCritical:
		severityColor = "#dc3545"
	case alerts.SeverityError:
		severityColor = "#fd7e14"
	case alerts.SeverityWarning:
		severityColor = "#ffc107"
	case alerts.SeverityInfo:
		severityColor = "#17a2b8"
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"></head>
<body style="font-family: Arial, sans-serif; margin: 0; padding: 20px; background: #f5f5f5;">
  <div style="max-width: 600px; margin: 0 auto; background: white; border-radius: 8px; overflow: hidden; box-shadow: 0 2px 8px rgba(0,0,0,0.1);">
    <div style="background: %s; color: white; padding: 16px 24px;">
      <h2 style="margin: 0;">NetMonitor Alert</h2>
    </div>
    <div style="padding: 24px;">
      <table style="width: 100%%; border-collapse: collapse;">
        <tr><td style="padding: 8px 0; font-weight: bold; width: 120px;">Type:</td><td>%s</td></tr>
        <tr><td style="padding: 8px 0; font-weight: bold;">Severity:</td><td><span style="color: %s; font-weight: bold;">%s</span></td></tr>
        <tr><td style="padding: 8px 0; font-weight: bold;">Title:</td><td>%s</td></tr>
        <tr><td style="padding: 8px 0; font-weight: bold;">Description:</td><td>%s</td></tr>
        <tr><td style="padding: 8px 0; font-weight: bold;">Time:</td><td>%s</td></tr>
        <tr><td style="padding: 8px 0; font-weight: bold;">Entity:</td><td>%s (%s)</td></tr>
      </table>
    </div>
    <div style="padding: 12px 24px; background: #f8f9fa; text-align: center; color: #666; font-size: 12px;">
      This is an automated alert from NetMonitor.
    </div>
  </div>
</body>
</html>`,
		severityColor,
		alert.Type,
		severityColor, alert.Severity,
		alert.Title,
		alert.Description,
		alert.TriggeredAt.Format("2006-01-02 15:04:05"),
		alert.EntityType, alert.EntityID,
	)
}
