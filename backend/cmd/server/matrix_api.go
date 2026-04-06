package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Traffic Matrix Handler
// ---------------------------------------------------------------------------

// GET /api/v1/stats/traffic-matrix
func (s *Server) handleTrafficMatrix(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	// Parse time range (default: last 1 hour)
	now := time.Now()
	start := now.Add(-1 * time.Hour)
	end := now

	if v := q.Get("start"); v != "" {
		if t, ok := parseTimeParam(v); ok {
			start = t
		}
	}
	if v := q.Get("end"); v != "" {
		if t, ok := parseTimeParam(v); ok {
			end = t
		}
	}

	// Parse limit
	limit := 20
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 50 {
		limit = 50
	}

	// Parse group_by
	groupBy := q.Get("group_by")
	if groupBy == "" {
		groupBy = "host"
	}
	if groupBy != "host" && groupBy != "subnet" {
		writeError(w, "group_by must be 'host' or 'subnet'", http.StatusBadRequest)
		return
	}

	// Query traffic pairs from database
	pairs, err := s.db.GetTrafficPairs(start, end, limit)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Build the matrix response
	var nodes []string
	nodeIndex := make(map[string]int)

	// Helper to get node key based on group_by
	getNodeKey := func(ip string) string {
		if groupBy == "subnet" {
			return ipToSubnet(ip)
		}
		return ip
	}

	// Collect all unique nodes
	for _, p := range pairs {
		srcKey := getNodeKey(p.SrcIP)
		dstKey := getNodeKey(p.DstIP)
		if _, exists := nodeIndex[srcKey]; !exists {
			nodeIndex[srcKey] = len(nodes)
			nodes = append(nodes, srcKey)
		}
		if _, exists := nodeIndex[dstKey]; !exists {
			nodeIndex[dstKey] = len(nodes)
			nodes = append(nodes, dstKey)
		}
	}

	n := len(nodes)
	// Build N×N matrix
	matrix := make([][]uint64, n)
	for i := range matrix {
		matrix[i] = make([]uint64, n)
	}

	// Details map: "src->dst" -> detail info
	type pairDetail struct {
		Bytes     uint64   `json:"bytes"`
		Flows     int      `json:"flows"`
		Protocols []string `json:"protocols"`
	}
	details := make(map[string]pairDetail)

	for _, p := range pairs {
		srcKey := getNodeKey(p.SrcIP)
		dstKey := getNodeKey(p.DstIP)
		srcIdx := nodeIndex[srcKey]
		dstIdx := nodeIndex[dstKey]

		matrix[srcIdx][dstIdx] += p.Bytes

		detailKey := fmt.Sprintf("%s->%s", srcKey, dstKey)
		existing, ok := details[detailKey]
		if ok {
			existing.Bytes += p.Bytes
			existing.Flows += p.FlowCount
			// Merge protocols
			protoSet := make(map[string]bool)
			for _, pr := range existing.Protocols {
				protoSet[pr] = true
			}
			for _, pr := range p.Protocols {
				protoSet[pr] = true
			}
			merged := make([]string, 0, len(protoSet))
			for pr := range protoSet {
				merged = append(merged, pr)
			}
			existing.Protocols = merged
			details[detailKey] = existing
		} else {
			protos := p.Protocols
			if protos == nil {
				protos = []string{}
			}
			details[detailKey] = pairDetail{
				Bytes:     p.Bytes,
				Flows:     p.FlowCount,
				Protocols: protos,
			}
		}
	}

	if nodes == nil {
		nodes = []string{}
	}

	writeJSON(w, map[string]interface{}{
		"nodes":   nodes,
		"matrix":  matrix,
		"details": details,
	})
}

// ipToSubnet extracts /24 subnet from an IPv4 address.
// e.g., "192.168.1.100" -> "192.168.1.0/24"
func ipToSubnet(ip string) string {
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return ip // return as-is for non-IPv4
	}
	return fmt.Sprintf("%s.%s.%s.0/24", parts[0], parts[1], parts[2])
}
