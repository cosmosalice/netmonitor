package main

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/netmonitor/backend/snmp"
)

// GET /api/v1/snmp/devices - List all SNMP devices
func (s *Server) getSNMPDevices(w http.ResponseWriter, r *http.Request) {
	devices := s.snmpManager.GetDevices()
	writeJSON(w, map[string]interface{}{"devices": devices})
}

// POST /api/v1/snmp/devices - Add a new SNMP device
func (s *Server) addSNMPDevice(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string `json:"name"`
		IP        string `json:"ip"`
		Community string `json:"community"`
		Version   string `json:"version"`
		Port      uint16 `json:"port"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Validate IP
	if !snmp.IsValidIP(req.IP) {
		writeError(w, "invalid IP address", http.StatusBadRequest)
		return
	}

	device := snmp.SNMPDevice{
		Name:      req.Name,
		IP:        req.IP,
		Community: req.Community,
		Version:   req.Version,
		Port:      req.Port,
		Enabled:   true,
	}

	if err := s.snmpManager.AddDevice(device); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]interface{}{"status": "ok", "device": device})
}

// DELETE /api/v1/snmp/devices/{id} - Delete an SNMP device
func (s *Server) deleteSNMPDevice(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		writeError(w, "device id is required", http.StatusBadRequest)
		return
	}

	s.snmpManager.RemoveDevice(id)
	writeJSON(w, map[string]string{"status": "deleted"})
}

// GET /api/v1/snmp/devices/{id} - Get SNMP device details
func (s *Server) getSNMPDevice(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		writeError(w, "device id is required", http.StatusBadRequest)
		return
	}

	device := s.snmpManager.GetDevice(id)
	if device == nil {
		writeError(w, "device not found", http.StatusNotFound)
		return
	}

	writeJSON(w, map[string]interface{}{"device": device})
}

// POST /api/v1/snmp/devices/{id}/poll - Manually poll an SNMP device
func (s *Server) pollSNMPDevice(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		writeError(w, "device id is required", http.StatusBadRequest)
		return
	}

	device := s.snmpManager.GetDevice(id)
	if device == nil {
		writeError(w, "device not found", http.StatusNotFound)
		return
	}

	if err := s.snmpManager.PollDevice(device); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get updated device
	device = s.snmpManager.GetDevice(id)
	writeJSON(w, map[string]interface{}{"status": "ok", "device": device})
}
