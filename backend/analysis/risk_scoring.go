package analysis

import (
	"sort"
	"strings"
	"sync"
	"time"
)

// RiskLevel represents the risk classification of a host
type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

// HostRiskScore holds the computed risk score for a host
type HostRiskScore struct {
	IP          string       `json:"ip"`
	Score       int          `json:"score"` // 0-100
	Level       RiskLevel    `json:"level"`
	Factors     []RiskFactor `json:"factors"`
	LastUpdated time.Time    `json:"last_updated"`
}

// RiskFactor describes a single scoring dimension
type RiskFactor struct {
	Name        string `json:"name"`
	Score       int    `json:"score"` // contribution
	Description string `json:"description"`
}

// RiskScorer computes and caches per-host risk scores
type RiskScorer struct {
	mu     sync.RWMutex
	scores map[string]*HostRiskScore // IP -> score
}

// NewRiskScorer creates a new RiskScorer instance
func NewRiskScorer() *RiskScorer {
	return &RiskScorer{
		scores: make(map[string]*HostRiskScore),
	}
}

// suspiciousProtocols are plaintext / risky protocols
var suspiciousProtocols = map[string]bool{
	"Telnet": true, "FTP": true, "TFTP": true,
	"SMTP": true, "POP3": true, "IMAP": true,
	"telnet": true, "ftp": true, "tftp": true,
	"smtp": true, "pop3": true, "imap": true,
}

// CalculateScore computes the risk score for a single host.
// Parameters:
//   - host: host statistics
//   - alertCount: number of alerts in last 24h for this host
//   - blacklisted: whether IP is on the blacklist
//   - flowCount: number of active flows for this host
//   - encryptedRatio: ratio of encrypted traffic (0.0 – 1.0)
func (rs *RiskScorer) CalculateScore(host *HostStats, alertCount int, blacklisted bool, flowCount int, encryptedRatio float64) *HostRiskScore {
	if host == nil {
		return nil
	}

	factors := make([]RiskFactor, 0, 6)
	total := 0

	// 1. Connection count (0-15)
	var connScore int
	switch {
	case flowCount > 200:
		connScore = 15
	case flowCount > 100:
		connScore = 10
	case flowCount > 50:
		connScore = 5
	}
	if connScore > 0 {
		factors = append(factors, RiskFactor{
			Name:        "connection_count",
			Score:       connScore,
			Description: formatConnDesc(flowCount),
		})
		total += connScore
	}

	// 2. Suspicious protocols (0-20)
	suspCount := 0
	suspNames := []string{}
	for proto := range host.Protocols {
		if suspiciousProtocols[proto] {
			suspCount++
			suspNames = append(suspNames, proto)
		}
	}
	var protoScore int
	switch {
	case suspCount >= 3:
		protoScore = 20
	case suspCount == 2:
		protoScore = 15
	case suspCount == 1:
		protoScore = 10
	}
	if protoScore > 0 {
		factors = append(factors, RiskFactor{
			Name:        "suspicious_protocols",
			Score:       protoScore,
			Description: "使用明文协议: " + strings.Join(suspNames, ", "),
		})
		total += protoScore
	}

	// 3. Blacklist hit (0-25)
	if blacklisted {
		factors = append(factors, RiskFactor{
			Name:        "blacklist_hit",
			Score:       25,
			Description: "IP 在黑名单中",
		})
		total += 25
	}

	// 4. Alert history (0-20)
	var alertScore int
	switch {
	case alertCount > 10:
		alertScore = 20
	case alertCount > 5:
		alertScore = 15
	case alertCount > 1:
		alertScore = 10
	}
	if alertScore > 0 {
		factors = append(factors, RiskFactor{
			Name:        "alert_history",
			Score:       alertScore,
			Description: formatAlertDesc(alertCount),
		})
		total += alertScore
	}

	// 5. Traffic ratio anomaly (0-10)
	if host.BytesRecv > 0 {
		ratio := float64(host.BytesSent) / float64(host.BytesRecv)
		if ratio > 10 {
			factors = append(factors, RiskFactor{
				Name:        "traffic_ratio",
				Score:       10,
				Description: "发送/接收比例异常 (>10:1)",
			})
			total += 10
		}
	} else if host.BytesSent > 0 {
		// Sending but never receiving is also suspicious
		factors = append(factors, RiskFactor{
			Name:        "traffic_ratio",
			Score:       10,
			Description: "仅有发送流量，无接收流量",
		})
		total += 10
	}

	// 6. Encrypted traffic ratio (0-10)
	if encryptedRatio < 0.3 { // non-encrypted > 70%
		factors = append(factors, RiskFactor{
			Name:        "encryption_ratio",
			Score:       10,
			Description: "非加密流量占比 > 70%",
		})
		total += 10
	}

	// Cap at 100
	if total > 100 {
		total = 100
	}

	level := scoreToLevel(total)

	result := &HostRiskScore{
		IP:          host.IP,
		Score:       total,
		Level:       level,
		Factors:     factors,
		LastUpdated: time.Now(),
	}

	// Cache the score
	rs.mu.Lock()
	rs.scores[host.IP] = result
	rs.mu.Unlock()

	return result
}

// GetScore returns the cached risk score for a given IP
func (rs *RiskScorer) GetScore(ip string) *HostRiskScore {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.scores[ip]
}

// GetAllScores returns a copy of all cached scores
func (rs *RiskScorer) GetAllScores() map[string]*HostRiskScore {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	out := make(map[string]*HostRiskScore, len(rs.scores))
	for k, v := range rs.scores {
		out[k] = v
	}
	return out
}

// GetTopRisks returns hosts sorted by risk score descending, limited to n
func (rs *RiskScorer) GetTopRisks(n int) []*HostRiskScore {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	list := make([]*HostRiskScore, 0, len(rs.scores))
	for _, v := range rs.scores {
		list = append(list, v)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].Score > list[j].Score
	})
	if n > 0 && n < len(list) {
		list = list[:n]
	}
	return list
}

// ── helpers ──────────────────────────────────────────────────────────────────

func scoreToLevel(score int) RiskLevel {
	switch {
	case score >= 76:
		return RiskCritical
	case score >= 51:
		return RiskHigh
	case score >= 26:
		return RiskMedium
	default:
		return RiskLow
	}
}

func formatConnDesc(n int) string {
	if n > 200 {
		return "活跃连接数 > 200"
	} else if n > 100 {
		return "活跃连接数 > 100"
	}
	return "活跃连接数 > 50"
}

func formatAlertDesc(n int) string {
	if n > 10 {
		return "近24小时告警 > 10"
	} else if n > 5 {
		return "近24小时告警 > 5"
	}
	return "近24小时告警 > 1"
}
