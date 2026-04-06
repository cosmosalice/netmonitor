package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/netmonitor/backend/monitoring"
)

// GET /api/v1/monitoring/probes - List all monitoring probes
func (s *Server) getMonitoringProbes(w http.ResponseWriter, r *http.Request) {
	targets := s.monitor.GetTargets()
	summary := s.monitor.GetSummary()

	writeJSON(w, map[string]interface{}{
		"probes":  targets,
		"summary": summary,
	})
}

// POST /api/v1/monitoring/probes - Create a new monitoring probe
func (s *Server) createMonitoringProbe(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		Type     string `json:"type"`     // ping, tcp, http
		Host     string `json:"host"`     // for ping and tcp
		Port     int    `json:"port"`     // for tcp
		URL      string `json:"url"`      // for http
		Interval int    `json:"interval"` // seconds
		Timeout  int    `json:"timeout"`  // milliseconds
		Enabled  bool   `json:"enabled"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	target := monitoring.MonitorTarget{
		Name:     req.Name,
		Type:     req.Type,
		Host:     req.Host,
		Port:     req.Port,
		URL:      req.URL,
		Interval: req.Interval,
		Timeout:  req.Timeout,
		Enabled:  req.Enabled,
	}

	if err := s.monitor.AddTarget(target); err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, map[string]interface{}{"status": "ok", "probe": target})
}

// DELETE /api/v1/monitoring/probes/{id} - Delete a monitoring probe
func (s *Server) deleteMonitoringProbe(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		writeError(w, "probe id is required", http.StatusBadRequest)
		return
	}

	s.monitor.RemoveTarget(id)
	writeJSON(w, map[string]string{"status": "deleted"})
}

// GET /api/v1/monitoring/probes/{id} - Get probe details
func (s *Server) getMonitoringProbe(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		writeError(w, "probe id is required", http.StatusBadRequest)
		return
	}

	target := s.monitor.GetTarget(id)
	if target == nil {
		writeError(w, "probe not found", http.StatusNotFound)
		return
	}

	results := s.monitor.GetResults(id, 100)

	writeJSON(w, map[string]interface{}{
		"probe":   target,
		"results": results,
	})
}

// GET /api/v1/monitoring/probes/{id}/results - Get probe history results
func (s *Server) getMonitoringProbeResults(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		writeError(w, "probe id is required", http.StatusBadRequest)
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	results := s.monitor.GetResults(id, limit)
	writeJSON(w, map[string]interface{}{"results": results})
}

// POST /api/v1/monitoring/probes/{id}/test - Run a test immediately
func (s *Server) testMonitoringProbe(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		writeError(w, "probe id is required", http.StatusBadRequest)
		return
	}

	target := s.monitor.GetTarget(id)
	if target == nil {
		writeError(w, "probe not found", http.StatusNotFound)
		return
	}

	result := s.monitor.CheckTarget(target)
	writeJSON(w, map[string]interface{}{"result": result})
}
