package checks

import (
	"fmt"
	"time"

	"github.com/netmonitor/backend/alerts"
)

// ---------------------------------------------------------------------------
// 1. BandwidthSpikeCheck — 突发流量检测
// ---------------------------------------------------------------------------

// BandwidthSpikeCheck 当前带宽超过历史平均的 5 倍时告警
type BandwidthSpikeCheck struct {
	// 简易滑动窗口：保留最近的带宽采样值
	history []float64
	maxHist int
}

func (c *BandwidthSpikeCheck) Name() string { return "bandwidth_spike" }
func (c *BandwidthSpikeCheck) Description() string {
	return "Detect sudden bandwidth spikes (>5x average)"
}
func (c *BandwidthSpikeCheck) Type() alerts.AlertType                { return alerts.AlertTypeSystem }
func (c *BandwidthSpikeCheck) DefaultSeverity() alerts.AlertSeverity { return alerts.SeverityWarning }

func (c *BandwidthSpikeCheck) Check(ctx *alerts.CheckContext) []alerts.Alert {
	if c.maxHist == 0 {
		c.maxHist = 60 // ~30 min at 30s intervals
	}

	current := ctx.BytesPerSec

	// Calculate average from history
	if len(c.history) > 0 {
		var sum float64
		for _, v := range c.history {
			sum += v
		}
		avg := sum / float64(len(c.history))

		if avg > 0 && current > avg*5 {
			c.appendHistory(current)
			return []alerts.Alert{{
				Type:        alerts.AlertTypeSystem,
				Severity:    alerts.SeverityWarning,
				Status:      alerts.StatusTriggered,
				RuleID:      "check_bandwidth_spike",
				Title:       "Bandwidth Spike Detected",
				Description: fmt.Sprintf("Current bandwidth %.2f B/s is %.1fx the average %.2f B/s", current, current/avg, avg),
				EntityType:  "system",
				EntityID:    "system",
				TriggeredAt: time.Now(),
			}}
		}
	}

	c.appendHistory(current)
	return nil
}

func (c *BandwidthSpikeCheck) appendHistory(v float64) {
	c.history = append(c.history, v)
	if len(c.history) > c.maxHist {
		c.history = c.history[1:]
	}
}

// ---------------------------------------------------------------------------
// 2. ElephantFlowCheck — 大流检测
// ---------------------------------------------------------------------------

// ElephantFlowCheck 单 Flow 超过 100MB
type ElephantFlowCheck struct{}

func (c *ElephantFlowCheck) Name() string                          { return "elephant_flow" }
func (c *ElephantFlowCheck) Description() string                   { return "Detect elephant flows (>100MB)" }
func (c *ElephantFlowCheck) Type() alerts.AlertType                { return alerts.AlertTypeFlow }
func (c *ElephantFlowCheck) DefaultSeverity() alerts.AlertSeverity { return alerts.SeverityWarning }

const elephantFlowThreshold uint64 = 100 * 1024 * 1024 // 100MB

func (c *ElephantFlowCheck) Check(ctx *alerts.CheckContext) []alerts.Alert {
	var result []alerts.Alert
	for _, f := range ctx.Flows {
		totalBytes := f.BytesSent + f.BytesRecv
		if totalBytes > elephantFlowThreshold {
			result = append(result, alerts.Alert{
				Type:        alerts.AlertTypeFlow,
				Severity:    alerts.SeverityWarning,
				Status:      alerts.StatusTriggered,
				RuleID:      "check_elephant_flow",
				Title:       "Elephant Flow Detected",
				Description: fmt.Sprintf("Flow %s (%s:%d -> %s:%d) transferred %d MB", f.FlowID, f.SrcIP, f.SrcPort, f.DstIP, f.DstPort, totalBytes/(1024*1024)),
				EntityType:  "flow",
				EntityID:    f.FlowID,
				TriggeredAt: time.Now(),
			})
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// 3. LongLivedFlowCheck — 长连接检测
// ---------------------------------------------------------------------------

// LongLivedFlowCheck Flow 持续超过 1 小时
type LongLivedFlowCheck struct{}

func (c *LongLivedFlowCheck) Name() string                          { return "long_lived_flow" }
func (c *LongLivedFlowCheck) Description() string                   { return "Detect long-lived flows (>1 hour)" }
func (c *LongLivedFlowCheck) Type() alerts.AlertType                { return alerts.AlertTypeFlow }
func (c *LongLivedFlowCheck) DefaultSeverity() alerts.AlertSeverity { return alerts.SeverityInfo }

const longLivedFlowThreshold = 1 * time.Hour

func (c *LongLivedFlowCheck) Check(ctx *alerts.CheckContext) []alerts.Alert {
	var result []alerts.Alert
	now := time.Now()
	for _, f := range ctx.Flows {
		if !f.IsActive {
			continue
		}
		duration := now.Sub(f.StartTime)
		if duration > longLivedFlowThreshold {
			result = append(result, alerts.Alert{
				Type:        alerts.AlertTypeFlow,
				Severity:    alerts.SeverityInfo,
				Status:      alerts.StatusTriggered,
				RuleID:      "check_long_lived_flow",
				Title:       "Long-Lived Flow Detected",
				Description: fmt.Sprintf("Flow %s (%s -> %s, %s) has been active for %s", f.FlowID, f.SrcIP, f.DstIP, f.Protocol, duration.Round(time.Minute)),
				EntityType:  "flow",
				EntityID:    f.FlowID,
				TriggeredAt: time.Now(),
			})
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// 4. HighPacketRateCheck — 高包速率检测
// ---------------------------------------------------------------------------

// HighPacketRateCheck 超过 100000 pps
type HighPacketRateCheck struct{}

func (c *HighPacketRateCheck) Name() string                          { return "high_packet_rate" }
func (c *HighPacketRateCheck) Description() string                   { return "Detect high packet rate (>100000 pps)" }
func (c *HighPacketRateCheck) Type() alerts.AlertType                { return alerts.AlertTypeSystem }
func (c *HighPacketRateCheck) DefaultSeverity() alerts.AlertSeverity { return alerts.SeverityCritical }

const highPacketRateThreshold float64 = 100000

func (c *HighPacketRateCheck) Check(ctx *alerts.CheckContext) []alerts.Alert {
	if ctx.PacketsPerSec > highPacketRateThreshold {
		return []alerts.Alert{{
			Type:        alerts.AlertTypeSystem,
			Severity:    alerts.SeverityCritical,
			Status:      alerts.StatusTriggered,
			RuleID:      "check_high_packet_rate",
			Title:       "High Packet Rate Detected",
			Description: fmt.Sprintf("Current packet rate %.0f pps exceeds threshold %.0f pps", ctx.PacketsPerSec, highPacketRateThreshold),
			EntityType:  "system",
			EntityID:    "system",
			TriggeredAt: time.Now(),
		}}
	}
	return nil
}
