package main

import (
	"net/http"
	"strconv"
	"time"

	"github.com/netmonitor/backend/storage"
)

// parseTimeParam parses a time parameter that can be either a Unix timestamp (seconds) or RFC3339 format.
// Returns zero time and false if the parameter is empty or unparseable.
func parseTimeParam(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}

	// Try Unix timestamp (integer seconds)
	if ts, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.Unix(ts, 0), true
	}

	// Try RFC3339
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, true
	}

	return time.Time{}, false
}

// granularityToTable maps user-specified granularity strings to table names.
func granularityToTable(g string) string {
	switch g {
	case "5min":
		return "timeseries_5min"
	case "1h":
		return "timeseries_1h"
	case "1d":
		return "timeseries_1d"
	default:
		return ""
	}
}

// GET /api/v1/historical/traffic
func (s *Server) handleHistoricalTraffic(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	start, ok := parseTimeParam(q.Get("start"))
	if !ok {
		writeError(w, "missing or invalid 'start' parameter", http.StatusBadRequest)
		return
	}
	end, ok := parseTimeParam(q.Get("end"))
	if !ok {
		writeError(w, "missing or invalid 'end' parameter", http.StatusBadRequest)
		return
	}

	granularity := q.Get("granularity")
	if granularity == "" {
		granularity = "auto"
	}

	var points []storage.AggregatedPoint
	var err error

	if granularity == "auto" {
		// Use AggregationManager's automatic selection
		points, err = s.aggManager.QueryAggregated("bandwidth", "", start, end)
	} else {
		table := granularityToTable(granularity)
		if table == "" {
			writeError(w, "invalid granularity: must be auto, 5min, 1h, or 1d", http.StatusBadRequest)
			return
		}
		// Query the specific table directly via QueryAggregated with a fixed duration hint
		// We use the aggManager's internal methods through QueryAggregated by adjusting selection
		points, err = s.aggManager.QueryAggregatedFromTable(table, "bandwidth", "", start, end)
	}

	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if points == nil {
		points = []storage.AggregatedPoint{}
	}

	writeJSON(w, map[string]interface{}{
		"granularity": granularity,
		"start":       start,
		"end":         end,
		"data":        points,
	})
}

// GET /api/v1/historical/hosts
func (s *Server) handleHistoricalHosts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	start, ok := parseTimeParam(q.Get("start"))
	if !ok {
		writeError(w, "missing or invalid 'start' parameter", http.StatusBadRequest)
		return
	}
	end, ok := parseTimeParam(q.Get("end"))
	if !ok {
		writeError(w, "missing or invalid 'end' parameter", http.StatusBadRequest)
		return
	}

	top := 20
	if v := q.Get("top"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			top = n
		}
	}

	sortBy := q.Get("sort")
	if sortBy == "" {
		sortBy = "total"
	}

	hosts, err := s.db.GetHistoricalHosts(start, end, top, sortBy)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if hosts == nil {
		hosts = []storage.HistoricalHost{}
	}

	writeJSON(w, map[string]interface{}{
		"start": start,
		"end":   end,
		"hosts": hosts,
	})
}

// GET /api/v1/historical/protocols
func (s *Server) handleHistoricalProtocols(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	start, ok := parseTimeParam(q.Get("start"))
	if !ok {
		writeError(w, "missing or invalid 'start' parameter", http.StatusBadRequest)
		return
	}
	end, ok := parseTimeParam(q.Get("end"))
	if !ok {
		writeError(w, "missing or invalid 'end' parameter", http.StatusBadRequest)
		return
	}

	protocols, err := s.db.GetHistoricalProtocols(start, end)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if protocols == nil {
		protocols = []storage.HistoricalProtocol{}
	}

	writeJSON(w, map[string]interface{}{
		"start":     start,
		"end":       end,
		"protocols": protocols,
	})
}

// GET /api/v1/historical/compare
func (s *Server) handleHistoricalCompare(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	p1Start, ok := parseTimeParam(q.Get("period1_start"))
	if !ok {
		writeError(w, "missing or invalid 'period1_start' parameter", http.StatusBadRequest)
		return
	}
	p1End, ok := parseTimeParam(q.Get("period1_end"))
	if !ok {
		writeError(w, "missing or invalid 'period1_end' parameter", http.StatusBadRequest)
		return
	}
	p2Start, ok := parseTimeParam(q.Get("period2_start"))
	if !ok {
		writeError(w, "missing or invalid 'period2_start' parameter", http.StatusBadRequest)
		return
	}
	p2End, ok := parseTimeParam(q.Get("period2_end"))
	if !ok {
		writeError(w, "missing or invalid 'period2_end' parameter", http.StatusBadRequest)
		return
	}

	metric := q.Get("metric")
	if metric == "" {
		metric = "bandwidth"
	}

	period1Data, err := s.aggManager.QueryAggregated(metric, "", p1Start, p1End)
	if err != nil {
		writeError(w, "period1 query error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if period1Data == nil {
		period1Data = []storage.AggregatedPoint{}
	}

	period2Data, err := s.aggManager.QueryAggregated(metric, "", p2Start, p2End)
	if err != nil {
		writeError(w, "period2 query error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if period2Data == nil {
		period2Data = []storage.AggregatedPoint{}
	}

	writeJSON(w, map[string]interface{}{
		"metric": metric,
		"period1": map[string]interface{}{
			"start": p1Start,
			"end":   p1End,
			"data":  period1Data,
		},
		"period2": map[string]interface{}{
			"start": p2Start,
			"end":   p2End,
			"data":  period2Data,
		},
	})
}

// GET /api/v1/flows/historical
func (s *Server) handleHistoricalFlows(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	start, ok := parseTimeParam(q.Get("start"))
	if !ok {
		writeError(w, "missing or invalid 'start' parameter", http.StatusBadRequest)
		return
	}
	end, ok := parseTimeParam(q.Get("end"))
	if !ok {
		writeError(w, "missing or invalid 'end' parameter", http.StatusBadRequest)
		return
	}

	limit := 100
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 1000 {
		limit = 1000
	}

	offset := 0
	if v := q.Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	params := storage.HistoricalFlowQuery{
		Start:      start,
		End:        end,
		SrcIP:      q.Get("src_ip"),
		DstIP:      q.Get("dst_ip"),
		Protocol:   q.Get("protocol"),
		L7Protocol: q.Get("l7_protocol"),
		Limit:      limit,
		Offset:     offset,
	}

	flows, total, err := s.db.GetHistoricalFlows(params)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if flows == nil {
		flows = []storage.FlowRecord{}
	}

	writeJSON(w, map[string]interface{}{
		"flows":  flows,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}
