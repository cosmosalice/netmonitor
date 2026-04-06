package analysis

import (
	"fmt"
	"sync"
	"time"
)

// Flow represents a network flow (5-tuple)
type Flow struct {
	ID          string      `json:"id"`
	SrcIP       string      `json:"src_ip"`
	DstIP       string      `json:"dst_ip"`
	SrcPort     uint16      `json:"src_port"`
	DstPort     uint16      `json:"dst_port"`
	Protocol    string      `json:"protocol"`
	VLANID      uint16      `json:"vlan_id"` // 802.1Q VLAN ID (0 if no VLAN tag)
	BytesSent   uint64      `json:"bytes_sent"`
	BytesRecv   uint64      `json:"bytes_recv"`
	PacketsSent uint64      `json:"packets_sent"`
	PacketsRecv uint64      `json:"packets_recv"`
	StartTime   time.Time   `json:"start_time"`
	LastSeen    time.Time   `json:"last_seen"`
	L7Protocol  string      `json:"l7_protocol"`
	L7Category  string      `json:"l7_category"`
	IsActive    bool        `json:"is_active"`
	TCPMetrics  *TCPMetrics `json:"tcp_metrics,omitempty"`
}

// FlowManager manages network flows
type FlowManager struct {
	mu        sync.RWMutex
	flows     map[string]*Flow
	timeout   map[string]time.Duration
	maxFlows  int
	onFlowEnd func(*Flow)
}

// NewFlowManager creates a new flow manager
func NewFlowManager(maxFlows int, onFlowEnd func(*Flow)) *FlowManager {
	fm := &FlowManager{
		flows:     make(map[string]*Flow),
		maxFlows:  maxFlows,
		onFlowEnd: onFlowEnd,
		timeout: map[string]time.Duration{
			"TCP": 300 * time.Second,
			"UDP": 60 * time.Second,
		},
	}

	// Start cleanup goroutine
	go fm.cleanupLoop()

	return fm
}

// GetOrCreateFlow gets an existing flow or creates a new one
func (fm *FlowManager) GetOrCreateFlow(srcIP, dstIP string, srcPort, dstPort uint16, protocol string, vlanID uint16) *Flow {
	flowKey := createFlowKey(srcIP, dstIP, srcPort, dstPort, protocol)

	fm.mu.Lock()
	defer fm.mu.Unlock()

	if flow, exists := fm.flows[flowKey]; exists {
		flow.LastSeen = time.Now()
		return flow
	}

	// Create new flow
	flow := &Flow{
		ID:        flowKey,
		SrcIP:     srcIP,
		DstIP:     dstIP,
		SrcPort:   srcPort,
		DstPort:   dstPort,
		Protocol:  protocol,
		VLANID:    vlanID,
		StartTime: time.Now(),
		LastSeen:  time.Now(),
		IsActive:  true,
	}

	fm.flows[flowKey] = flow

	// Evict old flows if at capacity
	if len(fm.flows) > fm.maxFlows {
		fm.evictOldestFlow()
	}

	return flow
}

// UpdateFlow updates flow statistics
func (fm *FlowManager) UpdateFlow(flowKey string, bytes uint64, isSrcToDst bool) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	flow, exists := fm.flows[flowKey]
	if !exists {
		return
	}

	flow.LastSeen = time.Now()

	if isSrcToDst {
		flow.BytesSent += bytes
		flow.PacketsSent++
	} else {
		flow.BytesRecv += bytes
		flow.PacketsRecv++
	}
}

// GetActiveFlows returns all active flows
func (fm *FlowManager) GetActiveFlows() []*Flow {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	var activeFlows []*Flow
	for _, flow := range fm.flows {
		if flow.IsActive {
			activeFlows = append(activeFlows, flow)
		}
	}

	return activeFlows
}

// GetFlowCount returns the number of active flows
func (fm *FlowManager) GetFlowCount() int {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	count := 0
	for _, flow := range fm.flows {
		if flow.IsActive {
			count++
		}
	}
	return count
}

// evictOldestFlow removes the oldest inactive flow
func (fm *FlowManager) evictOldestFlow() {
	var oldestKey string
	var oldestTime time.Time

	for key, flow := range fm.flows {
		if !flow.IsActive {
			if oldestKey == "" || flow.LastSeen.Before(oldestTime) {
				oldestKey = key
				oldestTime = flow.LastSeen
			}
		}
	}

	if oldestKey != "" {
		delete(fm.flows, oldestKey)
	}
}

// cleanupLoop periodically cleans up expired flows
func (fm *FlowManager) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		fm.cleanupExpiredFlows()
	}
}

// cleanupExpiredFlows removes flows that have timed out
func (fm *FlowManager) cleanupExpiredFlows() {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	now := time.Now()
	for _, flow := range fm.flows {
		if !flow.IsActive {
			continue
		}

		timeout, exists := fm.timeout[flow.Protocol]
		if !exists {
			timeout = 120 * time.Second // Default timeout
		}

		if now.Sub(flow.LastSeen) > timeout {
			flow.IsActive = false
			if fm.onFlowEnd != nil {
				fm.onFlowEnd(flow)
			}
		}
	}
}

// createFlowKey creates a unique key for a flow
func createFlowKey(srcIP, dstIP string, srcPort, dstPort uint16, protocol string) string {
	// Normalize flow key
	if srcIP > dstIP || (srcIP == dstIP && srcPort > dstPort) {
		srcIP, dstIP = dstIP, srcIP
		srcPort, dstPort = dstPort, srcPort
	}
	return fmt.Sprintf("%s:%d-%s:%d-%s", srcIP, srcPort, dstIP, dstPort, protocol)
}
