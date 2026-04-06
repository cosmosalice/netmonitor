package main

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/gopacket"
	"github.com/gorilla/mux"
)

// GET /api/v1/pcap/live?filter=&duration=10
func (s *Server) handlePCAPLive(w http.ResponseWriter, r *http.Request) {
	if s.pcapWriter == nil {
		writeError(w, "pcap writer not initialized", http.StatusServiceUnavailable)
		return
	}

	filterStr := r.URL.Query().Get("filter")
	durationStr := r.URL.Query().Get("duration")

	duration := 10
	if durationStr != "" {
		if v, err := strconv.Atoi(durationStr); err == nil && v > 0 && v <= 3600 {
			duration = v
		}
	}

	// Build combined filter
	var filterFn func(gopacket.Packet) bool

	durationFilter := s.pcapWriter.FilterByDuration(duration)
	bpfFilter := s.pcapWriter.FilterByBPF(filterStr)

	filterFn = func(pkt gopacket.Packet) bool {
		return durationFilter(pkt) && bpfFilter(pkt)
	}

	filename := fmt.Sprintf("capture_%s.pcap", time.Now().Format("20060102_150405"))
	w.Header().Set("Content-Type", "application/vnd.tcpdump.pcap")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	if err := s.pcapWriter.WritePCAP(w, filterFn); err != nil {
		// Headers already sent, just log
		fmt.Printf("pcap write error: %v\n", err)
	}
}

// GET /api/v1/pcap/flow/{id}
func (s *Server) handlePCAPFlow(w http.ResponseWriter, r *http.Request) {
	if s.pcapWriter == nil {
		writeError(w, "pcap writer not initialized", http.StatusServiceUnavailable)
		return
	}

	vars := mux.Vars(r)
	flowID := vars["id"]
	if flowID == "" {
		writeError(w, "flow id is required", http.StatusBadRequest)
		return
	}

	filter := s.pcapWriter.FilterByFlow(flowID)

	filename := fmt.Sprintf("flow_%s.pcap", time.Now().Format("20060102_150405"))
	w.Header().Set("Content-Type", "application/vnd.tcpdump.pcap")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	if err := s.pcapWriter.WritePCAP(w, filter); err != nil {
		fmt.Printf("pcap write error: %v\n", err)
	}
}

// GET /api/v1/pcap/host/{ip}
func (s *Server) handlePCAPHost(w http.ResponseWriter, r *http.Request) {
	if s.pcapWriter == nil {
		writeError(w, "pcap writer not initialized", http.StatusServiceUnavailable)
		return
	}

	vars := mux.Vars(r)
	ip := vars["ip"]
	if ip == "" {
		writeError(w, "ip is required", http.StatusBadRequest)
		return
	}

	filter := s.pcapWriter.FilterByHost(ip)

	filename := fmt.Sprintf("host_%s_%s.pcap", ip, time.Now().Format("20060102_150405"))
	w.Header().Set("Content-Type", "application/vnd.tcpdump.pcap")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	if err := s.pcapWriter.WritePCAP(w, filter); err != nil {
		fmt.Printf("pcap write error: %v\n", err)
	}
}

// GET /api/v1/hosts/{ip}/os
func (s *Server) handleHostOS(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	ip := vars["ip"]
	if ip == "" {
		writeError(w, "ip is required", http.StatusBadRequest)
		return
	}

	if s.osFingerprint == nil {
		writeError(w, "os fingerprint not initialized", http.StatusServiceUnavailable)
		return
	}

	info := s.osFingerprint.GetOSInfo(ip)
	if info == nil {
		writeJSON(w, map[string]interface{}{
			"ip": ip,
			"os": nil,
		})
		return
	}

	writeJSON(w, map[string]interface{}{
		"ip": ip,
		"os": info,
	})
}
