package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/netmonitor/backend/storage"
)

// ---------------------------------------------------------------------------
// Export Flows: GET /api/v1/export/flows
// ---------------------------------------------------------------------------

func (s *Server) exportFlows(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	format := q.Get("format")
	if format == "" {
		format = "csv"
	}
	if format != "csv" && format != "json" {
		writeError(w, "format must be csv or json", http.StatusBadRequest)
		return
	}

	// Time range defaults: last 24 hours
	endTime := time.Now()
	startTime := endTime.Add(-24 * time.Hour)

	if st := q.Get("start"); st != "" {
		if t, ok := parseTimeParam(st); ok {
			startTime = t
		}
	}
	if et := q.Get("end"); et != "" {
		if t, ok := parseTimeParam(et); ok {
			endTime = t
		}
	}

	limit := 10000
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 100000 {
		limit = 100000
	}

	params := storage.HistoricalFlowQuery{
		Start:      startTime,
		End:        endTime,
		SrcIP:      q.Get("src_ip"),
		DstIP:      q.Get("dst_ip"),
		Protocol:   q.Get("protocol"),
		L7Protocol: q.Get("l7_protocol"),
		Limit:      limit,
		Offset:     0,
	}

	flows, _, err := s.db.GetHistoricalFlows(params)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if format == "json" {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", `attachment; filename="flows_export.json"`)
		json.NewEncoder(w).Encode(flows)
		return
	}

	// CSV format with streaming
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachment; filename="flows_export.csv"`)

	cw := csv.NewWriter(w)
	flusher, hasFlusher := w.(http.Flusher)

	// Header row
	cw.Write([]string{
		"flow_id", "src_ip", "dst_ip", "src_port", "dst_port",
		"protocol", "l7_protocol", "bytes_sent", "bytes_recv",
		"packets_sent", "packets_recv", "start_time", "end_time",
	})

	for i, f := range flows {
		endTimeStr := ""
		if f.EndTime != nil {
			endTimeStr = f.EndTime.Format(time.RFC3339)
		}
		cw.Write([]string{
			f.FlowID,
			f.SrcIP,
			f.DstIP,
			strconv.Itoa(f.SrcPort),
			strconv.Itoa(f.DstPort),
			f.Protocol,
			f.L7Protocol,
			strconv.FormatInt(f.BytesSent, 10),
			strconv.FormatInt(f.BytesRecv, 10),
			strconv.FormatInt(f.PacketsSent, 10),
			strconv.FormatInt(f.PacketsRecv, 10),
			f.StartTime.Format(time.RFC3339),
			endTimeStr,
		})
		if i%500 == 0 {
			cw.Flush()
			if hasFlusher {
				flusher.Flush()
			}
		}
	}
	cw.Flush()
}

// ---------------------------------------------------------------------------
// Export Hosts: GET /api/v1/export/hosts
// ---------------------------------------------------------------------------

func (s *Server) exportHosts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	format := q.Get("format")
	if format == "" {
		format = "csv"
	}
	if format != "csv" && format != "json" {
		writeError(w, "format must be csv or json", http.StatusBadRequest)
		return
	}

	limit := 1000
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}

	sortBy := q.Get("sort")
	if sortBy == "" {
		sortBy = "total"
	}

	hosts, err := s.db.QueryHosts(limit)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if format == "json" {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", `attachment; filename="hosts_export.json"`)
		json.NewEncoder(w).Encode(hosts)
		return
	}

	// CSV format with streaming
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachment; filename="hosts_export.csv"`)

	cw := csv.NewWriter(w)
	flusher, hasFlusher := w.(http.Flusher)

	cw.Write([]string{
		"ip", "hostname", "mac", "bytes_sent", "bytes_recv",
		"total_bytes", "packets_sent", "packets_recv", "first_seen", "last_seen",
	})

	for i, h := range hosts {
		ip := fmt.Sprintf("%v", h["ip"])
		hostname := fmt.Sprintf("%v", h["hostname"])
		mac := fmt.Sprintf("%v", h["mac"])
		bytesSent := fmt.Sprintf("%v", h["bytes_sent"])
		bytesRecv := fmt.Sprintf("%v", h["bytes_recv"])

		var bSent, bRecv int64
		if v, ok := h["bytes_sent"].(int64); ok {
			bSent = v
		}
		if v, ok := h["bytes_recv"].(int64); ok {
			bRecv = v
		}
		totalBytes := strconv.FormatInt(bSent+bRecv, 10)

		packetsSent := fmt.Sprintf("%v", h["packets_sent"])
		packetsRecv := fmt.Sprintf("%v", h["packets_recv"])

		firstSeen := ""
		if t, ok := h["first_seen"].(time.Time); ok {
			firstSeen = t.Format(time.RFC3339)
		}
		lastSeen := ""
		if t, ok := h["last_seen"].(time.Time); ok {
			lastSeen = t.Format(time.RFC3339)
		}

		cw.Write([]string{
			ip, hostname, mac, bytesSent, bytesRecv,
			totalBytes, packetsSent, packetsRecv, firstSeen, lastSeen,
		})
		if i%500 == 0 {
			cw.Flush()
			if hasFlusher {
				flusher.Flush()
			}
		}
	}
	cw.Flush()
}

