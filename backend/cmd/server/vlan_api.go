package main

import (
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

// VLANInfo represents VLAN statistics
type VLANInfo struct {
	VLANID    uint16   `json:"vlan_id"`
	HostCount int      `json:"host_count"`
	Bytes     uint64   `json:"bytes"`
	Flows     int      `json:"flows"`
	Hosts     []string `json:"hosts,omitempty"`
}

// VLANHost represents a host in a VLAN
type VLANHost struct {
	IP        string `json:"ip"`
	BytesSent uint64 `json:"bytes_sent"`
	BytesRecv uint64 `json:"bytes_recv"`
	Flows     int    `json:"flows"`
}

// VLANFlow represents a flow in a VLAN
type VLANFlow struct {
	FlowID     string `json:"flow_id"`
	SrcIP      string `json:"src_ip"`
	DstIP      string `json:"dst_ip"`
	SrcPort    uint16 `json:"src_port"`
	DstPort    uint16 `json:"dst_port"`
	Protocol   string `json:"protocol"`
	BytesSent  uint64 `json:"bytes_sent"`
	BytesRecv  uint64 `json:"bytes_recv"`
	L7Protocol string `json:"l7_protocol"`
}

// GET /api/v1/vlans - List all VLANs with stats
func (s *Server) getVLANs(w http.ResponseWriter, r *http.Request) {
	vlans := s.buildVLANStats()
	writeJSON(w, map[string]interface{}{"vlans": vlans})
}

// GET /api/v1/vlans/{id}/hosts - Get hosts in a specific VLAN
func (s *Server) getVLANHosts(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	vlanIDStr := vars["id"]
	vlanID, err := strconv.ParseUint(vlanIDStr, 10, 16)
	if err != nil {
		writeError(w, "invalid VLAN ID", http.StatusBadRequest)
		return
	}

	hosts := s.getHostsByVLAN(uint16(vlanID))
	writeJSON(w, map[string]interface{}{"vlan_id": vlanID, "hosts": hosts})
}

// GET /api/v1/vlans/{id}/flows - Get flows in a specific VLAN
func (s *Server) getVLANFlows(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	vlanIDStr := vars["id"]
	vlanID, err := strconv.ParseUint(vlanIDStr, 10, 16)
	if err != nil {
		writeError(w, "invalid VLAN ID", http.StatusBadRequest)
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
			limit = v
		}
	}

	flows := s.getFlowsByVLAN(uint16(vlanID), limit)
	writeJSON(w, map[string]interface{}{"vlan_id": vlanID, "flows": flows})
}

// buildVLANStats aggregates VLAN statistics from active flows
func (s *Server) buildVLANStats() []VLANInfo {
	flows := s.flowManager.GetActiveFlows()

	// Aggregate by VLAN
	vlanMap := make(map[uint16]*VLANInfo)
	hostVLANMap := make(map[string]uint16) // IP -> VLAN ID

	for _, flow := range flows {
		vlanID := flow.VLANID
		if vlanID == 0 {
			continue // Skip untagged traffic
		}

		totalBytes := flow.BytesSent + flow.BytesRecv

		if info, exists := vlanMap[vlanID]; exists {
			info.Bytes += totalBytes
			info.Flows++
			// Track unique hosts
			if !containsHost(info.Hosts, flow.SrcIP) {
				info.Hosts = append(info.Hosts, flow.SrcIP)
				info.HostCount++
			}
			if !containsHost(info.Hosts, flow.DstIP) {
				info.Hosts = append(info.Hosts, flow.DstIP)
				info.HostCount++
			}
		} else {
			vlanMap[vlanID] = &VLANInfo{
				VLANID:    vlanID,
				HostCount: 2, // src and dst
				Bytes:     totalBytes,
				Flows:     1,
				Hosts:     []string{flow.SrcIP, flow.DstIP},
			}
		}

		// Track host-VLAN mapping
		hostVLANMap[flow.SrcIP] = vlanID
		hostVLANMap[flow.DstIP] = vlanID
	}

	// Convert to slice
	result := make([]VLANInfo, 0, len(vlanMap))
	for _, info := range vlanMap {
		result = append(result, *info)
	}

	return result
}

// getHostsByVLAN returns hosts associated with a specific VLAN
func (s *Server) getHostsByVLAN(vlanID uint16) []VLANHost {
	flows := s.flowManager.GetActiveFlows()

	// Aggregate hosts by IP for this VLAN
	hostMap := make(map[string]*VLANHost)

	for _, flow := range flows {
		if flow.VLANID != vlanID {
			continue
		}

		// Source host
		if host, exists := hostMap[flow.SrcIP]; exists {
			host.BytesSent += flow.BytesSent
			host.BytesRecv += flow.BytesRecv
			host.Flows++
		} else {
			hostMap[flow.SrcIP] = &VLANHost{
				IP:        flow.SrcIP,
				BytesSent: flow.BytesSent,
				BytesRecv: flow.BytesRecv,
				Flows:     1,
			}
		}

		// Destination host
		if host, exists := hostMap[flow.DstIP]; exists {
			host.BytesSent += flow.BytesRecv // Reverse perspective
			host.BytesRecv += flow.BytesSent
			host.Flows++
		} else {
			hostMap[flow.DstIP] = &VLANHost{
				IP:        flow.DstIP,
				BytesSent: flow.BytesRecv,
				BytesRecv: flow.BytesSent,
				Flows:     1,
			}
		}
	}

	// Convert to slice
	result := make([]VLANHost, 0, len(hostMap))
	for _, host := range hostMap {
		result = append(result, *host)
	}

	return result
}

// getFlowsByVLAN returns flows for a specific VLAN
func (s *Server) getFlowsByVLAN(vlanID uint16, limit int) []VLANFlow {
	flows := s.flowManager.GetActiveFlows()

	result := make([]VLANFlow, 0)
	count := 0

	for _, flow := range flows {
		if flow.VLANID != vlanID {
			continue
		}

		result = append(result, VLANFlow{
			FlowID:     flow.ID,
			SrcIP:      flow.SrcIP,
			DstIP:      flow.DstIP,
			SrcPort:    flow.SrcPort,
			DstPort:    flow.DstPort,
			Protocol:   flow.Protocol,
			BytesSent:  flow.BytesSent,
			BytesRecv:  flow.BytesRecv,
			L7Protocol: flow.L7Protocol,
		})

		count++
		if count >= limit {
			break
		}
	}

	return result
}

// containsHost checks if a host exists in a slice
func containsHost(hosts []string, ip string) bool {
	for _, h := range hosts {
		if h == ip {
			return true
		}
	}
	return false
}
