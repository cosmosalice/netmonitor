package main

import (
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

// GET /api/v1/devices - List all devices with pagination, sorting, and search
func (s *Server) getDevices(w http.ResponseWriter, r *http.Request) {
	if s.macTracker == nil {
		writeError(w, "MAC tracker not initialized", http.StatusInternalServerError)
		return
	}

	// Parse query parameters
	sortBy := r.URL.Query().Get("sort")
	if sortBy == "" {
		sortBy = "traffic"
	}

	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
			limit = v
		}
	}

	offset := 0
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if v, err := strconv.Atoi(offsetStr); err == nil && v >= 0 {
			offset = v
		}
	}

	search := r.URL.Query().Get("search")

	var devices []interface{}
	var total int

	if search != "" {
		// Search devices
		results := s.macTracker.SearchDevices(search)
		total = len(results)
		if offset < len(results) {
			end := offset + limit
			if end > len(results) {
				end = len(results)
			}
			for i := offset; i < end; i++ {
				devices = append(devices, results[i])
			}
		}
	} else {
		// Get all devices sorted
		allDevices := s.macTracker.GetDevicesSorted(sortBy, 0)
		total = len(allDevices)
		if offset < len(allDevices) {
			end := offset + limit
			if end > len(allDevices) {
				end = len(allDevices)
			}
			for i := offset; i < end; i++ {
				devices = append(devices, allDevices[i])
			}
		}
	}

	writeJSON(w, map[string]interface{}{
		"devices": devices,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

// GET /api/v1/devices/{mac} - Get device details by MAC address
func (s *Server) getDevice(w http.ResponseWriter, r *http.Request) {
	if s.macTracker == nil {
		writeError(w, "MAC tracker not initialized", http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(r)
	mac := vars["mac"]
	if mac == "" {
		writeError(w, "MAC address is required", http.StatusBadRequest)
		return
	}

	device := s.macTracker.GetDevice(mac)
	if device == nil {
		writeError(w, "Device not found", http.StatusNotFound)
		return
	}

	writeJSON(w, map[string]interface{}{
		"device": device,
	})
}

// GET /api/v1/devices/{mac}/flows - Get flows associated with a device
func (s *Server) getDeviceFlows(w http.ResponseWriter, r *http.Request) {
	if s.macTracker == nil {
		writeError(w, "MAC tracker not initialized", http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(r)
	mac := vars["mac"]
	if mac == "" {
		writeError(w, "MAC address is required", http.StatusBadRequest)
		return
	}

	device := s.macTracker.GetDevice(mac)
	if device == nil {
		writeError(w, "Device not found", http.StatusNotFound)
		return
	}

	// Get flows for all IPs associated with this device
	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
			limit = v
		}
	}

	// Get active flows matching device IPs
	activeFlows := s.flowManager.GetActiveFlows()
	var deviceFlows []map[string]interface{}

	for _, flow := range activeFlows {
		matched := false
		for _, ip := range device.IPs {
			if flow.SrcIP == ip || flow.DstIP == ip {
				matched = true
				break
			}
		}

		if matched {
			flowData := map[string]interface{}{
				"flow_id":      flow.ID,
				"src_ip":       flow.SrcIP,
				"dst_ip":       flow.DstIP,
				"src_port":     flow.SrcPort,
				"dst_port":     flow.DstPort,
				"protocol":     flow.Protocol,
				"l7_protocol":  flow.L7Protocol,
				"l7_category":  flow.L7Category,
				"bytes_sent":   flow.BytesSent,
				"bytes_recv":   flow.BytesRecv,
				"packets_sent": flow.PacketsSent,
				"packets_recv": flow.PacketsRecv,
				"start_time":   flow.StartTime,
				"last_seen":    flow.LastSeen,
				"is_active":    flow.IsActive,
			}
			deviceFlows = append(deviceFlows, flowData)

			if len(deviceFlows) >= limit {
				break
			}
		}
	}

	// If we have a database, also query historical flows
	if s.db != nil && len(deviceFlows) < limit {
		for _, ip := range device.IPs {
			rows, err := s.db.GetDB().Query(`
				SELECT flow_id, src_ip, dst_ip, src_port, dst_port, protocol, l7_protocol, 
					   bytes_sent, bytes_recv, packets_sent, packets_recv, start_time, end_time
				FROM flows 
				WHERE (src_ip = ? OR dst_ip = ?) AND is_active = 0
				ORDER BY end_time DESC
				LIMIT ?
			`, ip, ip, limit-len(deviceFlows))
			if err != nil {
				continue
			}
			defer rows.Close()

			for rows.Next() {
				var flowID, srcIP, dstIP, protocol, l7Protocol string
				var srcPort, dstPort int
				var bytesSent, bytesRecv, packetsSent, packetsRecv int64
				var startTime, endTime string

				if err := rows.Scan(&flowID, &srcIP, &dstIP, &srcPort, &dstPort, &protocol,
					&l7Protocol, &bytesSent, &bytesRecv, &packetsSent, &packetsRecv, &startTime, &endTime); err != nil {
					continue
				}

				flowData := map[string]interface{}{
					"flow_id":      flowID,
					"src_ip":       srcIP,
					"dst_ip":       dstIP,
					"src_port":     srcPort,
					"dst_port":     dstPort,
					"protocol":     protocol,
					"l7_protocol":  l7Protocol,
					"bytes_sent":   bytesSent,
					"bytes_recv":   bytesRecv,
					"packets_sent": packetsSent,
					"packets_recv": packetsRecv,
					"start_time":   startTime,
					"end_time":     endTime,
					"is_active":    false,
				}
				deviceFlows = append(deviceFlows, flowData)

				if len(deviceFlows) >= limit {
					break
				}
			}
		}
	}

	writeJSON(w, map[string]interface{}{
		"flows":      deviceFlows,
		"total":      len(deviceFlows),
		"device_mac": mac,
	})
}

// GET /api/v1/devices/stats - Get device statistics
func (s *Server) getDeviceStats(w http.ResponseWriter, r *http.Request) {
	if s.macTracker == nil {
		writeError(w, "MAC tracker not initialized", http.StatusInternalServerError)
		return
	}

	stats := s.macTracker.GetDeviceStats()
	writeJSON(w, stats)
}
