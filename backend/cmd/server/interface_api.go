package main

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
)

// GET /api/v1/interfaces - List all available interfaces
func (s *Server) listInterfacesHandler(w http.ResponseWriter, r *http.Request) {
	interfaces, err := s.interfaceMgr.ListInterfaces()
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]interface{}{"interfaces": interfaces})
}

// GET /api/v1/interfaces/active - Get currently active interfaces
func (s *Server) getActiveInterfacesHandler(w http.ResponseWriter, r *http.Request) {
	interfaces := s.interfaceMgr.GetActiveInterfaces()
	writeJSON(w, map[string]interface{}{"interfaces": interfaces})
}

// POST /api/v1/interfaces/{name}/enable - Enable interface capture
func (s *Server) enableInterfaceHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]
	if name == "" {
		writeError(w, "interface name is required", http.StatusBadRequest)
		return
	}

	var req struct {
		BPFFilter string `json:"bpf_filter,omitempty"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if err := s.interfaceMgr.EnableInterface(name, req.BPFFilter); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"status": "enabled"})
}

// POST /api/v1/interfaces/{name}/disable - Disable interface capture
func (s *Server) disableInterfaceHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]
	if name == "" {
		writeError(w, "interface name is required", http.StatusBadRequest)
		return
	}

	if err := s.interfaceMgr.DisableInterface(name); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"status": "disabled"})
}

// GET /api/v1/interfaces/{name}/stats - Get interface statistics
func (s *Server) getInterfaceStatsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]
	if name == "" {
		writeError(w, "interface name is required", http.StatusBadRequest)
		return
	}

	stats, err := s.interfaceMgr.GetInterfaceStats(name)
	if err != nil {
		writeError(w, err.Error(), http.StatusNotFound)
		return
	}

	writeJSON(w, map[string]interface{}{"interface": stats})
}

// GET /api/v1/interfaces/stats/aggregate - Get aggregated statistics
func (s *Server) getInterfacesAggregateStats(w http.ResponseWriter, r *http.Request) {
	stats := s.interfaceMgr.GetTotalStats()
	writeJSON(w, stats)
}
