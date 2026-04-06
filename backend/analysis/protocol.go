package analysis

import (
	"sort"
	"sync"
	"time"
)

// ProtocolStats represents statistics for a protocol
type ProtocolStats struct {
	Protocol   string    `json:"protocol"`
	Category   string    `json:"category"`
	Bytes      uint64    `json:"bytes"`
	Packets    uint64    `json:"packets"`
	FlowCount  uint64    `json:"flow_count"`
	Percentage float64   `json:"percentage"`
	LastUpdate time.Time `json:"last_update"`
}

// ProtocolManager manages protocol statistics
type ProtocolManager struct {
	mu         sync.RWMutex
	protocols  map[string]*ProtocolStats
	totalBytes uint64
}

// NewProtocolManager creates a new protocol manager
func NewProtocolManager() *ProtocolManager {
	return &ProtocolManager{
		protocols: make(map[string]*ProtocolStats),
	}
}

// UpdateProtocol updates protocol statistics
func (pm *ProtocolManager) UpdateProtocol(protocol string, category string, bytes uint64) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	stats, exists := pm.protocols[protocol]
	if !exists {
		stats = &ProtocolStats{
			Protocol: protocol,
			Category: category,
		}
		pm.protocols[protocol] = stats
	}

	stats.Bytes += bytes
	stats.Packets++
	stats.LastUpdate = time.Now()
	pm.totalBytes += bytes

	// Update percentages
	pm.updatePercentages()
}

// IncrementFlowCount increments the flow count for a protocol
func (pm *ProtocolManager) IncrementFlowCount(protocol string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	stats, exists := pm.protocols[protocol]
	if !exists {
		return
	}

	stats.FlowCount++
}

// GetProtocolStats returns statistics for a specific protocol
func (pm *ProtocolManager) GetProtocolStats(protocol string) *ProtocolStats {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	return pm.protocols[protocol]
}

// GetAllProtocols returns all protocol statistics
func (pm *ProtocolManager) GetAllProtocols() []*ProtocolStats {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var protocols []*ProtocolStats
	for _, stats := range pm.protocols {
		protocols = append(protocols, stats)
	}

	return protocols
}

// GetTopProtocols returns top N protocols by traffic
func (pm *ProtocolManager) GetTopProtocols(n int) []*ProtocolStats {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var protocols []*ProtocolStats
	for _, stats := range pm.protocols {
		protocols = append(protocols, stats)
	}

	// Sort by bytes
	sort.Slice(protocols, func(i, j int) bool {
		return protocols[i].Bytes > protocols[j].Bytes
	})

	if n > len(protocols) {
		n = len(protocols)
	}

	return protocols[:n]
}

// GetTotalBytes returns total bytes across all protocols
func (pm *ProtocolManager) GetTotalBytes() uint64 {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.totalBytes
}

// updatePercentages updates the percentage for each protocol
func (pm *ProtocolManager) updatePercentages() {
	if pm.totalBytes == 0 {
		return
	}

	for _, stats := range pm.protocols {
		stats.Percentage = float64(stats.Bytes) / float64(pm.totalBytes) * 100.0
	}
}

// GetProtocolCount returns the number of tracked protocols
func (pm *ProtocolManager) GetProtocolCount() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return len(pm.protocols)
}
