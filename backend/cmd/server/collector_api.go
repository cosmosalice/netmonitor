package main

import (
	"encoding/json"
	"net/http"
)

// CollectorStatus represents the status of a flow collector
type CollectorStatus struct {
	Type            string `json:"type"`
	Running         bool   `json:"running"`
	Port            int    `json:"port"`
	PacketsReceived uint64 `json:"packets_received"`
	FlowsReceived   uint64 `json:"flows_received"`
	BytesReceived   uint64 `json:"bytes_received"`
	Errors          uint64 `json:"errors"`
}

// GET /api/v1/collectors - List collector status
func (s *Server) listCollectorsHandler(w http.ResponseWriter, r *http.Request) {
	collectors := []CollectorStatus{}

	// NetFlow collector status
	if s.netflowCollector != nil {
		stats := s.netflowCollector.GetStats()
		collectors = append(collectors, CollectorStatus{
			Type:            "netflow",
			Running:         s.netflowCollector.IsRunning(),
			Port:            s.netflowPort,
			PacketsReceived: stats.PacketsReceived,
			FlowsReceived:   stats.FlowsReceived,
			BytesReceived:   stats.BytesReceived,
			Errors:          stats.Errors,
		})
	}

	// sFlow collector status
	if s.sflowCollector != nil {
		stats := s.sflowCollector.GetStats()
		collectors = append(collectors, CollectorStatus{
			Type:            "sflow",
			Running:         s.sflowCollector.IsRunning(),
			Port:            s.sflowPort,
			PacketsReceived: stats.PacketsReceived,
			FlowsReceived:   stats.FlowsReceived,
			BytesReceived:   stats.BytesReceived,
			Errors:          stats.Errors,
		})
	}

	writeJSON(w, map[string]interface{}{"collectors": collectors})
}

// POST /api/v1/collectors/netflow/start - Start NetFlow collector
func (s *Server) startNetFlowCollectorHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Port int `json:"port"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Default port if not provided
		req.Port = 2055
	}

	if req.Port <= 0 {
		req.Port = 2055
	}

	if s.netflowCollector == nil {
		writeError(w, "NetFlow collector not initialized", http.StatusInternalServerError)
		return
	}

	if s.netflowCollector.IsRunning() {
		writeJSON(w, map[string]string{"status": "already_running"})
		return
	}

	if err := s.netflowCollector.Start(req.Port); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.netflowPort = req.Port
	writeJSON(w, map[string]string{"status": "started"})
}

// POST /api/v1/collectors/netflow/stop - Stop NetFlow collector
func (s *Server) stopNetFlowCollectorHandler(w http.ResponseWriter, r *http.Request) {
	if s.netflowCollector == nil {
		writeError(w, "NetFlow collector not initialized", http.StatusInternalServerError)
		return
	}

	if !s.netflowCollector.IsRunning() {
		writeJSON(w, map[string]string{"status": "not_running"})
		return
	}

	if err := s.netflowCollector.Stop(); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"status": "stopped"})
}

// POST /api/v1/collectors/sflow/start - Start sFlow collector
func (s *Server) startSFlowCollectorHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Port int `json:"port"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Default port if not provided
		req.Port = 6343
	}

	if req.Port <= 0 {
		req.Port = 6343
	}

	if s.sflowCollector == nil {
		writeError(w, "sFlow collector not initialized", http.StatusInternalServerError)
		return
	}

	if s.sflowCollector.IsRunning() {
		writeJSON(w, map[string]string{"status": "already_running"})
		return
	}

	if err := s.sflowCollector.Start(req.Port); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.sflowPort = req.Port
	writeJSON(w, map[string]string{"status": "started"})
}

// POST /api/v1/collectors/sflow/stop - Stop sFlow collector
func (s *Server) stopSFlowCollectorHandler(w http.ResponseWriter, r *http.Request) {
	if s.sflowCollector == nil {
		writeError(w, "sFlow collector not initialized", http.StatusInternalServerError)
		return
	}

	if !s.sflowCollector.IsRunning() {
		writeJSON(w, map[string]string{"status": "not_running"})
		return
	}

	if err := s.sflowCollector.Stop(); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"status": "stopped"})
}

// GET /api/v1/collectors/stats - Get collector statistics
func (s *Server) getCollectorStatsHandler(w http.ResponseWriter, r *http.Request) {
	stats := map[string]interface{}{}

	if s.netflowCollector != nil {
		nfStats := s.netflowCollector.GetStats()
		stats["netflow"] = map[string]interface{}{
			"running":          s.netflowCollector.IsRunning(),
			"port":             s.netflowPort,
			"packets_received": nfStats.PacketsReceived,
			"flows_received":   nfStats.FlowsReceived,
			"bytes_received":   nfStats.BytesReceived,
			"errors":           nfStats.Errors,
		}
	}

	if s.sflowCollector != nil {
		sfStats := s.sflowCollector.GetStats()
		stats["sflow"] = map[string]interface{}{
			"running":          s.sflowCollector.IsRunning(),
			"port":             s.sflowPort,
			"packets_received": sfStats.PacketsReceived,
			"flows_received":   sfStats.FlowsReceived,
			"bytes_received":   sfStats.BytesReceived,
			"errors":           sfStats.Errors,
		}
	}

	writeJSON(w, stats)
}

// GET /api/v1/collectors/flows - Get recent flows from collectors
func (s *Server) getCollectorFlowsHandler(w http.ResponseWriter, r *http.Request) {
	// Return recent flows from collector buffer
	// This is a simplified version - in production you'd want a ring buffer
	flows := []map[string]interface{}{}

	writeJSON(w, map[string]interface{}{
		"flows": flows,
		"total": len(flows),
	})
}
