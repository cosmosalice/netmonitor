package checks

import (
	"fmt"
	"time"

	"github.com/netmonitor/backend/alerts"
)

// ---------------------------------------------------------------------------
// 15. BroadcastStormCheck — 广播风暴
// ---------------------------------------------------------------------------

// BroadcastStormCheck 广播包占比超过 30%
type BroadcastStormCheck struct{}

func (c *BroadcastStormCheck) Name() string { return "broadcast_storm" }
func (c *BroadcastStormCheck) Description() string {
	return "Detect broadcast storms (broadcast traffic >30%)"
}
func (c *BroadcastStormCheck) Type() alerts.AlertType                { return alerts.AlertTypeSystem }
func (c *BroadcastStormCheck) DefaultSeverity() alerts.AlertSeverity { return alerts.SeverityCritical }

const broadcastStormThreshold = 0.30

func (c *BroadcastStormCheck) Check(ctx *alerts.CheckContext) []alerts.Alert {
	if len(ctx.Flows) == 0 {
		return nil
	}

	var totalPackets, broadcastPackets uint64
	for _, f := range ctx.Flows {
		pkts := f.PacketsSent + f.PacketsRecv
		totalPackets += pkts
		// Detect broadcast by destination IP pattern (255.255.255.255 or .255 suffix)
		if f.DstIP == "255.255.255.255" || f.DstIP == "0.0.0.0" {
			broadcastPackets += pkts
		}
	}

	if totalPackets == 0 {
		return nil
	}

	ratio := float64(broadcastPackets) / float64(totalPackets)
	if ratio > broadcastStormThreshold {
		return []alerts.Alert{{
			Type:        alerts.AlertTypeSystem,
			Severity:    alerts.SeverityCritical,
			Status:      alerts.StatusTriggered,
			RuleID:      "check_broadcast_storm",
			Title:       "Broadcast Storm Detected",
			Description: fmt.Sprintf("Broadcast traffic is %.1f%% of total packets (threshold: %.0f%%)", ratio*100, broadcastStormThreshold*100),
			EntityType:  "system",
			EntityID:    "system",
			TriggeredAt: time.Now(),
		}}
	}
	return nil
}

// ---------------------------------------------------------------------------
// 16. HighRetransmissionCheck — 高重传率
// ---------------------------------------------------------------------------

// HighRetransmissionCheck Flow 重传率超过 5%
type HighRetransmissionCheck struct{}

func (c *HighRetransmissionCheck) Name() string { return "high_retransmission" }
func (c *HighRetransmissionCheck) Description() string {
	return "Detect flows with high retransmission rate (>5%)"
}
func (c *HighRetransmissionCheck) Type() alerts.AlertType { return alerts.AlertTypeFlow }
func (c *HighRetransmissionCheck) DefaultSeverity() alerts.AlertSeverity {
	return alerts.SeverityWarning
}

const highRetransmissionThreshold = 0.05

func (c *HighRetransmissionCheck) Check(ctx *alerts.CheckContext) []alerts.Alert {
	var result []alerts.Alert
	for _, f := range ctx.Flows {
		if f.Protocol != "TCP" || !f.IsActive {
			continue
		}
		totalPkts := f.PacketsSent + f.PacketsRecv
		if totalPkts == 0 || f.Retransmissions == 0 {
			continue
		}
		retransmitRate := float64(f.Retransmissions) / float64(totalPkts)
		if retransmitRate > highRetransmissionThreshold {
			result = append(result, alerts.Alert{
				Type:        alerts.AlertTypeFlow,
				Severity:    alerts.SeverityWarning,
				Status:      alerts.StatusTriggered,
				RuleID:      "check_high_retransmission",
				Title:       "High Retransmission Rate",
				Description: fmt.Sprintf("Flow %s (%s -> %s) has %.1f%% retransmission rate (%d retransmissions / %d packets)", f.FlowID, f.SrcIP, f.DstIP, retransmitRate*100, f.Retransmissions, totalPkts),
				EntityType:  "flow",
				EntityID:    f.FlowID,
				TriggeredAt: time.Now(),
			})
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// 17. HighLatencyCheck — 高延迟
// ---------------------------------------------------------------------------

// HighLatencyCheck RTT 超过 500ms
type HighLatencyCheck struct{}

func (c *HighLatencyCheck) Name() string                          { return "high_latency" }
func (c *HighLatencyCheck) Description() string                   { return "Detect flows with high latency (RTT >500ms)" }
func (c *HighLatencyCheck) Type() alerts.AlertType                { return alerts.AlertTypeFlow }
func (c *HighLatencyCheck) DefaultSeverity() alerts.AlertSeverity { return alerts.SeverityWarning }

const highLatencyThreshold float64 = 500.0 // ms

func (c *HighLatencyCheck) Check(ctx *alerts.CheckContext) []alerts.Alert {
	var result []alerts.Alert
	for _, f := range ctx.Flows {
		if f.Protocol != "TCP" || !f.IsActive {
			continue
		}
		if f.RTTMs > highLatencyThreshold {
			result = append(result, alerts.Alert{
				Type:        alerts.AlertTypeFlow,
				Severity:    alerts.SeverityWarning,
				Status:      alerts.StatusTriggered,
				RuleID:      "check_high_latency",
				Title:       "High Latency Detected",
				Description: fmt.Sprintf("Flow %s (%s -> %s) RTT %.1fms exceeds threshold %.0fms", f.FlowID, f.SrcIP, f.DstIP, f.RTTMs, highLatencyThreshold),
				EntityType:  "flow",
				EntityID:    f.FlowID,
				TriggeredAt: time.Now(),
			})
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// 18. PacketLossCheck — 高丢包率
// ---------------------------------------------------------------------------

// PacketLossCheck 丢包率超过 3%
type PacketLossCheck struct{}

func (c *PacketLossCheck) Name() string                          { return "packet_loss" }
func (c *PacketLossCheck) Description() string                   { return "Detect flows with high packet loss (>3%)" }
func (c *PacketLossCheck) Type() alerts.AlertType                { return alerts.AlertTypeFlow }
func (c *PacketLossCheck) DefaultSeverity() alerts.AlertSeverity { return alerts.SeverityWarning }

const packetLossThreshold = 0.03

func (c *PacketLossCheck) Check(ctx *alerts.CheckContext) []alerts.Alert {
	var result []alerts.Alert
	for _, f := range ctx.Flows {
		if f.Protocol != "TCP" || !f.IsActive {
			continue
		}
		totalPkts := f.PacketsSent + f.PacketsRecv
		if totalPkts == 0 || f.Retransmissions == 0 {
			continue
		}
		// Approximate packet loss as retransmission-based estimate
		lossRate := float64(f.Retransmissions) / float64(totalPkts+uint64(f.Retransmissions))
		if lossRate > packetLossThreshold {
			result = append(result, alerts.Alert{
				Type:        alerts.AlertTypeFlow,
				Severity:    alerts.SeverityWarning,
				Status:      alerts.StatusTriggered,
				RuleID:      "check_packet_loss",
				Title:       "High Packet Loss Detected",
				Description: fmt.Sprintf("Flow %s (%s -> %s) estimated packet loss %.1f%% (threshold: %.0f%%)", f.FlowID, f.SrcIP, f.DstIP, lossRate*100, packetLossThreshold*100),
				EntityType:  "flow",
				EntityID:    f.FlowID,
				TriggeredAt: time.Now(),
			})
		}
	}
	return result
}
