package main

import (
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

// TopologyNode represents a node in the network topology
type TopologyNode struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Type  string `json:"type"` // pc, server, gateway, external
	Bytes uint64 `json:"bytes"`
	Flows int    `json:"flows"`
}

// TopologyLink represents a link between two nodes
type TopologyLink struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Bytes  uint64 `json:"bytes"`
	Flows  int    `json:"flows"`
}

// TopologyData represents the complete topology graph
type TopologyData struct {
	Nodes []TopologyNode `json:"nodes"`
	Links []TopologyLink `json:"links"`
}

// GET /api/v1/topology
func (s *Server) getTopology(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 30
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
			limit = v
		}
	}

	// Build topology from active flows
	topology := s.buildTopology(limit)
	writeJSON(w, topology)
}

func (s *Server) buildTopology(limit int) TopologyData {
	flows := s.flowManager.GetActiveFlows()

	// Aggregate nodes and links
	nodeMap := make(map[string]*TopologyNode)
	linkMap := make(map[string]*TopologyLink)

	for _, flow := range flows {
		// Determine node types
		srcType := getNodeType(flow.SrcIP)
		dstType := getNodeType(flow.DstIP)

		// Calculate total bytes for the flow
		totalBytes := flow.BytesSent + flow.BytesRecv

		// Update or create source node
		if node, exists := nodeMap[flow.SrcIP]; exists {
			node.Bytes += totalBytes
			node.Flows++
		} else {
			nodeMap[flow.SrcIP] = &TopologyNode{
				ID:    flow.SrcIP,
				Label: flow.SrcIP,
				Type:  srcType,
				Bytes: totalBytes,
				Flows: 1,
			}
		}

		// Update or create destination node
		if node, exists := nodeMap[flow.DstIP]; exists {
			node.Bytes += totalBytes
			node.Flows++
		} else {
			nodeMap[flow.DstIP] = &TopologyNode{
				ID:    flow.DstIP,
				Label: flow.DstIP,
				Type:  dstType,
				Bytes: totalBytes,
				Flows: 1,
			}
		}

		// Create or update link (use ordered pair to avoid duplicates)
		linkKey := flow.SrcIP + "->" + flow.DstIP
		if link, exists := linkMap[linkKey]; exists {
			link.Bytes += totalBytes
			link.Flows++
		} else {
			linkMap[linkKey] = &TopologyLink{
				Source: flow.SrcIP,
				Target: flow.DstIP,
				Bytes:  totalBytes,
				Flows:  1,
			}
		}
	}

	// Convert maps to slices and apply limit
	nodes := make([]TopologyNode, 0, len(nodeMap))
	for _, node := range nodeMap {
		nodes = append(nodes, *node)
	}

	links := make([]TopologyLink, 0, len(linkMap))
	for _, link := range linkMap {
		links = append(links, *link)
	}

	// Sort nodes by bytes (descending) and apply limit
	sortNodesByBytes(nodes)
	if len(nodes) > limit {
		nodes = nodes[:limit]
		// Filter links to only include nodes in the limited set
		nodeSet := make(map[string]bool)
		for _, n := range nodes {
			nodeSet[n.ID] = true
		}
		filteredLinks := make([]TopologyLink, 0)
		for _, link := range links {
			if nodeSet[link.Source] && nodeSet[link.Target] {
				filteredLinks = append(filteredLinks, link)
			}
		}
		links = filteredLinks
	}

	return TopologyData{
		Nodes: nodes,
		Links: links,
	}
}

// getNodeType determines the type of a node based on IP address
func getNodeType(ip string) string {
	// Check if it's a private IP
	if isPrivateIP(ip) {
		// Simple heuristics for local network nodes
		// Gateway: typically ends with .1 or .254
		if isGatewayIP(ip) {
			return "gateway"
		}
		return "pc"
	}
	return "external"
}

// isPrivateIP checks if an IP address is private (RFC 1918)
func isPrivateIP(ip string) bool {
	// Simple check for common private IP ranges
	// 10.0.0.0/8
	if len(ip) >= 3 && ip[:3] == "10." {
		return true
	}
	// 172.16.0.0/12
	if len(ip) >= 7 && ip[:7] == "172.16." {
		return true
	}
	if len(ip) >= 7 && ip[:7] == "172.17." {
		return true
	}
	if len(ip) >= 7 && ip[:7] == "172.18." {
		return true
	}
	if len(ip) >= 7 && ip[:7] == "172.19." {
		return true
	}
	if len(ip) >= 7 && ip[:7] == "172.20." {
		return true
	}
	if len(ip) >= 7 && ip[:7] == "172.21." {
		return true
	}
	if len(ip) >= 7 && ip[:7] == "172.22." {
		return true
	}
	if len(ip) >= 7 && ip[:7] == "172.23." {
		return true
	}
	if len(ip) >= 7 && ip[:7] == "172.24." {
		return true
	}
	if len(ip) >= 7 && ip[:7] == "172.25." {
		return true
	}
	if len(ip) >= 7 && ip[:7] == "172.26." {
		return true
	}
	if len(ip) >= 7 && ip[:7] == "172.27." {
		return true
	}
	if len(ip) >= 7 && ip[:7] == "172.28." {
		return true
	}
	if len(ip) >= 7 && ip[:7] == "172.29." {
		return true
	}
	if len(ip) >= 7 && ip[:7] == "172.30." {
		return true
	}
	if len(ip) >= 7 && ip[:7] == "172.31." {
		return true
	}
	// 192.168.0.0/16
	if len(ip) >= 8 && ip[:8] == "192.168." {
		return true
	}
	// 127.0.0.0/8 (localhost)
	if len(ip) >= 4 && ip[:4] == "127." {
		return true
	}
	return false
}

// isGatewayIP checks if IP looks like a gateway/router
func isGatewayIP(ip string) bool {
	// Common gateway patterns
	if len(ip) >= 8 && ip[:8] == "192.168." {
		// Check for .1 or .254
		for i := len(ip) - 1; i >= 0; i-- {
			if ip[i] == '.' {
				suffix := ip[i+1:]
				return suffix == "1" || suffix == "254"
			}
		}
	}
	if len(ip) >= 3 && ip[:3] == "10." {
		for i := len(ip) - 1; i >= 0; i-- {
			if ip[i] == '.' {
				suffix := ip[i+1:]
				return suffix == "1" || suffix == "254"
			}
		}
	}
	return false
}

// sortNodesByBytes sorts nodes by bytes in descending order
func sortNodesByBytes(nodes []TopologyNode) {
	for i := 0; i < len(nodes); i++ {
		for j := i + 1; j < len(nodes); j++ {
			if nodes[j].Bytes > nodes[i].Bytes {
				nodes[i], nodes[j] = nodes[j], nodes[i]
			}
		}
	}
}

// RegisterTopologyRoutes registers topology API routes
func RegisterTopologyRoutes(r *mux.Router, s *Server) {
	r.HandleFunc("/api/v1/topology", s.getTopology).Methods("GET", "OPTIONS")
}
