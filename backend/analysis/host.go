package analysis

import (
	"sort"
	"sync"
	"time"
)

// HostStats represents statistics for a host
type HostStats struct {
	IP          string            `json:"ip"`
	MAC         string            `json:"mac"`
	Hostname    string            `json:"hostname"`
	BytesSent   uint64            `json:"bytes_sent"`
	BytesRecv   uint64            `json:"bytes_recv"`
	PacketsSent uint64            `json:"packets_sent"`
	PacketsRecv uint64            `json:"packets_recv"`
	ActiveFlows int               `json:"active_flows"`
	Protocols   map[string]uint64 `json:"protocols"`
	FirstSeen   time.Time         `json:"first_seen"`
	LastSeen    time.Time         `json:"last_seen"`
	Country     string            `json:"country"`
	City        string            `json:"city"`
	Latitude    float64           `json:"latitude"`
	Longitude   float64           `json:"longitude"`
	ASN         uint              `json:"asn"`
	ASOrg       string            `json:"as_org"`
	OS          *OSInfo           `json:"os,omitempty"`
	RiskScore   *HostRiskScore    `json:"risk_score,omitempty"`
}

// HostManager manages host statistics
type HostManager struct {
	mu    sync.RWMutex
	hosts map[string]*HostStats
}

// NewHostManager creates a new host manager
func NewHostManager() *HostManager {
	return &HostManager{
		hosts: make(map[string]*HostStats),
	}
}

// UpdateHost updates host statistics
func (hm *HostManager) UpdateHost(ip string, mac string, bytes uint64, isSent bool) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	host, exists := hm.hosts[ip]
	if !exists {
		host = &HostStats{
			IP:        ip,
			MAC:       mac,
			Protocols: make(map[string]uint64),
			FirstSeen: time.Now(),
		}
		hm.hosts[ip] = host
	}

	host.LastSeen = time.Now()

	if isSent {
		host.BytesSent += bytes
		host.PacketsSent++
	} else {
		host.BytesRecv += bytes
		host.PacketsRecv++
	}
}

// UpdateHostProtocol updates protocol statistics for a host
func (hm *HostManager) UpdateHostProtocol(ip string, protocol string, bytes uint64) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	host, exists := hm.hosts[ip]
	if !exists {
		return
	}

	host.Protocols[protocol] += bytes
}

// GetHostStats returns statistics for a specific host
func (hm *HostManager) GetHostStats(ip string) *HostStats {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	return hm.hosts[ip]
}

// GetAllHosts returns all host statistics
func (hm *HostManager) GetAllHosts() []*HostStats {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	var hosts []*HostStats
	for _, host := range hm.hosts {
		hosts = append(hosts, host)
	}

	return hosts
}

// GetTopTalkers returns top N hosts by traffic
func (hm *HostManager) GetTopTalkers(n int, sortBy string) []*HostStats {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	var hosts []*HostStats
	for _, host := range hm.hosts {
		hosts = append(hosts, host)
	}

	// Sort by specified metric
	switch sortBy {
	case "bytes_sent":
		sort.Slice(hosts, func(i, j int) bool {
			return hosts[i].BytesSent > hosts[j].BytesSent
		})
	case "bytes_recv":
		sort.Slice(hosts, func(i, j int) bool {
			return hosts[i].BytesRecv > hosts[j].BytesRecv
		})
	case "packets":
		sort.Slice(hosts, func(i, j int) bool {
			return (hosts[i].PacketsSent + hosts[i].PacketsRecv) > (hosts[j].PacketsSent + hosts[j].PacketsRecv)
		})
	default: // total bytes
		sort.Slice(hosts, func(i, j int) bool {
			return (hosts[i].BytesSent + hosts[i].BytesRecv) > (hosts[j].BytesSent + hosts[j].BytesRecv)
		})
	}

	if n > len(hosts) {
		n = len(hosts)
	}

	return hosts[:n]
}

// GetHostCount returns the number of tracked hosts
func (hm *HostManager) GetHostCount() int {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	return len(hm.hosts)
}
