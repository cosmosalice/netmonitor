package main

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/netmonitor/backend/reports"
)

// GET /api/v1/reports
func (s *Server) getReports(w http.ResponseWriter, r *http.Request) {
	rpts, err := s.reportGen.GetReports()
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]interface{}{"reports": rpts})
}

// GET /api/v1/reports/{id}/download
func (s *Server) downloadReport(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		writeError(w, "report id is required", http.StatusBadRequest)
		return
	}

	report, err := s.reportGen.GetReportByID(id)
	if err != nil {
		writeError(w, "report not found", http.StatusNotFound)
		return
	}

	content, err := os.ReadFile(report.FilePath)
	if err != nil {
		writeError(w, "report file not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Disposition", "inline; filename=\""+report.Name+".html\"")
	w.Write(content)
}

// POST /api/v1/reports/generate
func (s *Server) generateReport(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type string `json:"type"`
		Date string `json:"date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Type == "" {
		writeError(w, "type is required", http.StatusBadRequest)
		return
	}

	var date time.Time
	var err error

	if req.Date != "" {
		date, err = time.ParseInLocation("2006-01-02", req.Date, time.Local)
		if err != nil {
			writeError(w, "invalid date format, use YYYY-MM-DD", http.StatusBadRequest)
			return
		}
	} else {
		date = time.Now().AddDate(0, 0, -1)
	}

	var report interface{}
	switch req.Type {
	case "daily":
		report, err = s.reportGen.GenerateDaily(date)
	case "weekly":
		report, err = s.reportGen.GenerateWeekly(date)
	case "monthly":
		report, err = s.reportGen.GenerateMonthly(date)
	default:
		writeError(w, "invalid type: must be daily, weekly, or monthly", http.StatusBadRequest)
		return
	}

	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]interface{}{"report": report})
}

// GET /api/v1/reports/configs
func (s *Server) getReportConfigs(w http.ResponseWriter, r *http.Request) {
	configs, err := s.reportGen.GetReportConfigs()
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]interface{}{"configs": configs})
}

// POST /api/v1/reports/configs
func (s *Server) saveReportConfig(w http.ResponseWriter, r *http.Request) {
	var cfg reports.ReportConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if cfg.ID == "" || cfg.Type == "" {
		writeError(w, "id and type are required", http.StatusBadRequest)
		return
	}

	if err := s.reportGen.SaveReportConfig(cfg); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"status": "ok"})
}
