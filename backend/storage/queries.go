package storage

import (
	"fmt"
	"strings"
	"time"
)

// FlowDetail represents a complete flow record with TCP metrics
type FlowDetail struct {
	FlowID      string     `json:"flow_id"`
	SrcIP       string     `json:"src_ip"`
	DstIP       string     `json:"dst_ip"`
	SrcPort     int        `json:"src_port"`
	DstPort     int        `json:"dst_port"`
	Protocol    string     `json:"protocol"`
	L7Protocol  string     `json:"l7_protocol"`
	L7Category  string     `json:"l7_category"`
	BytesSent   int64      `json:"bytes_sent"`
	BytesRecv   int64      `json:"bytes_recv"`
	PacketsSent int64      `json:"packets_sent"`
	PacketsRecv int64      `json:"packets_recv"`
	StartTime   time.Time  `json:"start_time"`
	EndTime     *time.Time `json:"end_time,omitempty"`
	IsActive    bool       `json:"is_active"`
	// TCP performance metrics
	RTT             float64 `json:"rtt_ms"`
	MinRTT          float64 `json:"min_rtt_ms"`
	MaxRTT          float64 `json:"max_rtt_ms"`
	Retransmissions int     `json:"retransmissions"`
	OutOfOrder      int     `json:"out_of_order"`
	PacketLoss      int     `json:"packet_loss"`
	AvgWindowSize   int     `json:"avg_window_size"`
}

