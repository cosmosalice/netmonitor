package analysis

import (
	"sync"
	"time"

	"github.com/google/gopacket/layers"
)

// OSInfo represents OS fingerprint information for a host
type OSInfo struct {
	OS          string    `json:"os"`
	OSFamily    string    `json:"os_family"`
	Confidence  int       `json:"confidence"`
	Method      string    `json:"method"`
	LastUpdated time.Time `json:"last_updated"`
}

// OSFingerprint performs passive OS fingerprinting based on TCP/IP characteristics
type OSFingerprint struct {
	mu    sync.RWMutex
	hosts map[string]*OSInfo // IP -> OS info
}

// NewOSFingerprint creates a new OS fingerprint analyzer
func NewOSFingerprint() *OSFingerprint {
	return &OSFingerprint{
		hosts: make(map[string]*OSInfo),
	}
}

// AnalyzePacket analyzes TCP/IP characteristics of a SYN packet for OS fingerprinting
func (of *OSFingerprint) AnalyzePacket(srcIP string, ttl uint8, windowSize uint16, tcpOptions []layers.TCPOption, df bool) {
	info := of.matchOS(ttl, windowSize, tcpOptions, df)
	if info == nil {
		return
	}
	info.LastUpdated = time.Now()

	of.mu.Lock()
	defer of.mu.Unlock()

	existing, ok := of.hosts[srcIP]
	if !ok || info.Confidence > existing.Confidence {
		of.hosts[srcIP] = info
	}
}

// GetOSInfo returns OS info for a specific IP
func (of *OSFingerprint) GetOSInfo(ip string) *OSInfo {
	of.mu.RLock()
	defer of.mu.RUnlock()
	return of.hosts[ip]
}

// GetAllOSInfo returns OS info for all known hosts
func (of *OSFingerprint) GetAllOSInfo() map[string]*OSInfo {
	of.mu.RLock()
	defer of.mu.RUnlock()
	result := make(map[string]*OSInfo, len(of.hosts))
	for ip, info := range of.hosts {
		result[ip] = info
	}
	return result
}

// matchOS performs rule-based OS matching from TCP/IP characteristics
func (of *OSFingerprint) matchOS(ttl uint8, windowSize uint16, tcpOptions []layers.TCPOption, df bool) *OSInfo {
	// Determine TTL-based OS family
	ttlFamily := ""
	ttlConfidence := 0

	switch {
	case ttl <= 64 && ttl > 32:
		ttlFamily = "Linux" // Could be Linux, macOS, Android
		ttlConfidence = 40
	case ttl <= 128 && ttl > 64:
		ttlFamily = "Windows"
		ttlConfidence = 50
	case ttl > 128:
		ttlFamily = "Network"
		ttlConfidence = 30
	default:
		ttlConfidence = 10
	}

	// Analyze TCP options to refine
	hasTimestamps := false
	hasSACK := false
	hasWindowScale := false
	hasMSS := false
	optionOrder := make([]uint8, 0)

	for _, opt := range tcpOptions {
		switch opt.OptionType {
		case layers.TCPOptionKindMSS:
			hasMSS = true
			optionOrder = append(optionOrder, uint8(layers.TCPOptionKindMSS))
		case layers.TCPOptionKindWindowScale:
			hasWindowScale = true
			optionOrder = append(optionOrder, uint8(layers.TCPOptionKindWindowScale))
		case layers.TCPOptionKindTimestamps:
			hasTimestamps = true
			optionOrder = append(optionOrder, uint8(layers.TCPOptionKindTimestamps))
		case layers.TCPOptionKindSACKPermitted:
			hasSACK = true
			optionOrder = append(optionOrder, uint8(layers.TCPOptionKindSACKPermitted))
		}
	}

	// Combined analysis
	os := ""
	osFamily := ""
	confidence := ttlConfidence
	method := "ttl_heuristic"

	switch ttlFamily {
	case "Windows":
		osFamily = "Windows"
		method = "tcp_fingerprint"
		switch windowSize {
		case 64240:
			os = "Windows 10/11"
			confidence = 80
		case 65535:
			os = "Windows 7/XP"
			confidence = 70
		case 8192:
			os = "Windows Server"
			confidence = 65
		default:
			os = "Windows"
			confidence = 55
		}
		// Windows typically: MSS, NOP, WindowScale, SACKPermitted, Timestamps
		if hasMSS && hasWindowScale && hasSACK {
			confidence += 10
		}

	case "Linux":
		// Differentiate Linux, macOS, Android, iOS
		switch {
		case windowSize == 65535 && df:
			// macOS typically uses window 65535
			osFamily = "macOS"
			os = "macOS"
			confidence = 65
			method = "tcp_fingerprint"
			// macOS often has MSS + WindowScale + Timestamps + SACK
			if hasMSS && hasWindowScale && hasTimestamps && hasSACK {
				confidence = 80
			}
		case windowSize == 29200 || windowSize == 28960:
			// Default Linux window
			osFamily = "Linux"
			os = "Linux"
			confidence = 70
			method = "tcp_fingerprint"
			// Linux typical: MSS, NOP, WindowScale, NOP, NOP, Timestamps, SACKPermitted
			if hasMSS && hasWindowScale && hasTimestamps && hasSACK {
				confidence = 85
			}
		case windowSize == 14600:
			osFamily = "Android"
			os = "Android"
			confidence = 60
			method = "tcp_fingerprint"
		default:
			// Check option ordering for more clues
			if isLinuxOptionOrder(optionOrder) {
				osFamily = "Linux"
				os = "Linux"
				confidence = 60
				method = "tcp_fingerprint"
			} else {
				osFamily = "Linux"
				os = "Linux/Unix"
				confidence = 45
			}
		}

	case "Network":
		osFamily = "Network"
		os = "Network Device"
		confidence = 40
		if !df {
			confidence += 10
		}

	default:
		osFamily = "Unknown"
		os = "Unknown"
		confidence = 10
	}

	// DF flag boost: most modern OS set DF
	if df && (osFamily == "Windows" || osFamily == "Linux" || osFamily == "macOS") {
		confidence += 5
	}

	// Cap confidence at 100
	if confidence > 100 {
		confidence = 100
	}

	return &OSInfo{
		OS:         os,
		OSFamily:   osFamily,
		Confidence: confidence,
		Method:     method,
	}
}

// isLinuxOptionOrder checks if TCP option ordering matches Linux patterns
func isLinuxOptionOrder(order []uint8) bool {
	// Linux typical order: MSS, WindowScale, Timestamps, SACK
	if len(order) < 3 {
		return false
	}
	// Check if MSS comes first
	if order[0] == uint8(layers.TCPOptionKindMSS) {
		return true
	}
	return false
}
