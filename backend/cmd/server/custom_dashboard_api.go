package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

// WidgetConfig Widget 配置
type WidgetConfig struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"` // chart/table/stat/map
	Title      string          `json:"title"`
	DataSource string          `json:"data_source"` // API endpoint or data source
	Position   WidgetPosition  `json:"position"`
	Config     json.RawMessage `json:"config"` // Widget-specific config
}

// WidgetPosition Widget 位置
type WidgetPosition struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

// DashboardConfig Dashboard 配置
type DashboardConfig struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Widgets   []WidgetConfig `json:"widgets"`
	IsDefault bool           `json:"is_default"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// DashboardManager Dashboard 管理器
type DashboardManager struct {
	db *sql.DB
}

// NewDashboardManager 创建 Dashboard 管理器
func NewDashboardManager(db *sql.DB) *DashboardManager {
	return &DashboardManager{db: db}
}

// Init 初始化默认 Dashboard
func (dm *DashboardManager) Init() error {
	// 检查是否已有默认 dashboard
	var count int
	err := dm.db.QueryRow("SELECT COUNT(*) FROM dashboards").Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		// 创建默认 Dashboard
		if err := dm.createDefaultDashboards(); err != nil {
			return err
		}
	}

	return nil
}

// createDefaultDashboards 创建默认 Dashboard 配置
func (dm *DashboardManager) createDefaultDashboards() error {
	// 概览 Dashboard
	overview := DashboardConfig{
		ID:        "overview",
		Name:      "概览",
		IsDefault: true,
		Widgets: []WidgetConfig{
			{ID: "w1", Type: "stat", Title: "总流量", DataSource: "/api/v1/stats/summary", Position: WidgetPosition{X: 0, Y: 0, W: 3, H: 2}, Config: json.RawMessage(`{"metric":"total_bytes"}`)},
			{ID: "w2", Type: "stat", Title: "活跃主机", DataSource: "/api/v1/stats/summary", Position: WidgetPosition{X: 3, Y: 0, W: 3, H: 2}, Config: json.RawMessage(`{"metric":"active_hosts"}`)},
			{ID: "w3", Type: "stat", Title: "活跃流", DataSource: "/api/v1/stats/summary", Position: WidgetPosition{X: 6, Y: 0, W: 3, H: 2}, Config: json.RawMessage(`{"metric":"active_flows"}`)},
			{ID: "w4", Type: "stat", Title: "告警数", DataSource: "/api/v1/alerts/stats", Position: WidgetPosition{X: 9, Y: 0, W: 3, H: 2}, Config: json.RawMessage(`{"metric":"total"}`)},
			{ID: "w5", Type: "line", Title: "流量趋势", DataSource: "/api/v1/timeseries", Position: WidgetPosition{X: 0, Y: 2, W: 6, H: 4}, Config: json.RawMessage(`{"type":"bandwidth"}`)},
			{ID: "w6", Type: "pie", Title: "协议分布", DataSource: "/api/v1/stats/protocols", Position: WidgetPosition{X: 6, Y: 2, W: 6, H: 4}, Config: json.RawMessage(`{"limit":10}`)},
			{ID: "w7", Type: "bar", Title: "Top Hosts", DataSource: "/api/v1/stats/hosts", Position: WidgetPosition{X: 0, Y: 6, W: 6, H: 4}, Config: json.RawMessage(`{"limit":10}`)},
			{ID: "w8", Type: "table", Title: "最近告警", DataSource: "/api/v1/alerts", Position: WidgetPosition{X: 6, Y: 6, W: 6, H: 4}, Config: json.RawMessage(`{"limit":10}`)},
		},
	}

	if err := dm.SaveDashboard(&overview); err != nil {
		return err
	}

	// 安全 Dashboard
	security := DashboardConfig{
		ID:   "security",
		Name: "安全",
		Widgets: []WidgetConfig{
			{ID: "w1", Type: "stat", Title: "高危告警", DataSource: "/api/v1/alerts/stats", Position: WidgetPosition{X: 0, Y: 0, W: 4, H: 2}, Config: json.RawMessage(`{"severity":"critical"}`)},
			{ID: "w2", Type: "stat", Title: "黑名单命中", DataSource: "/api/v1/alerts/stats", Position: WidgetPosition{X: 4, Y: 0, W: 4, H: 2}, Config: json.RawMessage(`{"type":"blacklist"}`)},
			{ID: "w3", Type: "stat", Title: "风险主机", DataSource: "/api/v1/hosts/risks", Position: WidgetPosition{X: 8, Y: 0, W: 4, H: 2}, Config: json.RawMessage(`{"min_score":70}`)},
			{ID: "w4", Type: "table", Title: "未确认告警", DataSource: "/api/v1/alerts", Position: WidgetPosition{X: 0, Y: 2, W: 6, H: 6}, Config: json.RawMessage(`{"status":"triggered","limit":20}`)},
			{ID: "w5", Type: "bar", Title: "风险主机排行", DataSource: "/api/v1/hosts/risks", Position: WidgetPosition{X: 6, Y: 2, W: 6, H: 6}, Config: json.RawMessage(`{"limit":10}`)},
		},
	}

	if err := dm.SaveDashboard(&security); err != nil {
		return err
	}

	// 性能 Dashboard
	performance := DashboardConfig{
		ID:   "performance",
		Name: "性能",
		Widgets: []WidgetConfig{
			{ID: "w1", Type: "stat", Title: "PPS", DataSource: "/api/v1/stats/summary", Position: WidgetPosition{X: 0, Y: 0, W: 3, H: 2}, Config: json.RawMessage(`{"metric":"packets_per_sec"}`)},
			{ID: "w2", Type: "stat", Title: "BPS", DataSource: "/api/v1/stats/summary", Position: WidgetPosition{X: 3, Y: 0, W: 3, H: 2}, Config: json.RawMessage(`{"metric":"bytes_per_sec"}`)},
			{ID: "w3", Type: "stat", Title: "活跃协议", DataSource: "/api/v1/stats/summary", Position: WidgetPosition{X: 6, Y: 0, W: 3, H: 2}, Config: json.RawMessage(`{"metric":"active_protocols"}`)},
			{ID: "w4", Type: "stat", Title: "TCP重传", DataSource: "/api/v1/stats/summary", Position: WidgetPosition{X: 9, Y: 0, W: 3, H: 2}, Config: json.RawMessage(`{"metric":"tcp_retransmissions"}`)},
			{ID: "w5", Type: "line", Title: "带宽趋势", DataSource: "/api/v1/timeseries", Position: WidgetPosition{X: 0, Y: 2, W: 12, H: 4}, Config: json.RawMessage(`{"type":"bandwidth"}`)},
			{ID: "w6", Type: "line", Title: "协议趋势", DataSource: "/api/v1/timeseries", Position: WidgetPosition{X: 0, Y: 6, W: 6, H: 4}, Config: json.RawMessage(`{"type":"protocols"}`)},
			{ID: "w7", Type: "table", Title: "Top Talkers", DataSource: "/api/v1/stats/hosts", Position: WidgetPosition{X: 6, Y: 6, W: 6, H: 4}, Config: json.RawMessage(`{"limit":10,"sort":"total"}`)},
		},
	}

	return dm.SaveDashboard(&performance)
}

// GetDashboards 获取所有 Dashboard 列表
func (dm *DashboardManager) GetDashboards() ([]DashboardConfig, error) {
	rows, err := dm.db.Query(
		"SELECT id, name, config, is_default, created_at, updated_at FROM dashboards ORDER BY created_at",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dashboards []DashboardConfig
	for rows.Next() {
		var d DashboardConfig
		var configJSON string
		err := rows.Scan(&d.ID, &d.Name, &configJSON, &d.IsDefault, &d.CreatedAt, &d.UpdatedAt)
		if err != nil {
			continue
		}

		var fullConfig struct {
			Widgets []WidgetConfig `json:"widgets"`
		}
		if err := json.Unmarshal([]byte(configJSON), &fullConfig); err == nil {
			d.Widgets = fullConfig.Widgets
		}

		dashboards = append(dashboards, d)
	}

	return dashboards, nil
}

// GetDashboard 获取单个 Dashboard
func (dm *DashboardManager) GetDashboard(id string) (*DashboardConfig, error) {
	var d DashboardConfig
	var configJSON string

	err := dm.db.QueryRow(
		"SELECT id, name, config, is_default, created_at, updated_at FROM dashboards WHERE id = ?",
		id,
	).Scan(&d.ID, &d.Name, &configJSON, &d.IsDefault, &d.CreatedAt, &d.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("dashboard not found")
	}
	if err != nil {
		return nil, err
	}

	var fullConfig struct {
		Widgets []WidgetConfig `json:"widgets"`
	}
	if err := json.Unmarshal([]byte(configJSON), &fullConfig); err == nil {
		d.Widgets = fullConfig.Widgets
	}

	return &d, nil
}

// SaveDashboard 保存 Dashboard
func (dm *DashboardManager) SaveDashboard(d *DashboardConfig) error {
	configData := map[string]interface{}{
		"widgets": d.Widgets,
	}
	configJSON, err := json.Marshal(configData)
	if err != nil {
		return err
	}

	_, err = dm.db.Exec(
		`INSERT INTO dashboards (id, name, config, is_default, created_at, updated_at)
		 VALUES (?, ?, ?, ?, datetime('now'), datetime('now'))
		 ON CONFLICT(id) DO UPDATE SET
		 name = excluded.name,
		 config = excluded.config,
		 is_default = excluded.is_default,
		 updated_at = datetime('now')`,
		d.ID, d.Name, string(configJSON), boolToInt(d.IsDefault),
	)
	return err
}

// DeleteDashboard 删除 Dashboard
func (dm *DashboardManager) DeleteDashboard(id string) error {
	_, err := dm.db.Exec("DELETE FROM dashboards WHERE id = ?", id)
	return err
}

// ---------------------------------------------------------------------------
// HTTP Handlers
// ---------------------------------------------------------------------------

// GET /api/v1/dashboards - Dashboard 列表
func (s *Server) getDashboards(w http.ResponseWriter, r *http.Request) {
	if s.dashboardMgr == nil {
		writeJSON(w, map[string]interface{}{"dashboards": []interface{}{}})
		return
	}

	dashboards, err := s.dashboardMgr.GetDashboards()
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]interface{}{"dashboards": dashboards})
}

// POST /api/v1/dashboards - 创建 Dashboard
func (s *Server) createDashboard(w http.ResponseWriter, r *http.Request) {
	if s.dashboardMgr == nil {
		writeError(w, "dashboard manager not initialized", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		ID      string         `json:"id"`
		Name    string         `json:"name"`
		Widgets []WidgetConfig `json:"widgets"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.ID == "" || req.Name == "" {
		writeError(w, "id and name are required", http.StatusBadRequest)
		return
	}

	dashboard := &DashboardConfig{
		ID:        req.ID,
		Name:      req.Name,
		Widgets:   req.Widgets,
		IsDefault: false,
	}

	if err := s.dashboardMgr.SaveDashboard(dashboard); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, dashboard)
}

