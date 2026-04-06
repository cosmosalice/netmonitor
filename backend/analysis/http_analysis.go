package analysis

import (
	"bufio"
	"bytes"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/netmonitor/backend/capture"
)

// HTTPHostStats tracks stats for a single HTTP host
type HTTPHostStats struct {
	Host         string    `json:"host"`
	RequestCount int64     `json:"request_count"`
	BytesIn      uint64    `json:"bytes_in"`
	BytesOut     uint64    `json:"bytes_out"`
	LastSeen     time.Time `json:"last_seen"`
}

// HTTPSummary returns summary statistics
type HTTPSummary struct {
	TotalRequests  int64            `json:"total_requests"`
	TotalResponses int64            `json:"total_responses"`
	UniqueHosts    int              `json:"unique_hosts"`
	TopMethods     map[string]int64 `json:"top_methods"`
	TopStatusCodes map[int]int64    `json:"top_status_codes"`
}

// HTTPStats tracks HTTP traffic statistics
type HTTPStats struct {
	mu             sync.RWMutex
	hosts          map[string]*HTTPHostStats
	userAgents     map[string]int64
	contentTypes   map[string]int64
	statusCodes    map[int]int64
	methods        map[string]int64
	totalRequests  int64
	totalResponses int64
}

// NewHTTPStats creates a new HTTP stats tracker
func NewHTTPStats() *HTTPStats {
	return &HTTPStats{
		hosts:        make(map[string]*HTTPHostStats),
		userAgents:   make(map[string]int64),
		contentTypes: make(map[string]int64),
		statusCodes:  make(map[int]int64),
		methods:      make(map[string]int64),
	}
}

var httpMethods = []string{"GET ", "POST ", "PUT ", "DELETE ", "HEAD ", "OPTIONS ", "PATCH ", "CONNECT ", "TRACE "}

// ProcessPacket analyzes a TCP packet for HTTP content
func (h *HTTPStats) ProcessPacket(pkt capture.Packet, srcPort, dstPort uint16, pktLen uint64) {
	// Parse the packet to get TCP payload
	packet := gopacket.NewPacket(pkt.Data, layers.LayerTypeEthernet, gopacket.NoCopy)
	tcpLayer := packet.Layer(layers.LayerTypeTCP)
	if tcpLayer == nil {
		return
	}
	tcp, _ := tcpLayer.(*layers.TCP)
	payload := tcp.Payload
	if len(payload) < 10 {
		return
	}

	payloadStr := string(payload)

	// Check if it's an HTTP request
	isRequest := false
	for _, method := range httpMethods {
		if strings.HasPrefix(payloadStr, method) {
			isRequest = true
			break
		}
	}

	if isRequest {
		h.parseHTTPRequest(payloadStr, pktLen)
		return
	}

	// Check if it's an HTTP response
	if strings.HasPrefix(payloadStr, "HTTP/") {
		h.parseHTTPResponse(payloadStr, pktLen)
	}
}

func (h *HTTPStats) parseHTTPRequest(data string, pktLen uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.totalRequests++

	scanner := bufio.NewScanner(strings.NewReader(data))

	// Parse request line
	if scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, " ", 3)
		if len(parts) >= 1 {
			h.methods[parts[0]]++
		}
	}

	// Parse headers
	host := ""
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			break // end of headers
		}
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "host:") {
			host = strings.TrimSpace(line[5:])
		} else if strings.HasPrefix(lower, "user-agent:") {
			ua := strings.TrimSpace(line[11:])
			if ua != "" {
				h.userAgents[ua]++
			}
		} else if strings.HasPrefix(lower, "content-type:") {
			ct := strings.TrimSpace(line[13:])
			if ct != "" {
				// Simplify content-type (remove params)
				if idx := strings.Index(ct, ";"); idx > 0 {
					ct = strings.TrimSpace(ct[:idx])
				}
				h.contentTypes[ct]++
			}
		}
	}

	if host != "" {
		hs, ok := h.hosts[host]
		if !ok {
			hs = &HTTPHostStats{Host: host}
			h.hosts[host] = hs
		}
		hs.RequestCount++
		hs.BytesOut += pktLen
		hs.LastSeen = time.Now()
	}
}

