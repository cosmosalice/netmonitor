package checks

import (
	"fmt"
	"time"

	"github.com/netmonitor/backend/alerts"
)

// ---------------------------------------------------------------------------
// 5. PortScanCheck — 端口扫描检测
// ---------------------------------------------------------------------------

// PortScanCheck 单主机在短时间内连接超过 50 个不同端口
type PortScanCheck struct{}

func (c *PortScanCheck) Name() string { return "port_scan" }
func (c *PortScanCheck) Description() string {
	return "Detect port scanning (>50 unique dst ports from one host)"
}
func (c *PortScanCheck) Type() alerts.AlertType                { return alerts.AlertTypeHost }
func (c *PortScanCheck) DefaultSeverity() alerts.AlertSeverity { return alerts.SeverityCritical }

const portScanThreshold = 50

func (c *PortScanCheck) Check(ctx *alerts.CheckContext) []alerts.Alert {
	// Count unique destination ports per source IP (active flows only)
	hostPorts := make(map[string]map[uint16]struct{})
	for _, f := range ctx.Flows {
		if !f.IsActive {
			continue
		}
		if _, ok := hostPorts[f.SrcIP]; !ok {
			hostPorts[f.SrcIP] = make(map[uint16]struct{})
		}
		hostPorts[f.SrcIP][f.DstPort] = struct{}{}
	}

	var result []alerts.Alert
	for ip, ports := range hostPorts {
		if len(ports) > portScanThreshold {
			result = append(result, alerts.Alert{
				Type:        alerts.AlertTypeHost,
				Severity:    alerts.SeverityCritical,
				Status:      alerts.StatusTriggered,
				RuleID:      "check_port_scan",
				Title:       "Port Scan Detected",
				Description: fmt.Sprintf("Host %s is connecting to %d unique destination ports", ip, len(ports)),
				EntityType:  "host",
				EntityID:    ip,
				TriggeredAt: time.Now(),
			})
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// 6. SYNFloodCheck — SYN Flood 检测
// ---------------------------------------------------------------------------

// SYNFloodCheck 单主机大量半开连接（高 SYN 包率）
type SYNFloodCheck struct{}

func (c *SYNFloodCheck) Name() string { return "syn_flood" }
func (c *SYNFloodCheck) Description() string {
	return "Detect SYN flood attacks (many TCP flows with minimal data)"
}
func (c *SYNFloodCheck) Type() alerts.AlertType                { return alerts.AlertTypeHost }
func (c *SYNFloodCheck) DefaultSeverity() alerts.AlertSeverity { return alerts.SeverityCritical }

const synFloodFlowThreshold = 100
const synFloodBytesThreshold uint64 = 256 // minimal data per flow = likely SYN-only

func (c *SYNFloodCheck) Check(ctx *alerts.CheckContext) []alerts.Alert {
	// Count TCP flows per source IP where total bytes are very small (half-open)
	type hostStat struct {
		totalTCPFlows int
		halfOpenFlows int
	}
	stats := make(map[string]*hostStat)

	for _, f := range ctx.Flows {
		if f.Protocol != "TCP" || !f.IsActive {
			continue
		}
		if _, ok := stats[f.SrcIP]; !ok {
			stats[f.SrcIP] = &hostStat{}
		}
		s := stats[f.SrcIP]
		s.totalTCPFlows++
		if f.BytesSent+f.BytesRecv < synFloodBytesThreshold {
			s.halfOpenFlows++
		}
	}

	var result []alerts.Alert
	for ip, s := range stats {
		if s.halfOpenFlows > synFloodFlowThreshold {
			result = append(result, alerts.Alert{
				Type:        alerts.AlertTypeHost,
				Severity:    alerts.SeverityCritical,
				Status:      alerts.StatusTriggered,
				RuleID:      "check_syn_flood",
				Title:       "SYN Flood Detected",
				Description: fmt.Sprintf("Host %s has %d half-open TCP connections out of %d total TCP flows", ip, s.halfOpenFlows, s.totalTCPFlows),
				EntityType:  "host",
				EntityID:    ip,
				TriggeredAt: time.Now(),
			})
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// 7. HighConnectionCountCheck — 异常连接数
// ---------------------------------------------------------------------------

// HighConnectionCountCheck 单主机活跃 Flow 超过 500
type HighConnectionCountCheck struct{}

func (c *HighConnectionCountCheck) Name() string { return "high_connection_count" }
func (c *HighConnectionCountCheck) Description() string {
	return "Detect hosts with excessive active connections (>500)"
}
func (c *HighConnectionCountCheck) Type() alerts.AlertType { return alerts.AlertTypeHost }
func (c *HighConnectionCountCheck) DefaultSeverity() alerts.AlertSeverity {
	return alerts.SeverityWarning
}

const highConnectionThreshold = 500

func (c *HighConnectionCountCheck) Check(ctx *alerts.CheckContext) []alerts.Alert {
	for _, h := range ctx.Hosts {
		if h.FlowCount > highConnectionThreshold {
			return append([]alerts.Alert{}, alerts.Alert{
				Type:        alerts.AlertTypeHost,
				Severity:    alerts.SeverityWarning,
				Status:      alerts.StatusTriggered,
				RuleID:      "check_high_connection_count",
				Title:       "High Connection Count",
				Description: fmt.Sprintf("Host %s has %d active flows (threshold: %d)", h.IP, h.FlowCount, highConnectionThreshold),
				EntityType:  "host",
				EntityID:    h.IP,
				TriggeredAt: time.Now(),
			})
		}
	}

	// Also check from flows data
	hostFlowCount := make(map[string]int)
	for _, f := range ctx.Flows {
		if f.IsActive {
			hostFlowCount[f.SrcIP]++
			hostFlowCount[f.DstIP]++
		}
	}
	var result []alerts.Alert
	for ip, count := range hostFlowCount {
		if count > highConnectionThreshold {
			result = append(result, alerts.Alert{
				Type:        alerts.AlertTypeHost,
				Severity:    alerts.SeverityWarning,
				Status:      alerts.StatusTriggered,
				RuleID:      "check_high_connection_count",
				Title:       "High Connection Count",
				Description: fmt.Sprintf("Host %s participates in %d active flows (threshold: %d)", ip, count, highConnectionThreshold),
				EntityType:  "host",
				EntityID:    ip,
				TriggeredAt: time.Now(),
			})
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// 8. DataExfiltrationCheck — 数据外泄检测
// ---------------------------------------------------------------------------

// DataExfiltrationCheck 单主机发送流量远大于接收（比例 > 10:1 且超过 50MB）
type DataExfiltrationCheck struct{}

func (c *DataExfiltrationCheck) Name() string { return "data_exfiltration" }
func (c *DataExfiltrationCheck) Description() string {
	return "Detect potential data exfiltration (sent/recv ratio >10:1 and >50MB sent)"
}
func (c *DataExfiltrationCheck) Type() alerts.AlertType { return alerts.AlertTypeHost }
func (c *DataExfiltrationCheck) DefaultSeverity() alerts.AlertSeverity {
	return alerts.SeverityCritical
}

const dataExfilRatio float64 = 10.0
const dataExfilMinBytes uint64 = 50 * 1024 * 1024 // 50MB

func (c *DataExfiltrationCheck) Check(ctx *alerts.CheckContext) []alerts.Alert {
	var result []alerts.Alert
	for _, h := range ctx.Hosts {
		if h.BytesSent > dataExfilMinBytes && h.BytesRecv > 0 {
			ratio := float64(h.BytesSent) / float64(h.BytesRecv)
			if ratio > dataExfilRatio {
				result = append(result, alerts.Alert{
					Type:        alerts.AlertTypeHost,
					Severity:    alerts.SeverityCritical,
					Status:      alerts.StatusTriggered,
					RuleID:      "check_data_exfiltration",
					Title:       "Potential Data Exfiltration",
					Description: fmt.Sprintf("Host %s sent %d MB with send/recv ratio %.1f:1 (threshold: %.0f:1, min %d MB)", h.IP, h.BytesSent/(1024*1024), ratio, dataExfilRatio, dataExfilMinBytes/(1024*1024)),
					EntityType:  "host",
					EntityID:    h.IP,
					TriggeredAt: time.Now(),
				})
			}
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// 9. NewHostCheck — 新主机发现
// ---------------------------------------------------------------------------

// NewHostCheck 首次出现的主机（FirstSeen 在最近 5 分钟内）
type NewHostCheck struct{}

func (c *NewHostCheck) Name() string { return "new_host" }
func (c *NewHostCheck) Description() string {
	return "Detect newly discovered hosts (first seen within 5 minutes)"
}
func (c *NewHostCheck) Type() alerts.AlertType                { return alerts.AlertTypeHost }
func (c *NewHostCheck) DefaultSeverity() alerts.AlertSeverity { return alerts.SeverityInfo }

const newHostWindow = 5 * time.Minute

func (c *NewHostCheck) Check(ctx *alerts.CheckContext) []alerts.Alert {
	var result []alerts.Alert
	now := time.Now()
	for _, h := range ctx.Hosts {
		if now.Sub(h.FirstSeen) < newHostWindow {
			result = append(result, alerts.Alert{
				Type:        alerts.AlertTypeHost,
				Severity:    alerts.SeverityInfo,
				Status:      alerts.StatusTriggered,
				RuleID:      "check_new_host",
				Title:       "New Host Discovered",
				Description: fmt.Sprintf("Host %s first seen at %s", h.IP, h.FirstSeen.Format(time.RFC3339)),
				EntityType:  "host",
				EntityID:    h.IP,
				TriggeredAt: time.Now(),
			})
		}
	}
	return result
}
