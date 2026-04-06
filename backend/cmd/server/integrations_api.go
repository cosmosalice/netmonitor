package main

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/netmonitor/backend/integrations"
)

// IntegrationManager 管理所有集成
type IntegrationManager struct {
	db      *sql.DB
	syslog  *integrations.SyslogForwarder
	elastic *integrations.ElasticsearchExporter
}

// NewIntegrationManager 创建集成管理器
func NewIntegrationManager(db *sql.DB) *IntegrationManager {
	return &IntegrationManager{
		db: db,
	}
}

// Init 从数据库加载配置并初始化
func (im *IntegrationManager) Init() error {
	// 加载 Syslog 配置
	syslogConfig, err := im.loadSyslogConfig()
	if err != nil {
		return err
	}
	im.syslog = integrations.NewSyslogForwarder(*syslogConfig)
	if syslogConfig.Enabled {
		if err := im.syslog.Start(); err != nil {
			return err
		}
	}

	// 加载 ES 配置
	esConfig, err := im.loadESConfig()
	if err != nil {
		return err
	}
	im.elastic = integrations.NewElasticsearchExporter(*esConfig)
	if esConfig.Enabled {
		if err := im.elastic.Start(); err != nil {
			return err
		}
	}

	return nil
}

// Shutdown 关闭所有集成
func (im *IntegrationManager) Shutdown() {
	if im.syslog != nil {
		im.syslog.Stop()
	}
	if im.elastic != nil {
		im.elastic.Stop()
	}
}

// loadSyslogConfig 从数据库加载 Syslog 配置
func (im *IntegrationManager) loadSyslogConfig() (*integrations.SyslogConfig, error) {
	config := &integrations.SyslogConfig{
		Target:   "localhost",
		Port:     514,
		Protocol: "udp",
		Facility: 16, // local0
		Severity: 6,  // info
		Enabled:  false,
	}

	row := im.db.QueryRow(
		"SELECT config, enabled FROM integrations WHERE id = 'syslog'",
	)

	var configJSON string
	var enabled int
	err := row.Scan(&configJSON, &enabled)
	if err != nil {
		// 如果没有记录，返回默认配置
		return config, nil
	}

	if err := json.Unmarshal([]byte(configJSON), config); err != nil {
		return config, nil
	}
	config.Enabled = enabled == 1

	return config, nil
}

// loadESConfig 从数据库加载 ES 配置
func (im *IntegrationManager) loadESConfig() (*integrations.ESConfig, error) {
	config := &integrations.ESConfig{
		URL:           "http://localhost:9200",
		IndexPrefix:   "netmonitor",
		BatchSize:     100,
		FlushInterval: 30,
		Enabled:       false,
	}

	row := im.db.QueryRow(
		"SELECT config, enabled FROM integrations WHERE id = 'elasticsearch'",
	)

	var configJSON string
	var enabled int
	err := row.Scan(&configJSON, &enabled)
	if err != nil {
		return config, nil
	}

	if err := json.Unmarshal([]byte(configJSON), config); err != nil {
		return config, nil
	}
	config.Enabled = enabled == 1

	return config, nil
}