func (h *HTTPStats) parseHTTPResponse(data string, pktLen uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.totalResponses++

	scanner := bufio.NewScanner(strings.NewReader(data))

	// Parse status line: HTTP/1.1 200 OK
	if scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, " ", 3)
		if len(parts) >= 2 {
			if code, err := strconv.Atoi(parts[1]); err == nil {
				h.statusCodes[code]++
			}
		}
	}

	// Parse response headers
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			break
		}
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "content-type:") {
			ct := strings.TrimSpace(line[13:])
			if ct != "" {
				if idx := strings.Index(ct, ";"); idx > 0 {
					ct = strings.TrimSpace(ct[:idx])
				}
				h.contentTypes[ct]++
			}
		}
	}
}

// IsHTTPPort checks if a port is a common HTTP port
func IsHTTPPort(port uint16) bool {
	return port == 80 || port == 8080 || port == 8000 || port == 8888 || port == 3000
}

// IsHTTPPayload checks if TCP payload looks like HTTP
func IsHTTPPayload(payload []byte) bool {
	if len(payload) < 4 {
		return false
	}
	for _, method := range httpMethods {
		if bytes.HasPrefix(payload, []byte(method)) {
			return true
		}
	}
	return bytes.HasPrefix(payload, []byte("HTTP/"))
}

// GetSummary returns HTTP summary statistics
func (h *HTTPStats) GetSummary() HTTPSummary {
	h.mu.RLock()
	defer h.mu.RUnlock()

	methods := make(map[string]int64, len(h.methods))
	for k, v := range h.methods {
		methods[k] = v
	}
	codes := make(map[int]int64, len(h.statusCodes))
	for k, v := range h.statusCodes {
		codes[k] = v
	}

	return HTTPSummary{
		TotalRequests:  h.totalRequests,
		TotalResponses: h.totalResponses,
		UniqueHosts:    len(h.hosts),
		TopMethods:     methods,
		TopStatusCodes: codes,
	}
}

// GetTopHosts returns top HTTP hosts by request count
func (h *HTTPStats) GetTopHosts(limit int) []*HTTPHostStats {
	h.mu.RLock()
	defer h.mu.RUnlock()

	all := make([]*HTTPHostStats, 0, len(h.hosts))
	for _, hs := range h.hosts {
		cp := *hs
		all = append(all, &cp)
	}

	// Sort by request count descending
	for i := 0; i < len(all); i++ {
		for j := i + 1; j < len(all); j++ {
			if all[j].RequestCount > all[i].RequestCount {
				all[i], all[j] = all[j], all[i]
			}
		}
	}

	if limit > 0 && limit < len(all) {
		all = all[:limit]
	}
	return all
}

// UserAgentEntry represents a user-agent with count
type UserAgentEntry struct {
	UserAgent string `json:"user_agent"`
	Count     int64  `json:"count"`
}

// GetTopUserAgents returns top user agents
func (h *HTTPStats) GetTopUserAgents(limit int) []UserAgentEntry {
	h.mu.RLock()
	defer h.mu.RUnlock()

	entries := make([]UserAgentEntry, 0, len(h.userAgents))
	for ua, count := range h.userAgents {
		entries = append(entries, UserAgentEntry{UserAgent: ua, Count: count})
	}

	// Sort descending
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].Count > entries[i].Count {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	if limit > 0 && limit < len(entries) {
		entries = entries[:limit]
	}
	return entries
}

// ContentTypeEntry represents a content-type with count
type ContentTypeEntry struct {
	ContentType string `json:"content_type"`
	Count       int64  `json:"count"`
}

// GetTopContentTypes returns top content types
func (h *HTTPStats) GetTopContentTypes(limit int) []ContentTypeEntry {
	h.mu.RLock()
	defer h.mu.RUnlock()

	entries := make([]ContentTypeEntry, 0, len(h.contentTypes))
	for ct, count := range h.contentTypes {
		entries = append(entries, ContentTypeEntry{ContentType: ct, Count: count})
	}

	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].Count > entries[i].Count {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	if limit > 0 && limit < len(entries) {
		entries = entries[:limit]
	}
	return entries
}

// GetMethods returns HTTP method distribution
func (h *HTTPStats) GetMethods() map[string]int64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	result := make(map[string]int64, len(h.methods))
	for k, v := range h.methods {
		result[k] = v
	}
	return result
}

// GetStatusCodes returns HTTP status code distribution
func (h *HTTPStats) GetStatusCodes() map[int]int64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	result := make(map[int]int64, len(h.statusCodes))
	for k, v := range h.statusCodes {
		result[k] = v
	}
	return result
}