// QueryHosts queries host statistics
func (d *Database) QueryHosts(limit int) ([]map[string]interface{}, error) {
	rows, err := d.db.Query(`
		SELECT ip, mac, hostname, bytes_sent, bytes_recv, packets_sent, packets_recv,
		       first_seen, last_seen
		FROM hosts
		ORDER BY (bytes_sent + bytes_recv) DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hosts []map[string]interface{}
	for rows.Next() {
		var ip, mac, hostname string
		var bytesSent, bytesRecv, packetsSent, packetsRecv int64
		var firstSeen, lastSeen time.Time

		if err := rows.Scan(&ip, &mac, &hostname, &bytesSent, &bytesRecv,
			&packetsSent, &packetsRecv, &firstSeen, &lastSeen); err != nil {
			return nil, err
		}

		hosts = append(hosts, map[string]interface{}{
			"ip":           ip,
			"mac":          mac,
			"hostname":     hostname,
			"bytes_sent":   bytesSent,
			"bytes_recv":   bytesRecv,
			"packets_sent": packetsSent,
			"packets_recv": packetsRecv,
			"first_seen":   firstSeen,
			"last_seen":    lastSeen,
		})
	}

	return hosts, nil
}

// QueryProtocols queries protocol statistics
func (d *Database) QueryProtocols(limit int) ([]map[string]interface{}, error) {
	rows, err := d.db.Query(`
		SELECT protocol, category, SUM(bytes) as total_bytes, SUM(packets) as total_packets,
		       SUM(flow_count) as total_flows
		FROM protocol_stats
		GROUP BY protocol, category
		ORDER BY total_bytes DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var protocols []map[string]interface{}
	for rows.Next() {
		var protocol, category string
		var totalBytes, totalPackets, totalFlows int64

		if err := rows.Scan(&protocol, &category, &totalBytes, &totalPackets, &totalFlows); err != nil {
			return nil, err
		}

		protocols = append(protocols, map[string]interface{}{
			"protocol":   protocol,
			"category":   category,
			"bytes":      totalBytes,
			"packets":    totalPackets,
			"flow_count": totalFlows,
		})
	}

	return protocols, nil
}

// QueryTimeseries queries timeseries data within a time range
func (d *Database) QueryTimeseries(metricType string, startTime, endTime time.Time) ([]map[string]interface{}, error) {
	rows, err := d.db.Query(`
		SELECT timestamp, metric_key, value
		FROM timeseries
		WHERE metric_type = ? AND timestamp BETWEEN ? AND ?
		ORDER BY timestamp ASC
	`, metricType, startTime, endTime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var data []map[string]interface{}
	for rows.Next() {
		var timestamp time.Time
		var metricKey string
		var value float64

		if err := rows.Scan(&timestamp, &metricKey, &value); err != nil {
			return nil, err
		}

		data = append(data, map[string]interface{}{
			"timestamp":  timestamp,
			"metric_key": metricKey,
			"value":      value,
		})
	}

	return data, nil
}

// QueryActiveFlows queries active flows
func (d *Database) QueryActiveFlows(limit int) ([]map[string]interface{}, error) {
	rows, err := d.db.Query(`
		SELECT flow_id, src_ip, dst_ip, src_port, dst_port, protocol,
		       l7_protocol, l7_category, bytes_sent, bytes_recv,
		       packets_sent, packets_recv, start_time
		FROM flows
		WHERE is_active = 1
		ORDER BY (bytes_sent + bytes_recv) DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var flows []map[string]interface{}
	for rows.Next() {
		var flowID, srcIP, dstIP, protocol string
		var l7Protocol, l7Category string
		var srcPort, dstPort int
		var bytesSent, bytesRecv, packetsSent, packetsRecv int64
		var startTime time.Time

		if err := rows.Scan(&flowID, &srcIP, &dstIP, &srcPort, &dstPort, &protocol,
			&l7Protocol, &l7Category, &bytesSent, &bytesRecv,
			&packetsSent, &packetsRecv, &startTime); err != nil {
			return nil, err
		}

		flows = append(flows, map[string]interface{}{
			"flow_id":      flowID,
			"src_ip":       srcIP,
			"dst_ip":       dstIP,
			"src_port":     srcPort,
			"dst_port":     dstPort,
			"protocol":     protocol,
			"l7_protocol":  l7Protocol,
			"l7_category":  l7Category,
			"bytes_sent":   bytesSent,
			"bytes_recv":   bytesRecv,
			"packets_sent": packetsSent,
			"packets_recv": packetsRecv,
			"start_time":   startTime,
		})
	}

	return flows, nil
}

// GetFlowByID queries a single flow by its flow_id
func (d *Database) GetFlowByID(flowID string) (*FlowDetail, error) {
	row := d.db.QueryRow(`
		SELECT flow_id, src_ip, dst_ip, src_port, dst_port, protocol,
		       l7_protocol, l7_category, bytes_sent, bytes_recv,
		       packets_sent, packets_recv, start_time, end_time, is_active,
		       rtt_ms, min_rtt_ms, max_rtt_ms, retransmissions,
		       out_of_order, packet_loss, avg_window_size
		FROM flows
		WHERE flow_id = ?
	`, flowID)

	var f FlowDetail
	var endTime *time.Time
	var isActive int

	err := row.Scan(&f.FlowID, &f.SrcIP, &f.DstIP, &f.SrcPort, &f.DstPort, &f.Protocol,
		&f.L7Protocol, &f.L7Category, &f.BytesSent, &f.BytesRecv,
		&f.PacketsSent, &f.PacketsRecv, &f.StartTime, &endTime, &isActive,
		&f.RTT, &f.MinRTT, &f.MaxRTT, &f.Retransmissions,
		&f.OutOfOrder, &f.PacketLoss, &f.AvgWindowSize)
	if err != nil {
		return nil, fmt.Errorf("flow not found: %w", err)
	}

	f.EndTime = endTime
	f.IsActive = isActive == 1

	return &f, nil
}

// SaveFlow inserts or updates a flow record including TCP metrics
func (d *Database) SaveFlow(flowID, srcIP, dstIP string, srcPort, dstPort int, protocol,
	l7Protocol, l7Category string, bytesSent, bytesRecv, packetsSent, packetsRecv int64,
	startTime time.Time, endTime *time.Time, isActive bool,
	rttMs, minRttMs, maxRttMs float64, retransmissions, outOfOrder, packetLoss, avgWindowSize int) error {

	active := 0
	if isActive {
		active = 1
	}

	_, err := d.db.Exec(`
		INSERT INTO flows (flow_id, src_ip, dst_ip, src_port, dst_port, protocol,
			l7_protocol, l7_category, bytes_sent, bytes_recv, packets_sent, packets_recv,
			start_time, end_time, is_active,
			rtt_ms, min_rtt_ms, max_rtt_ms, retransmissions, out_of_order, packet_loss, avg_window_size)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(flow_id) DO UPDATE SET
			bytes_sent = excluded.bytes_sent,
			bytes_recv = excluded.bytes_recv,
			packets_sent = excluded.packets_sent,
			packets_recv = excluded.packets_recv,
			end_time = excluded.end_time,
			is_active = excluded.is_active,
			l7_protocol = excluded.l7_protocol,
			l7_category = excluded.l7_category,
			rtt_ms = excluded.rtt_ms,
			min_rtt_ms = excluded.min_rtt_ms,
			max_rtt_ms = excluded.max_rtt_ms,
			retransmissions = excluded.retransmissions,
			out_of_order = excluded.out_of_order,
			packet_loss = excluded.packet_loss,
			avg_window_size = excluded.avg_window_size
	`, flowID, srcIP, dstIP, srcPort, dstPort, protocol,
		l7Protocol, l7Category, bytesSent, bytesRecv, packetsSent, packetsRecv,
		startTime, endTime, active,
		rttMs, minRttMs, maxRttMs, retransmissions, outOfOrder, packetLoss, avgWindowSize)

	return err
}

// TrafficPair represents traffic between a source and destination IP
type TrafficPair struct {
	SrcIP     string   `json:"src_ip"`
	DstIP     string   `json:"dst_ip"`
	Bytes     uint64   `json:"bytes"`
	FlowCount int      `json:"flow_count"`
	Protocols []string `json:"protocols"`
}

// GetTrafficPairs queries aggregated traffic pairs within a time range.
// It returns pairs sorted by total bytes descending, limited to flows involving the top N hosts.
func (d *Database) GetTrafficPairs(start, end time.Time, limit int) ([]TrafficPair, error) {
	// Step 1: Find the top N hosts by total traffic in the time range
	topHostsQuery := `
		WITH host_bytes AS (
			SELECT src_ip AS ip, SUM(bytes_sent + bytes_recv) AS total
			FROM flows
			WHERE start_time >= ? AND start_time <= ?
			GROUP BY src_ip
			UNION ALL
			SELECT dst_ip AS ip, SUM(bytes_sent + bytes_recv) AS total
			FROM flows
			WHERE start_time >= ? AND start_time <= ?
			GROUP BY dst_ip
		)
		SELECT ip, SUM(total) AS grand_total
		FROM host_bytes
		GROUP BY ip
		ORDER BY grand_total DESC
		LIMIT ?
	`
	rows, err := d.db.Query(topHostsQuery, start, end, start, end, limit)
	if err != nil {
		return nil, fmt.Errorf("query top hosts for matrix: %w", err)
	}
	defer rows.Close()

	topHosts := make(map[string]bool)
	for rows.Next() {
		var ip string
		var total int64
		if err := rows.Scan(&ip, &total); err != nil {
			return nil, fmt.Errorf("scan top host: %w", err)
		}
		topHosts[ip] = true
	}

	if len(topHosts) == 0 {
		return []TrafficPair{}, nil
	}

	// Step 2: Query all flow pairs between top hosts
	pairsQuery := `
		SELECT src_ip, dst_ip,
		       SUM(bytes_sent + bytes_recv) AS total_bytes,
		       COUNT(*) AS flow_count,
		       GROUP_CONCAT(DISTINCT COALESCE(NULLIF(l7_protocol, ''), protocol)) AS protocols
		FROM flows
		WHERE start_time >= ? AND start_time <= ?
		GROUP BY src_ip, dst_ip
		ORDER BY total_bytes DESC
	`
	pairRows, err := d.db.Query(pairsQuery, start, end)
	if err != nil {
		return nil, fmt.Errorf("query traffic pairs: %w", err)
	}
	defer pairRows.Close()

	var pairs []TrafficPair
	for pairRows.Next() {
		var srcIP, dstIP string
		var totalBytes uint64
		var flowCount int
		var protocols string

		if err := pairRows.Scan(&srcIP, &dstIP, &totalBytes, &flowCount, &protocols); err != nil {
			return nil, fmt.Errorf("scan traffic pair: %w", err)
		}

		// Only include pairs where both src and dst are in top hosts
		if !topHosts[srcIP] || !topHosts[dstIP] {
			continue
		}

		var protoList []string
		if protocols != "" {
			for _, p := range strings.Split(protocols, ",") {
				p = strings.TrimSpace(p)
				if p != "" {
					protoList = append(protoList, p)
				}
			}
		}

		pairs = append(pairs, TrafficPair{
			SrcIP:     srcIP,
			DstIP:     dstIP,
			Bytes:     totalBytes,
			FlowCount: flowCount,
			Protocols: protoList,
		})
	}

	if pairs == nil {
		pairs = []TrafficPair{}
	}

	return pairs, nil
}

// HistoricalHost represents a host's traffic stats within a time range
type HistoricalHost struct {
	IP         string `json:"ip"`
	Hostname   string `json:"hostname"`
	BytesSent  int64  `json:"bytes_sent"`
	BytesRecv  int64  `json:"bytes_recv"`
	TotalBytes int64  `json:"total_bytes"`
	FlowCount  int64  `json:"flow_count"`
}

// HistoricalProtocol represents a protocol's traffic stats within a time range
type HistoricalProtocol struct {
	Protocol   string  `json:"protocol"`
	Category   string  `json:"category"`
	TotalBytes int64   `json:"total_bytes"`
	FlowCount  int64   `json:"flow_count"`
	Percentage float64 `json:"percentage"`
}

// HistoricalFlowQuery represents query parameters for historical flow search
type HistoricalFlowQuery struct {
	Start      time.Time
	End        time.Time
	SrcIP      string
	DstIP      string
	Protocol   string
	L7Protocol string
	Limit      int
	Offset     int
}

// FlowRecord represents a flow record returned from historical queries
type FlowRecord struct {
	FlowID      string     `json:"flow_id"`
	SrcIP       string     `json:"src_ip"`
	DstIP       string     `json:"dst_ip"`
	SrcPort     int        `json:"src_port"`
	DstPort     int        `json:"dst_port"`
	Protocol    string     `json:"protocol"`
	L7Protocol  string     `json:"l7_protocol"`
	L7Category  string     `json:"l7_category"`
	BytesSent   int64      `json:"bytes_sent"`
	BytesRecv   int64      `json:"bytes_recv"`
	PacketsSent int64      `json:"packets_sent"`
	PacketsRecv int64      `json:"packets_recv"`
	StartTime   time.Time  `json:"start_time"`
	EndTime     *time.Time `json:"end_time,omitempty"`
	IsActive    bool       `json:"is_active"`
}

// GetHistoricalHosts queries top hosts within a time range from the flows table
func (d *Database) GetHistoricalHosts(start, end time.Time, limit int, sortBy string) ([]HistoricalHost, error) {
	orderClause := "total_bytes DESC"
	switch sortBy {
	case "sent":
		orderClause = "bytes_sent DESC"
	case "recv":
		orderClause = "bytes_recv DESC"
	}

	query := fmt.Sprintf(`
		WITH host_traffic AS (
			SELECT src_ip AS ip, SUM(bytes_sent) AS bytes_sent, SUM(bytes_recv) AS bytes_recv, COUNT(*) AS flow_count
			FROM flows
			WHERE start_time >= ? AND start_time <= ?
			GROUP BY src_ip
			UNION ALL
			SELECT dst_ip AS ip, SUM(bytes_recv) AS bytes_sent, SUM(bytes_sent) AS bytes_recv, COUNT(*) AS flow_count
			FROM flows
			WHERE start_time >= ? AND start_time <= ?
			GROUP BY dst_ip
		)
		SELECT ht.ip,
		       COALESCE(h.hostname, '') AS hostname,
		       SUM(ht.bytes_sent) AS bytes_sent,
		       SUM(ht.bytes_recv) AS bytes_recv,
		       SUM(ht.bytes_sent) + SUM(ht.bytes_recv) AS total_bytes,
		       SUM(ht.flow_count) AS flow_count
		FROM host_traffic ht
		LEFT JOIN hosts h ON h.ip = ht.ip
		GROUP BY ht.ip
		ORDER BY %s
		LIMIT ?
	`, orderClause)

	rows, err := d.db.Query(query, start, end, start, end, limit)
	if err != nil {
		return nil, fmt.Errorf("query historical hosts: %w", err)
	}
	defer rows.Close()

	var hosts []HistoricalHost
	for rows.Next() {
		var h HistoricalHost
		if err := rows.Scan(&h.IP, &h.Hostname, &h.BytesSent, &h.BytesRecv, &h.TotalBytes, &h.FlowCount); err != nil {
			return nil, fmt.Errorf("scan historical host: %w", err)
		}
		hosts = append(hosts, h)
	}
	return hosts, nil
}

// GetHistoricalProtocols queries protocol distribution within a time range from the flows table
func (d *Database) GetHistoricalProtocols(start, end time.Time) ([]HistoricalProtocol, error) {
	rows, err := d.db.Query(`
		SELECT COALESCE(l7_protocol, protocol) AS proto,
		       COALESCE(l7_category, '') AS category,
		       SUM(bytes_sent + bytes_recv) AS total_bytes,
		       COUNT(*) AS flow_count
		FROM flows
		WHERE start_time >= ? AND start_time <= ?
		GROUP BY proto, category
		ORDER BY total_bytes DESC
	`, start, end)
	if err != nil {
		return nil, fmt.Errorf("query historical protocols: %w", err)
	}
	defer rows.Close()

	var protocols []HistoricalProtocol
	var grandTotal int64
	for rows.Next() {
		var p HistoricalProtocol
		if err := rows.Scan(&p.Protocol, &p.Category, &p.TotalBytes, &p.FlowCount); err != nil {
			return nil, fmt.Errorf("scan historical protocol: %w", err)
		}
		grandTotal += p.TotalBytes
		protocols = append(protocols, p)
	}

	// Calculate percentages
	if grandTotal > 0 {
		for i := range protocols {
			protocols[i].Percentage = float64(protocols[i].TotalBytes) / float64(grandTotal) * 100.0
		}
	}

	return protocols, nil
}

// HostWithGeo represents a host with geographic information
type HostWithGeo struct {
	IP        string  `json:"ip"`
	Hostname  string  `json:"hostname"`
	Country   string  `json:"country"`
	City      string  `json:"city"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	ASN       uint    `json:"asn"`
	ASOrg     string  `json:"as_org"`
	BytesSent uint64  `json:"bytes_sent"`
	BytesRecv uint64  `json:"bytes_recv"`
}

// CountryStats represents aggregated traffic by country
type CountryStats struct {
	Country    string `json:"country"`
	HostCount  int    `json:"host_count"`
	TotalBytes uint64 `json:"total_bytes"`
	FlowCount  int    `json:"flow_count"`
}

// ASNStats represents aggregated traffic by ASN
type ASNStats struct {
	ASN        uint   `json:"asn"`
	ASOrg      string `json:"as_org"`
	HostCount  int    `json:"host_count"`
	TotalBytes uint64 `json:"total_bytes"`
}

// GetGeoHosts returns hosts with geographic information
func (d *Database) GetGeoHosts(limit int) ([]HostWithGeo, error) {
	rows, err := d.db.Query(`
		SELECT ip, COALESCE(hostname, '') AS hostname,
		       COALESCE(country, '') AS country, COALESCE(city, '') AS city,
		       COALESCE(latitude, 0) AS latitude, COALESCE(longitude, 0) AS longitude,
		       COALESCE(asn, 0) AS asn, COALESCE(as_org, '') AS as_org,
		       bytes_sent, bytes_recv
		FROM hosts
		WHERE country != '' AND country IS NOT NULL
		ORDER BY (bytes_sent + bytes_recv) DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("query geo hosts: %w", err)
	}
	defer rows.Close()

	var hosts []HostWithGeo
	for rows.Next() {
		var h HostWithGeo
		if err := rows.Scan(&h.IP, &h.Hostname, &h.Country, &h.City,
			&h.Latitude, &h.Longitude, &h.ASN, &h.ASOrg,
			&h.BytesSent, &h.BytesRecv); err != nil {
			return nil, fmt.Errorf("scan geo host: %w", err)
		}
		hosts = append(hosts, h)
	}
	return hosts, nil
}

// GetCountryStats returns aggregated traffic statistics by country
func (d *Database) GetCountryStats() ([]CountryStats, error) {
	rows, err := d.db.Query(`
		SELECT COALESCE(country, '') AS country,
		       COUNT(*) AS host_count,
		       SUM(bytes_sent + bytes_recv) AS total_bytes,
		       0 AS flow_count
		FROM hosts
		WHERE country != '' AND country IS NOT NULL
		GROUP BY country
		ORDER BY total_bytes DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("query country stats: %w", err)
	}
	defer rows.Close()

	var stats []CountryStats
	for rows.Next() {
		var s CountryStats
		if err := rows.Scan(&s.Country, &s.HostCount, &s.TotalBytes, &s.FlowCount); err != nil {
			return nil, fmt.Errorf("scan country stats: %w", err)
		}
		stats = append(stats, s)
	}
	return stats, nil
}

// GetASNStats returns aggregated traffic statistics by ASN
func (d *Database) GetASNStats() ([]ASNStats, error) {
	rows, err := d.db.Query(`
		SELECT COALESCE(asn, 0) AS asn,
		       COALESCE(as_org, '') AS as_org,
		       COUNT(*) AS host_count,
		       SUM(bytes_sent + bytes_recv) AS total_bytes
		FROM hosts
		WHERE asn > 0
		GROUP BY asn, as_org
		ORDER BY total_bytes DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("query ASN stats: %w", err)
	}
	defer rows.Close()

	var stats []ASNStats
	for rows.Next() {
		var s ASNStats
		if err := rows.Scan(&s.ASN, &s.ASOrg, &s.HostCount, &s.TotalBytes); err != nil {
			return nil, fmt.Errorf("scan ASN stats: %w", err)
		}
		stats = append(stats, s)
	}
	return stats, nil
}

// UpsertHostWithGeo inserts or updates a host record including GeoIP fields
func (d *Database) UpsertHostWithGeo(ip, mac, hostname, country, city string,
	lat, lon float64, asn uint, asOrg string,
	bytesSent, bytesRecv, packetsSent, packetsRecv uint64) error {

	_, err := d.db.Exec(`
		INSERT INTO hosts (ip, mac, hostname, bytes_sent, bytes_recv, packets_sent, packets_recv,
			country, city, latitude, longitude, asn, as_org)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(ip) DO UPDATE SET
			mac = CASE WHEN excluded.mac != '' THEN excluded.mac ELSE hosts.mac END,
			hostname = CASE WHEN excluded.hostname != '' THEN excluded.hostname ELSE hosts.hostname END,
			bytes_sent = excluded.bytes_sent,
			bytes_recv = excluded.bytes_recv,
			packets_sent = excluded.packets_sent,
			packets_recv = excluded.packets_recv,
			country = CASE WHEN excluded.country != '' THEN excluded.country ELSE hosts.country END,
			city = CASE WHEN excluded.city != '' THEN excluded.city ELSE hosts.city END,
			latitude = CASE WHEN excluded.latitude != 0 THEN excluded.latitude ELSE hosts.latitude END,
			longitude = CASE WHEN excluded.longitude != 0 THEN excluded.longitude ELSE hosts.longitude END,
			asn = CASE WHEN excluded.asn != 0 THEN excluded.asn ELSE hosts.asn END,
			as_org = CASE WHEN excluded.as_org != '' THEN excluded.as_org ELSE hosts.as_org END,
			last_seen = CURRENT_TIMESTAMP
	`, ip, mac, hostname, bytesSent, bytesRecv, packetsSent, packetsRecv,
		country, city, lat, lon, asn, asOrg)

	return err
}

// GetHistoricalFlows queries historical flow records with multi-dimensional filtering and pagination
func (d *Database) GetHistoricalFlows(params HistoricalFlowQuery) ([]FlowRecord, int, error) {
	whereClauses := []string{"start_time >= ?", "start_time <= ?"}
	args := []interface{}{params.Start, params.End}

	if params.SrcIP != "" {
		whereClauses = append(whereClauses, "src_ip = ?")
		args = append(args, params.SrcIP)
	}
	if params.DstIP != "" {
		whereClauses = append(whereClauses, "dst_ip = ?")
		args = append(args, params.DstIP)
	}
	if params.Protocol != "" {
		whereClauses = append(whereClauses, "protocol = ?")
		args = append(args, params.Protocol)
	}
	if params.L7Protocol != "" {
		whereClauses = append(whereClauses, "l7_protocol = ?")
		args = append(args, params.L7Protocol)
	}

	whereStr := strings.Join(whereClauses, " AND ")

	// Count total matching records
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM flows WHERE %s", whereStr)
	var total int
	if err := d.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count historical flows: %w", err)
	}

	// Fetch paginated records
	dataQuery := fmt.Sprintf(`
		SELECT flow_id, src_ip, dst_ip, src_port, dst_port, protocol,
		       l7_protocol, l7_category, bytes_sent, bytes_recv,
		       packets_sent, packets_recv, start_time, end_time, is_active
		FROM flows
		WHERE %s
		ORDER BY start_time DESC
		LIMIT ? OFFSET ?
	`, whereStr)
	dataArgs := append(args, params.Limit, params.Offset)

	rows, err := d.db.Query(dataQuery, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query historical flows: %w", err)
	}
	defer rows.Close()

	var flows []FlowRecord
	for rows.Next() {
		var f FlowRecord
		var endTime *time.Time
		var isActive int
		if err := rows.Scan(&f.FlowID, &f.SrcIP, &f.DstIP, &f.SrcPort, &f.DstPort, &f.Protocol,
			&f.L7Protocol, &f.L7Category, &f.BytesSent, &f.BytesRecv,
			&f.PacketsSent, &f.PacketsRecv, &f.StartTime, &endTime, &isActive); err != nil {
			return nil, 0, fmt.Errorf("scan historical flow: %w", err)
		}
		f.EndTime = endTime
		f.IsActive = isActive == 1
		flows = append(flows, f)
	}

	return flows, total, nil
}
