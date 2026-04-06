package main

import (
	"net/http"
	"strconv"
)

// ---------------------------------------------------------------------------
// DNS Analysis API handlers
// ---------------------------------------------------------------------------

// GET /api/v1/dns/summary
func (s *Server) getDNSSummary(w http.ResponseWriter, r *http.Request) {
	summary := s.dnsStats.GetSummary()
	writeJSON(w, summary)
}

// GET /api/v1/dns/domains?limit=50
func (s *Server) getDNSDomains(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	domains := s.dnsStats.GetTopDomains(limit)
	writeJSON(w, map[string]interface{}{"domains": domains})
}

// GET /api/v1/dns/servers?limit=20
func (s *Server) getDNSServers(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	servers := s.dnsStats.GetTopDNSServers(limit)
	writeJSON(w, map[string]interface{}{"servers": servers})
}

// GET /api/v1/dns/response-codes
func (s *Server) getDNSResponseCodes(w http.ResponseWriter, r *http.Request) {
	codes := s.dnsStats.GetResponseCodeStats()
	writeJSON(w, map[string]interface{}{"response_codes": codes})
}

// GET /api/v1/dns/query-types
func (s *Server) getDNSQueryTypes(w http.ResponseWriter, r *http.Request) {
	types := s.dnsStats.GetQueryTypeStats()
	writeJSON(w, map[string]interface{}{"query_types": types})
}