// GET /api/v1/dashboards/{id} - 获取 Dashboard 配置
func (s *Server) getDashboard(w http.ResponseWriter, r *http.Request) {
	if s.dashboardMgr == nil {
		writeError(w, "dashboard manager not initialized", http.StatusServiceUnavailable)
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	dashboard, err := s.dashboardMgr.GetDashboard(id)
	if err != nil {
		writeError(w, err.Error(), http.StatusNotFound)
		return
	}

	writeJSON(w, dashboard)
}

// PUT /api/v1/dashboards/{id} - 更新 Dashboard
func (s *Server) updateDashboard(w http.ResponseWriter, r *http.Request) {
	if s.dashboardMgr == nil {
		writeError(w, "dashboard manager not initialized", http.StatusServiceUnavailable)
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	// 检查是否存在
	existing, err := s.dashboardMgr.GetDashboard(id)
	if err != nil {
		writeError(w, err.Error(), http.StatusNotFound)
		return
	}

	var req struct {
		Name      string         `json:"name"`
		Widgets   []WidgetConfig `json:"widgets"`
		IsDefault bool           `json:"is_default"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.Widgets != nil {
		existing.Widgets = req.Widgets
	}
	existing.IsDefault = req.IsDefault

	if err := s.dashboardMgr.SaveDashboard(existing); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, existing)
}

// DELETE /api/v1/dashboards/{id} - 删除 Dashboard
func (s *Server) deleteDashboard(w http.ResponseWriter, r *http.Request) {
	if s.dashboardMgr == nil {
		writeError(w, "dashboard manager not initialized", http.StatusServiceUnavailable)
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	// 不能删除默认 dashboard
	dashboard, err := s.dashboardMgr.GetDashboard(id)
	if err != nil {
		writeError(w, err.Error(), http.StatusNotFound)
		return
	}

	if dashboard.IsDefault {
		writeError(w, "cannot delete default dashboard", http.StatusBadRequest)
		return
	}

	if err := s.dashboardMgr.DeleteDashboard(id); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"status": "deleted"})
}