// saveIntegrationConfig 保存集成配置到数据库
func (im *IntegrationManager) saveIntegrationConfig(id string, config interface{}, enabled bool) error {
	configJSON, err := json.Marshal(config)
	if err != nil {
		return err
	}

	_, err = im.db.Exec(
		`INSERT INTO integrations (id, type, config, enabled, updated_at) 
		 VALUES (?, ?, ?, ?, datetime('now'))
		 ON CONFLICT(id) DO UPDATE SET 
		 config = excluded.config, 
		 enabled = excluded.enabled,
		 updated_at = excluded.updated_at`,
		id, id, string(configJSON), boolToInt(enabled),
	)
	return err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// ---------------------------------------------------------------------------
// HTTP Handlers
// ---------------------------------------------------------------------------

// GET /api/v1/integrations - 所有集成状态
func (s *Server) getIntegrations(w http.ResponseWriter, r *http.Request) {
	if s.integrationMgr == nil {
		writeJSON(w, map[string]interface{}{
			"integrations": []interface{}{},
		})
		return
	}

	statuses := []map[string]interface{}{}

	if s.integrationMgr.syslog != nil {
		status := s.integrationMgr.syslog.GetStatus()
		status["id"] = "syslog"
		status["name"] = "Syslog"
		status["type"] = "forwarder"
		statuses = append(statuses, status)
	}

	if s.integrationMgr.elastic != nil {
		status := s.integrationMgr.elastic.GetStatus()
		status["id"] = "elasticsearch"
		status["name"] = "Elasticsearch"
		status["type"] = "exporter"
		statuses = append(statuses, status)
	}

	writeJSON(w, map[string]interface{}{
		"integrations": statuses,
	})
}

// GET /api/v1/integrations/syslog - Syslog 配置
func (s *Server) getSyslogConfig(w http.ResponseWriter, r *http.Request) {
	if s.integrationMgr == nil || s.integrationMgr.syslog == nil {
		writeError(w, "syslog not initialized", http.StatusServiceUnavailable)
		return
	}

	status := s.integrationMgr.syslog.GetStatus()
	writeJSON(w, status)
}

// PUT /api/v1/integrations/syslog - 更新 Syslog 配置
func (s *Server) updateSyslogConfig(w http.ResponseWriter, r *http.Request) {
	if s.integrationMgr == nil {
		writeError(w, "integration manager not initialized", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		Target   string `json:"target"`
		Port     int    `json:"port"`
		Protocol string `json:"protocol"`
		Facility int    `json:"facility"`
		Severity int    `json:"severity"`
		UseTLS   bool   `json:"use_tls"`
		Enabled  bool   `json:"enabled"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	config := integrations.SyslogConfig{
		Target:   req.Target,
		Port:     req.Port,
		Protocol: req.Protocol,
		Facility: req.Facility,
		Severity: req.Severity,
		UseTLS:   req.UseTLS,
		Enabled:  req.Enabled,
	}

	// 保存到数据库
	if err := s.integrationMgr.saveIntegrationConfig("syslog", config, config.Enabled); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 更新转发器
	if err := s.integrationMgr.syslog.UpdateConfig(config); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"status": "ok"})
}

// POST /api/v1/integrations/syslog/test - 测试 Syslog 连接
func (s *Server) testSyslogConnection(w http.ResponseWriter, r *http.Request) {
	if s.integrationMgr == nil {
		writeError(w, "integration manager not initialized", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		Target   string `json:"target"`
		Port     int    `json:"port"`
		Protocol string `json:"protocol"`
		UseTLS   bool   `json:"use_tls"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	config := integrations.SyslogConfig{
		Target:   req.Target,
		Port:     req.Port,
		Protocol: req.Protocol,
		UseTLS:   req.UseTLS,
	}

	tempForwarder := integrations.NewSyslogForwarder(config)
	if err := tempForwarder.TestConnection(); err != nil {
		writeJSON(w, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	writeJSON(w, map[string]interface{}{
		"success": true,
		"message": "Connection successful",
	})
}

// GET /api/v1/integrations/elasticsearch - ES 配置
func (s *Server) getESConfig(w http.ResponseWriter, r *http.Request) {
	if s.integrationMgr == nil || s.integrationMgr.elastic == nil {
		writeError(w, "elasticsearch not initialized", http.StatusServiceUnavailable)
		return
	}

	status := s.integrationMgr.elastic.GetStatus()
	writeJSON(w, status)
}

// PUT /api/v1/integrations/elasticsearch - 更新 ES 配置
func (s *Server) updateESConfig(w http.ResponseWriter, r *http.Request) {
	if s.integrationMgr == nil {
		writeError(w, "integration manager not initialized", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		URL           string `json:"url"`
		IndexPrefix   string `json:"index_prefix"`
		Username      string `json:"username"`
		Password      string `json:"password"`
		BatchSize     int    `json:"batch_size"`
		FlushInterval int    `json:"flush_interval"`
		Enabled       bool   `json:"enabled"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	config := integrations.ESConfig{
		URL:           req.URL,
		IndexPrefix:   req.IndexPrefix,
		Username:      req.Username,
		Password:      req.Password,
		BatchSize:     req.BatchSize,
		FlushInterval: req.FlushInterval,
		Enabled:       req.Enabled,
	}

	// 保存到数据库
	if err := s.integrationMgr.saveIntegrationConfig("elasticsearch", config, config.Enabled); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 更新导出器
	if err := s.integrationMgr.elastic.UpdateConfig(config); err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"status": "ok"})
}

// POST /api/v1/integrations/elasticsearch/test - 测试 ES 连接
func (s *Server) testESConnection(w http.ResponseWriter, r *http.Request) {
	if s.integrationMgr == nil {
		writeError(w, "integration manager not initialized", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		URL      string `json:"url"`
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, err.Error(), http.StatusBadRequest)
		return
	}

	config := integrations.ESConfig{
		URL:      req.URL,
		Username: req.Username,
		Password: req.Password,
	}

	tempExporter := integrations.NewElasticsearchExporter(config)
	if err := tempExporter.TestConnection(); err != nil {
		writeJSON(w, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	writeJSON(w, map[string]interface{}{
		"success": true,
		"message": "Connection successful",
	})
}