// ---------------------------------------------------------------------------
// Export Timeseries: GET /api/v1/export/timeseries
// ---------------------------------------------------------------------------

func (s *Server) exportTimeseries(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	format := q.Get("format")
	if format == "" {
		format = "csv"
	}
	if format != "csv" && format != "json" {
		writeError(w, "format must be csv or json", http.StatusBadRequest)
		return
	}

	metricType := q.Get("type")
	if metricType == "" {
		metricType = "bandwidth"
	}

	// Time range defaults: last 1 hour
	endTime := time.Now()
	startTime := endTime.Add(-1 * time.Hour)

	if st := q.Get("start"); st != "" {
		if t, ok := parseTimeParam(st); ok {
			startTime = t
		}
	}
	if et := q.Get("end"); et != "" {
		if t, ok := parseTimeParam(et); ok {
			endTime = t
		}
	}

	// Select granularity automatically
	duration := endTime.Sub(startTime)
	table := s.aggManager.SelectGranularity(duration)
	useAggregated := table != "timeseries"

	if format == "json" {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", `attachment; filename="timeseries_export.json"`)

		if useAggregated {
			points, err := s.aggManager.QueryAggregatedFromTable(table, metricType, "", startTime, endTime)
			if err != nil {
				writeError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(points)
		} else {
			data, err := s.db.QueryTimeseries(metricType, startTime, endTime)
			if err != nil {
				writeError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(data)
		}
		return
	}

	// CSV format with streaming
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachment; filename="timeseries_export.csv"`)

	cw := csv.NewWriter(w)
	flusher, hasFlusher := w.(http.Flusher)

	if useAggregated {
		cw.Write([]string{"timestamp", "metric_type", "metric_key", "avg_value", "max_value", "min_value"})

		points, err := s.aggManager.QueryAggregatedFromTable(table, metricType, "", startTime, endTime)
		if err != nil {
			// Headers already sent, write error as CSV comment
			cw.Write([]string{"# error: " + err.Error()})
			cw.Flush()
			return
		}

		for i, p := range points {
			cw.Write([]string{
				p.Timestamp.Format(time.RFC3339),
				p.MetricType,
				p.MetricKey,
				strconv.FormatFloat(p.AvgValue, 'f', 4, 64),
				strconv.FormatFloat(p.MaxValue, 'f', 4, 64),
				strconv.FormatFloat(p.MinValue, 'f', 4, 64),
			})
			if i%500 == 0 {
				cw.Flush()
				if hasFlusher {
					flusher.Flush()
				}
			}
		}
	} else {
		cw.Write([]string{"timestamp", "metric_type", "metric_key", "value"})

		data, err := s.db.QueryTimeseries(metricType, startTime, endTime)
		if err != nil {
			cw.Write([]string{"# error: " + err.Error()})
			cw.Flush()
			return
		}

		for i, d := range data {
			ts := ""
			if t, ok := d["timestamp"].(time.Time); ok {
				ts = t.Format(time.RFC3339)
			}
			mk := fmt.Sprintf("%v", d["metric_key"])
			val := fmt.Sprintf("%v", d["value"])
			cw.Write([]string{ts, metricType, mk, val})
			if i%500 == 0 {
				cw.Flush()
				if hasFlusher {
					flusher.Flush()
				}
			}
		}
	}
	cw.Flush()
}
