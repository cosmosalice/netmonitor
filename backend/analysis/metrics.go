package analysis

import (
	"sync"
	"time"
)

// Metrics holds real-time network metrics
type Metrics struct {
	BytesPerSec     uint64    `json:"bytes_per_sec"`
	PacketsPerSec   uint64    `json:"packets_per_sec"`
	ActiveFlows     int       `json:"active_flows"`
	ActiveHosts     int       `json:"active_hosts"`
	ActiveProtocols int       `json:"active_protocols"`
	Timestamp       time.Time `json:"timestamp"`
}

// MetricsCalculator calculates real-time metrics
type MetricsCalculator struct {
	mu            sync.Mutex
	bytesWindow   []uint64
	packetsWindow []uint64
	windowSize    int
	currentBytes  uint64
	currentPackets uint64
	lastUpdate    time.Time
}

// NewMetricsCalculator creates a new metrics calculator
func NewMetricsCalculator(windowSeconds int) *MetricsCalculator {
	mc := &MetricsCalculator{
		bytesWindow:   make([]uint64, 0, windowSeconds),
		packetsWindow: make([]uint64, 0, windowSeconds),
		windowSize:    windowSeconds,
		lastUpdate:    time.Now(),
	}

	// Start metrics update loop
	go mc.updateLoop()

	return mc
}

// Update updates current metrics
func (mc *MetricsCalculator) Update(bytes uint64, packets uint64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.currentBytes += bytes
	mc.currentPackets += packets
}

// GetMetrics returns current calculated metrics
func (mc *MetricsCalculator) GetMetrics(activeFlows, activeHosts, activeProtocols int) *Metrics {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	// Calculate average over window
	var avgBytes, avgPackets uint64
	if len(mc.bytesWindow) > 0 {
		for _, b := range mc.bytesWindow {
			avgBytes += b
		}
		avgBytes /= uint64(len(mc.bytesWindow))

		for _, p := range mc.packetsWindow {
			avgPackets += p
		}
		avgPackets /= uint64(len(mc.packetsWindow))
	}

	return &Metrics{
		BytesPerSec:     avgBytes,
		PacketsPerSec:   avgPackets,
		ActiveFlows:     activeFlows,
		ActiveHosts:     activeHosts,
		ActiveProtocols: activeProtocols,
		Timestamp:       time.Now(),
	}
}

// updateLoop updates metrics every second
func (mc *MetricsCalculator) updateLoop() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		mc.mu.Lock()
		
		// Add current values to window
		mc.bytesWindow = append(mc.bytesWindow, mc.currentBytes)
		mc.packetsWindow = append(mc.packetsWindow, mc.currentPackets)

		// Keep window size limited
		if len(mc.bytesWindow) > mc.windowSize {
			mc.bytesWindow = mc.bytesWindow[1:]
			mc.packetsWindow = mc.packetsWindow[1:]
		}

		// Reset current counters
		mc.currentBytes = 0
		mc.currentPackets = 0
		mc.lastUpdate = time.Now()
		
		mc.mu.Unlock()
	}
}
