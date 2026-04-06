package main

import (
	"net/http"
	"strconv"

	"github.com/netmonitor/backend/storage"
)

// GET /api/v1/geo/hosts?limit=100
func (s *Server) getGeoHosts(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
			limit = v
		}
	}

	hosts, err := s.db.GetGeoHosts(limit)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if hosts == nil {
		hosts = []storage.HostWithGeo{}
	}
	writeJSON(w, map[string]interface{}{
		"hosts":   hosts,
		"enabled": s.geoIPMgr.IsEnabled(),
	})
}

// GET /api/v1/stats/countries
func (s *Server) getCountryStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.db.GetCountryStats()
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if stats == nil {
		stats = []storage.CountryStats{}
	}
	writeJSON(w, map[string]interface{}{"countries": stats})
}

// GET /api/v1/stats/asn
func (s *Server) getASNStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.db.GetASNStats()
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if stats == nil {
		stats = []storage.ASNStats{}
	}
	writeJSON(w, map[string]interface{}{"asn_stats": stats})
}
