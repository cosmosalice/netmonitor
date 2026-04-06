package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/netmonitor/backend/alerts"
)

// ---------------------------------------------------------------------------
// Alert Handlers
// ---------------------------------------------------------------------------

// GET /api/v1/alerts
func (s *Server) getAlerts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	filter := alerts.AlertFilter{}

	if v := q.Get("type"); v != "" {
		filter.Type = alerts.AlertType(v)
	}
	if v := q.Get("severity"); v != "" {
		filter.Severity = alerts.AlertSeverity(v)
	}
	if v := q.Get("status"); v != "" {
		filter.Status = alerts.AlertStatus(v)
	}
	if v := q.Get("entity_id"); v != "" {
		filter.EntityID = v
	}
	if v := q.Get("start"); v != "" {
		if ts, err := strconv.ParseInt(v, 10, 64); err == nil {
			t := time.Unix(ts, 0)
			filter.Start = &t
		}
	}
	if v := q.Get("end"); v != "" {
		if ts, err := strconv.ParseInt(v, 10, 64); err == nil {
			t := time.Unix(ts, 0)
			filter.End = &t
		}
	}
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filter.Limit = n
		}
	}
	if v := q.Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filter.Offset = n
		}
	}

	alertList, total, err := s.alertEngine.GetAlerts(filter)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if alertList == nil {
		alertList = []alerts.Alert{}
	}
	writeJSON(w, map[string]interface{}{
		"alerts": alertList,
		"total":  total,
	})
}

// POST /api/v1/alerts/{id}/acknowledge
func (s *Server) acknowledgeAlert(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		writeError(w, "invalid alert id", http.StatusBadRequest)
		return
	}

	if err := s.alertEngine.AcknowledgeAlert(id); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]interface{}{"success": true})
}

// POST /api/v1/alerts/{id}/resolve
func (s *Server) resolveAlert(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		writeError(w, "invalid alert id", http.StatusBadRequest)
		return
	}

	if err := s.alertEngine.ResolveAlert(id); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]interface{}{"success": true})
}

// GET /api/v1/alerts/stats
func (s *Server) getAlertStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.alertEngine.GetAlertStats()
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, stats)
}

// ---------------------------------------------------------------------------
// Rule Handlers
// ---------------------------------------------------------------------------

// GET /api/v1/alerts/rules
func (s *Server) getAlertRules(w http.ResponseWriter, r *http.Request) {
	rm := s.alertEngine.GetRuleManager()
	if rm == nil {
		writeJSON(w, map[string]interface{}{"rules": []interface{}{}})
		return
	}
	rules := rm.GetRules()
	if rules == nil {
		rules = []*alerts.AlertRule{}
	}
	writeJSON(w, map[string]interface{}{"rules": rules})
}

// POST /api/v1/alerts/rules
func (s *Server) saveAlertRule(w http.ResponseWriter, r *http.Request) {
	rm := s.alertEngine.GetRuleManager()
	if rm == nil {
		writeError(w, "rule manager not initialized", http.StatusInternalServerError)
		return
	}

	var rule alerts.AlertRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if rule.ID == "" {
		writeError(w, "rule id is required", http.StatusBadRequest)
		return
	}

	if err := rm.SaveRule(&rule); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]interface{}{"success": true, "rule": rule})
}

// DELETE /api/v1/alerts/rules/{id}
func (s *Server) deleteAlertRule(w http.ResponseWriter, r *http.Request) {
	rm := s.alertEngine.GetRuleManager()
	if rm == nil {
		writeError(w, "rule manager not initialized", http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		writeError(w, "rule id is required", http.StatusBadRequest)
		return
	}

	if err := rm.DeleteRule(id); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]interface{}{"success": true})
}

// ---------------------------------------------------------------------------
// Notification Endpoint Handlers
// ---------------------------------------------------------------------------

// GET /api/v1/alerts/notification-endpoints
func (s *Server) getNotificationEndpoints(w http.ResponseWriter, r *http.Request) {
	nm := s.alertEngine.GetNotificationManager()
	if nm == nil {
		writeJSON(w, map[string]interface{}{"endpoints": []interface{}{}})
		return
	}
	endpoints := nm.GetEndpoints()
	if endpoints == nil {
		endpoints = []alerts.NotificationEndpoint{}
	}
	writeJSON(w, map[string]interface{}{"endpoints": endpoints})
}

// POST /api/v1/alerts/notification-endpoints
func (s *Server) saveNotificationEndpoint(w http.ResponseWriter, r *http.Request) {
	nm := s.alertEngine.GetNotificationManager()
	if nm == nil {
		writeError(w, "notification manager not initialized", http.StatusInternalServerError)
		return
	}

	var ep alerts.NotificationEndpoint
	if err := json.NewDecoder(r.Body).Decode(&ep); err != nil {
		writeError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if ep.ID == "" {
		writeError(w, "endpoint id is required", http.StatusBadRequest)
		return
	}

	if err := nm.SaveEndpoint(ep); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]interface{}{"success": true, "endpoint": ep})
}

// DELETE /api/v1/alerts/notification-endpoints/{id}
func (s *Server) deleteNotificationEndpoint(w http.ResponseWriter, r *http.Request) {
	nm := s.alertEngine.GetNotificationManager()
	if nm == nil {
		writeError(w, "notification manager not initialized", http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		writeError(w, "endpoint id is required", http.StatusBadRequest)
		return
	}

	if err := nm.DeleteEndpoint(id); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]interface{}{"success": true})
}
