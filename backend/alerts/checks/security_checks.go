package checks

import (
	"fmt"
	"time"

	"github.com/netmonitor/backend/alerts"
)

// ---------------------------------------------------------------------------
// 19. ICMPFloodCheck — ICMP Flood
// ---------------------------------------------------------------------------

// ICMPFloodCheck 大量 ICMP 包（> 1000 pps 来自单主机）
type ICMPFloodCheck struct{}

func (c *ICMPFloodCheck) Name() string { return "icmp_flood" }
func (c *ICMPFloodCheck) Description() string {
	return "Detect ICMP flood attacks (>1000 ICMP packets from one host)"
}
func (c *ICMPFloodCheck) Type() alerts.AlertType                { return alerts.AlertTypeHost }
func (c *ICMPFloodCheck) DefaultSeverity() alerts.AlertSeverity { return alerts.SeverityCritical }

const icmpFloodThreshold uint64 = 1000

func (c *ICMPFloodCheck) Check(ctx *alerts.CheckContext) []alerts.Alert {
	// Count ICMP packets per source host
	icmpPackets := make(map[string]uint64)
	for _, f := range ctx.Flows {
		if f.Protocol == "ICMP" || f.Protocol == "ICMPv6" {
			icmpPackets[f.SrcIP] += f.PacketsSent
		}
	}

	var result []alerts.Alert
	for ip, pkts := range icmpPackets {
		if pkts > icmpFloodThreshold {
			result = append(result, alerts.Alert{
				Type:        alerts.AlertTypeHost,
				Severity:    alerts.SeverityCritical,
				Status:      alerts.StatusTriggered,
				RuleID:      "check_icmp_flood",
				Title:       "ICMP Flood Detected",
				Description: fmt.Sprintf("Host %s sent %d ICMP packets (threshold: %d)", ip, pkts, icmpFloodThreshold),
				EntityType:  "host",
				EntityID:    ip,
				TriggeredAt: time.Now(),
			})
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// 20. SuspiciousPortCheck — 可疑端口使用
// ---------------------------------------------------------------------------

// SuspiciousPortCheck 检测已知恶意软件常用端口
type SuspiciousPortCheck struct{}

func (c *SuspiciousPortCheck) Name() string { return "suspicious_port" }
func (c *SuspiciousPortCheck) Description() string {
	return "Detect traffic on ports commonly used by malware"
}
func (c *SuspiciousPortCheck) Type() alerts.AlertType                { return alerts.AlertTypeFlow }
func (c *SuspiciousPortCheck) DefaultSeverity() alerts.AlertSeverity { return alerts.SeverityCritical }

// suspiciousPorts maps known malicious ports to their common association
var suspiciousPorts = map[uint16]string{
	4444:  "Metasploit default",
	5555:  "Common RAT",
	6666:  "IRC backdoor",
	6667:  "IRC botnet C&C",
	31337: "Back Orifice",
	12345: "NetBus",
	27374: "SubSeven",
	1337:  "Common hacker port",
	9999:  "Common backdoor",
	3127:  "MyDoom worm",
	65535: "Common backdoor",
}

func (c *SuspiciousPortCheck) Check(ctx *alerts.CheckContext) []alerts.Alert {
	var result []alerts.Alert
	for _, f := range ctx.Flows {
		if !f.IsActive {
			continue
		}
		if desc, ok := suspiciousPorts[f.DstPort]; ok {
			result = append(result, alerts.Alert{
				Type:        alerts.AlertTypeFlow,
				Severity:    alerts.SeverityCritical,
				Status:      alerts.StatusTriggered,
				RuleID:      "check_suspicious_port",
				Title:       "Suspicious Port Activity",
				Description: fmt.Sprintf("Traffic to port %d (%s) detected: %s:%d -> %s:%d", f.DstPort, desc, f.SrcIP, f.SrcPort, f.DstIP, f.DstPort),
				EntityType:  "flow",
				EntityID:    f.FlowID,
				TriggeredAt: time.Now(),
			})
		}
		if desc, ok := suspiciousPorts[f.SrcPort]; ok {
			result = append(result, alerts.Alert{
				Type:        alerts.AlertTypeFlow,
				Severity:    alerts.SeverityCritical,
				Status:      alerts.StatusTriggered,
				RuleID:      "check_suspicious_port",
				Title:       "Suspicious Port Activity",
				Description: fmt.Sprintf("Traffic from port %d (%s) detected: %s:%d -> %s:%d", f.SrcPort, desc, f.SrcIP, f.SrcPort, f.DstIP, f.DstPort),
				EntityType:  "flow",
				EntityID:    f.FlowID,
				TriggeredAt: time.Now(),
			})
		}
	}
	return result
}
