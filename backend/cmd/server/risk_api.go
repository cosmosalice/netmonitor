package main

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/netmonitor/backend/alerts"
)

// GET /api/v1/hosts/risks?sort=score&limit=50
func (s *Server) getHostRisks(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
			limit = v
		}
	}

	risks := s.riskScorer.GetTopRisks(limit)
	writeJSON(w, map[string]interface{}{
		"hosts": risks,
		"total": len(risks),
	})
}

// GET /api/v1/hosts/{ip}/risk
func (s *Server) getHostRiskDetail(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	ip := vars["ip"]
	if ip == "" {
		writeError(w, "ip is required", http.StatusBadRequest)
		return
	}

	score := s.riskScorer.GetScore(ip)
	if score == nil {
		writeError(w, "no risk data for this host", http.StatusNotFound)
		return
	}

	writeJSON(w, score)
}

// riskScoringLoop periodically recalculates risk scores for all active hosts
func (s *Server) riskScoringLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	log.Println("[RiskScorer] started, recalculating every 30s")
	for range ticker.C {
		s.recalculateRiskScores()
	}
}

func (s *Server) recalculateRiskScores() {
	hosts := s.hostManager.GetAllHosts()
	if len(hosts) == 0 {
		return
	}

	// Get active flows to count per-host flows
	activeFlows := s.flowManager.GetActiveFlows()
	flowCountByIP := make(map[string]int)
	encryptedBytesByIP := make(map[string]uint64)
	totalBytesByIP := make(map[string]uint64)
	for _, f := range activeFlows {
		flowCountByIP[f.SrcIP]++
		flowCountByIP[f.DstIP]++
		bytes := f.BytesSent + f.BytesRecv
		totalBytesByIP[f.SrcIP] += bytes
		totalBytesByIP[f.DstIP] += bytes
		// Check if flow uses encrypted protocol
		proto := f.L7Protocol
		if isEncryptedProto(proto) {
			encryptedBytesByIP[f.SrcIP] += bytes
			encryptedBytesByIP[f.DstIP] += bytes
		}
	}

	for _, host := range hosts {
		// Get alert count for this host (last 24h)
		alertCount := 0
		if s.alertEngine != nil {
			now := time.Now()
			since := now.Add(-24 * time.Hour)
			alertList, _, err := s.alertEngine.GetAlerts(alerts.AlertFilter{EntityID: host.IP, Start: &since, Limit: 100})
			if err == nil {
				alertCount = len(alertList)
			}
		}

		// Check blacklist
		blacklisted := false
		if s.blacklistMgr != nil {
			// We only check, not trigger alert again (CheckIP triggers alerts internally,
			// but it's idempotent due to cooldown)
			entry := s.blacklistMgr.CheckIP(host.IP)
			blacklisted = entry != nil
		}

		flowCount := flowCountByIP[host.IP]

		// Compute encrypted ratio
		var encryptedRatio float64
		if totalBytesByIP[host.IP] > 0 {
			encryptedRatio = float64(encryptedBytesByIP[host.IP]) / float64(totalBytesByIP[host.IP])
		} else {
			encryptedRatio = 1.0 // no data, assume safe
		}

		score := s.riskScorer.CalculateScore(host, alertCount, blacklisted, flowCount, encryptedRatio)
		host.RiskScore = score
	}
}

func isEncryptedProto(proto string) bool {
	switch proto {
	case "TLS", "SSL", "HTTPS", "SSH", "QUIC", "DTLS",
		"tls", "ssl", "https", "ssh", "quic", "dtls":
		return true
	}
	return false
}
