package checks

import (
	"fmt"
	"strings"
	"time"

	"github.com/netmonitor/backend/alerts"
)

// ---------------------------------------------------------------------------
// 10. NonStandardPortCheck — 非标准端口
// ---------------------------------------------------------------------------

// NonStandardPortCheck 常见服务运行在非标准端口
type NonStandardPortCheck struct{}

func (c *NonStandardPortCheck) Name() string { return "non_standard_port" }
func (c *NonStandardPortCheck) Description() string {
	return "Detect well-known protocols on non-standard ports"
}
func (c *NonStandardPortCheck) Type() alerts.AlertType                { return alerts.AlertTypeFlow }
func (c *NonStandardPortCheck) DefaultSeverity() alerts.AlertSeverity { return alerts.SeverityWarning }

// protocolStandardPorts maps L7 protocol names to their standard ports
var protocolStandardPorts = map[string][]uint16{
	"HTTP":  {80, 8080, 8000, 8443, 8888},
	"HTTPS": {443, 8443},
	"DNS":   {53},
	"SSH":   {22},
	"FTP":   {21},
	"SMTP":  {25, 465, 587},
	"IMAP":  {143, 993},
	"POP3":  {110, 995},
	"MySQL": {3306},
}

func (c *NonStandardPortCheck) Check(ctx *alerts.CheckContext) []alerts.Alert {
	var result []alerts.Alert
	for _, f := range ctx.Flows {
		if f.L7Protocol == "" || !f.IsActive {
			continue
		}
		upperProto := strings.ToUpper(f.L7Protocol)
		standardPorts, ok := protocolStandardPorts[upperProto]
		if !ok {
			continue
		}
		isStandard := false
		for _, p := range standardPorts {
			if f.DstPort == p || f.SrcPort == p {
				isStandard = true
				break
			}
		}
		if !isStandard {
			result = append(result, alerts.Alert{
				Type:        alerts.AlertTypeFlow,
				Severity:    alerts.SeverityWarning,
				Status:      alerts.StatusTriggered,
				RuleID:      "check_non_standard_port",
				Title:       "Non-Standard Port Usage",
				Description: fmt.Sprintf("%s traffic on port %d (flow %s: %s:%d -> %s:%d)", f.L7Protocol, f.DstPort, f.FlowID, f.SrcIP, f.SrcPort, f.DstIP, f.DstPort),
				EntityType:  "flow",
				EntityID:    f.FlowID,
				TriggeredAt: time.Now(),
			})
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// 11. CleartextCredentialCheck — 明文凭证协议
// ---------------------------------------------------------------------------

// CleartextCredentialCheck 检测 Telnet(23)、FTP(21) 等不安全协议使用
type CleartextCredentialCheck struct{}

func (c *CleartextCredentialCheck) Name() string { return "cleartext_credential" }
func (c *CleartextCredentialCheck) Description() string {
	return "Detect cleartext credential protocols (Telnet, FTP, etc.)"
}
func (c *CleartextCredentialCheck) Type() alerts.AlertType { return alerts.AlertTypeFlow }
func (c *CleartextCredentialCheck) DefaultSeverity() alerts.AlertSeverity {
	return alerts.SeverityError
}

var insecurePorts = map[uint16]string{
	21: "FTP",
	23: "Telnet",
	69: "TFTP",
}

func (c *CleartextCredentialCheck) Check(ctx *alerts.CheckContext) []alerts.Alert {
	var result []alerts.Alert
	for _, f := range ctx.Flows {
		if !f.IsActive {
			continue
		}
		if proto, ok := insecurePorts[f.DstPort]; ok {
			result = append(result, alerts.Alert{
				Type:        alerts.AlertTypeFlow,
				Severity:    alerts.SeverityError,
				Status:      alerts.StatusTriggered,
				RuleID:      "check_cleartext_credential",
				Title:       "Cleartext Credential Protocol Detected",
				Description: fmt.Sprintf("%s (port %d) detected: %s:%d -> %s:%d — credentials may be transmitted in cleartext", proto, f.DstPort, f.SrcIP, f.SrcPort, f.DstIP, f.DstPort),
				EntityType:  "flow",
				EntityID:    f.FlowID,
				TriggeredAt: time.Now(),
			})
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// 12. DNSAnomalyCheck — DNS 异常
// ---------------------------------------------------------------------------

// DNSAnomalyCheck DNS 查询频率异常高（单主机 > 100 qps）
type DNSAnomalyCheck struct{}

func (c *DNSAnomalyCheck) Name() string { return "dns_anomaly" }
func (c *DNSAnomalyCheck) Description() string {
	return "Detect DNS query anomalies (>100 DNS flows from one host)"
}
func (c *DNSAnomalyCheck) Type() alerts.AlertType                { return alerts.AlertTypeHost }
func (c *DNSAnomalyCheck) DefaultSeverity() alerts.AlertSeverity { return alerts.SeverityWarning }

const dnsAnomalyThreshold = 100

func (c *DNSAnomalyCheck) Check(ctx *alerts.CheckContext) []alerts.Alert {
	// Count DNS flows per source host
	dnsFlowCount := make(map[string]int)
	for _, f := range ctx.Flows {
		if f.DstPort == 53 || f.SrcPort == 53 ||
			strings.EqualFold(f.L7Protocol, "DNS") {
			dnsFlowCount[f.SrcIP]++
		}
	}

	var result []alerts.Alert
	for ip, count := range dnsFlowCount {
		if count > dnsAnomalyThreshold {
			result = append(result, alerts.Alert{
				Type:        alerts.AlertTypeHost,
				Severity:    alerts.SeverityWarning,
				Status:      alerts.StatusTriggered,
				RuleID:      "check_dns_anomaly",
				Title:       "DNS Anomaly Detected",
				Description: fmt.Sprintf("Host %s has %d DNS flows (threshold: %d) — possible DNS tunneling or exfiltration", ip, count, dnsAnomalyThreshold),
				EntityType:  "host",
				EntityID:    ip,
				TriggeredAt: time.Now(),
			})
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// 13. UnknownProtocolCheck — 未知协议
// ---------------------------------------------------------------------------

// UnknownProtocolCheck 大量流量使用无法识别的协议
type UnknownProtocolCheck struct{}

func (c *UnknownProtocolCheck) Name() string { return "unknown_protocol" }
func (c *UnknownProtocolCheck) Description() string {
	return "Detect significant traffic with unknown L7 protocol"
}
func (c *UnknownProtocolCheck) Type() alerts.AlertType                { return alerts.AlertTypeFlow }
func (c *UnknownProtocolCheck) DefaultSeverity() alerts.AlertSeverity { return alerts.SeverityWarning }

const unknownProtocolBytesThreshold uint64 = 10 * 1024 * 1024 // 10MB

func (c *UnknownProtocolCheck) Check(ctx *alerts.CheckContext) []alerts.Alert {
	var result []alerts.Alert
	for _, f := range ctx.Flows {
		if !f.IsActive || f.L7Protocol != "" {
			continue
		}
		totalBytes := f.BytesSent + f.BytesRecv
		if totalBytes > unknownProtocolBytesThreshold {
			result = append(result, alerts.Alert{
				Type:        alerts.AlertTypeFlow,
				Severity:    alerts.SeverityWarning,
				Status:      alerts.StatusTriggered,
				RuleID:      "check_unknown_protocol",
				Title:       "Unknown Protocol with Significant Traffic",
				Description: fmt.Sprintf("Flow %s (%s:%d -> %s:%d) has %d MB of unidentified protocol traffic", f.FlowID, f.SrcIP, f.SrcPort, f.DstIP, f.DstPort, totalBytes/(1024*1024)),
				EntityType:  "flow",
				EntityID:    f.FlowID,
				TriggeredAt: time.Now(),
			})
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// 14. EncryptedTrafficRatioCheck — 加密流量比例异常低
// ---------------------------------------------------------------------------

// EncryptedTrafficRatioCheck 网络中加密流量占比 < 30%
type EncryptedTrafficRatioCheck struct{}

func (c *EncryptedTrafficRatioCheck) Name() string { return "encrypted_traffic_ratio" }
func (c *EncryptedTrafficRatioCheck) Description() string {
	return "Detect low encrypted traffic ratio (<30%)"
}
func (c *EncryptedTrafficRatioCheck) Type() alerts.AlertType { return alerts.AlertTypeSystem }
func (c *EncryptedTrafficRatioCheck) DefaultSeverity() alerts.AlertSeverity {
	return alerts.SeverityWarning
}

const encryptedRatioThreshold = 0.30

// encryptedProtocols lists protocols considered encrypted
var encryptedProtocols = map[string]bool{
	"TLS": true, "SSL": true, "HTTPS": true,
	"SSH": true, "QUIC": true, "DTLS": true,
	"IMAPS": true, "POP3S": true, "SMTPS": true,
}

func (c *EncryptedTrafficRatioCheck) Check(ctx *alerts.CheckContext) []alerts.Alert {
	if len(ctx.Flows) == 0 {
		return nil
	}

	var totalBytes, encryptedBytes uint64
	for _, f := range ctx.Flows {
		b := f.BytesSent + f.BytesRecv
		totalBytes += b
		upper := strings.ToUpper(f.L7Protocol)
		if encryptedProtocols[upper] || f.DstPort == 443 || f.SrcPort == 443 {
			encryptedBytes += b
		}
	}

	if totalBytes == 0 {
		return nil
	}

	ratio := float64(encryptedBytes) / float64(totalBytes)
	if ratio < encryptedRatioThreshold {
		return []alerts.Alert{{
			Type:        alerts.AlertTypeSystem,
			Severity:    alerts.SeverityWarning,
			Status:      alerts.StatusTriggered,
			RuleID:      "check_encrypted_traffic_ratio",
			Title:       "Low Encrypted Traffic Ratio",
			Description: fmt.Sprintf("Only %.1f%% of traffic is encrypted (threshold: %.0f%%)", ratio*100, encryptedRatioThreshold*100),
			EntityType:  "system",
			EntityID:    "system",
			TriggeredAt: time.Now(),
		}}
	}
	return nil
}
