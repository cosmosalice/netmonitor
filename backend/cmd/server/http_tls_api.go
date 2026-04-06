package main

import (
	"net/http"
	"strconv"
)

// ---------------------------------------------------------------------------
// HTTP Analysis API
// ---------------------------------------------------------------------------

// GET /api/v1/http/summary
func (s *Server) getHTTPSummary(w http.ResponseWriter, r *http.Request) {
	summary := s.httpStats.GetSummary()
	writeJSON(w, summary)
}

// GET /api/v1/http/hosts?limit=50
func (s *Server) getHTTPHosts(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	hosts := s.httpStats.GetTopHosts(limit)
	writeJSON(w, map[string]interface{}{"hosts": hosts})
}

// GET /api/v1/http/user-agents?limit=30
func (s *Server) getHTTPUserAgents(w http.ResponseWriter, r *http.Request) {
	limit := 30
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	agents := s.httpStats.GetTopUserAgents(limit)
	writeJSON(w, map[string]interface{}{"user_agents": agents})
}

// GET /api/v1/http/methods
func (s *Server) getHTTPMethods(w http.ResponseWriter, r *http.Request) {
	methods := s.httpStats.GetMethods()
	writeJSON(w, map[string]interface{}{"methods": methods})
}

// GET /api/v1/http/status-codes
func (s *Server) getHTTPStatusCodes(w http.ResponseWriter, r *http.Request) {
	codes := s.httpStats.GetStatusCodes()
	writeJSON(w, map[string]interface{}{"status_codes": codes})
}

// ---------------------------------------------------------------------------
// TLS Analysis API
// ---------------------------------------------------------------------------

// GET /api/v1/tls/summary
func (s *Server) getTLSSummary(w http.ResponseWriter, r *http.Request) {
	summary := s.tlsStats.GetSummary()
	writeJSON(w, summary)
}

// GET /api/v1/tls/sni?limit=50
func (s *Server) getTLSSNI(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	sni := s.tlsStats.GetTopSNI(limit)
	writeJSON(w, map[string]interface{}{"sni_domains": sni})
}

// GET /api/v1/tls/ja3?limit=30
func (s *Server) getTLSJA3(w http.ResponseWriter, r *http.Request) {
	limit := 30
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	ja3 := s.tlsStats.GetTopJA3(limit)
	writeJSON(w, map[string]interface{}{"ja3_hashes": ja3})
}

// GET /api/v1/tls/versions
func (s *Server) getTLSVersions(w http.ResponseWriter, r *http.Request) {
	versions := s.tlsStats.GetTLSVersions()
	writeJSON(w, map[string]interface{}{"versions": versions})
}
