package reports

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ReportConfig 报表配置
type ReportConfig struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Type        string     `json:"type"`
	Enabled     bool       `json:"enabled"`
	OutputDir   string     `json:"output_dir"`
	LastGenTime *time.Time `json:"last_gen_time,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// Report 已生成的报表
type Report struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Period      string    `json:"period"`
	GeneratedAt time.Time `json:"generated_at"`
	FilePath    string    `json:"file_path"`
	FileSize    int64     `json:"file_size"`
	CreatedAt   time.Time `json:"created_at"`
}

// ReportGenerator 报表生成器
type ReportGenerator struct {
	db        *sql.DB
	outputDir string
	stopCh    chan struct{}
	mu        sync.Mutex
}

// NewReportGenerator 创建报表生成器
func NewReportGenerator(db *sql.DB, outputDir string) *ReportGenerator {
	if outputDir == "" {
		outputDir = "reports"
	}
	// Ensure output directory exists
	os.MkdirAll(outputDir, 0755)

	rg := &ReportGenerator{
		db:        db,
		outputDir: outputDir,
		stopCh:    make(chan struct{}),
	}
	rg.ensureDefaultConfigs()
	return rg
}

// Start 启动定时调度
func (rg *ReportGenerator) Start() {
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		log.Println("[ReportGenerator] started, checking schedule every hour")
		for {
			select {
			case <-ticker.C:
				rg.checkSchedule()
			case <-rg.stopCh:
				log.Println("[ReportGenerator] stopped")
				return
			}
		}
	}()
}

// Stop 停止调度
func (rg *ReportGenerator) Stop() {
	close(rg.stopCh)
}

// checkSchedule 检查是否需要生成报表
func (rg *ReportGenerator) checkSchedule() {
	now := time.Now()

	configs, err := rg.GetReportConfigs()
	if err != nil {
		log.Printf("[ReportGenerator] failed to load configs: %v", err)
		return
	}

	for _, cfg := range configs {
		if !cfg.Enabled {
			continue
		}

		switch cfg.Type {
		case "daily":
			// 每天 00:05 生成前一天报表
			if now.Hour() == 0 && now.Minute() >= 5 && now.Minute() < 65 {
				yesterday := now.AddDate(0, 0, -1)
				period := yesterday.Format("2006-01-02")
				if !rg.reportExists(cfg.Type, period) {
					if _, err := rg.GenerateDaily(yesterday); err != nil {
						log.Printf("[ReportGenerator] daily report error: %v", err)
					}
				}
			}
		case "weekly":
			// 每周一 00:10 生成上周报表
			if now.Weekday() == time.Monday && now.Hour() == 0 && now.Minute() >= 10 && now.Minute() < 70 {
				weekStart := now.AddDate(0, 0, -7)
				year, week := weekStart.ISOWeek()
				period := fmt.Sprintf("%d-W%02d", year, week)
				if !rg.reportExists(cfg.Type, period) {
					if _, err := rg.GenerateWeekly(weekStart); err != nil {
						log.Printf("[ReportGenerator] weekly report error: %v", err)
					}
				}
			}
		case "monthly":
			// 每月1号 00:15 生成上月报表
			if now.Day() == 1 && now.Hour() == 0 && now.Minute() >= 15 && now.Minute() < 75 {
				lastMonth := now.AddDate(0, -1, 0)
				period := lastMonth.Format("2006-01")
				if !rg.reportExists(cfg.Type, period) {
					if _, err := rg.GenerateMonthly(lastMonth); err != nil {
						log.Printf("[ReportGenerator] monthly report error: %v", err)
					}
				}
			}
		}
	}
}

// reportExists 检查报表是否已存在
func (rg *ReportGenerator) reportExists(reportType, period string) bool {
	var count int
	err := rg.db.QueryRow("SELECT COUNT(*) FROM reports WHERE type = ? AND period = ?", reportType, period).Scan(&count)
	if err != nil {
		return false
	}
	return count > 0
}

// GenerateDaily 生成日报
func (rg *ReportGenerator) GenerateDaily(date time.Time) (*Report, error) {
	rg.mu.Lock()
	defer rg.mu.Unlock()

	start := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.Local)
	end := start.AddDate(0, 0, 1).Add(-time.Second)
	period := date.Format("2006-01-02")

	data, err := rg.collectReportData(fmt.Sprintf("日报 - %s", period), period, start, end)
	if err != nil {
		return nil, fmt.Errorf("collect data: %w", err)
	}

	return rg.generateAndSave("daily", period, data)
}

// GenerateWeekly 生成周报
func (rg *ReportGenerator) GenerateWeekly(weekStart time.Time) (*Report, error) {
	rg.mu.Lock()
	defer rg.mu.Unlock()

	// Align to Monday
	for weekStart.Weekday() != time.Monday {
		weekStart = weekStart.AddDate(0, 0, -1)
	}
	start := time.Date(weekStart.Year(), weekStart.Month(), weekStart.Day(), 0, 0, 0, 0, time.Local)
	end := start.AddDate(0, 0, 7).Add(-time.Second)
	year, week := start.ISOWeek()
	period := fmt.Sprintf("%d-W%02d", year, week)

	data, err := rg.collectReportData(fmt.Sprintf("周报 - %s", period), period, start, end)
	if err != nil {
		return nil, fmt.Errorf("collect data: %w", err)
	}

	return rg.generateAndSave("weekly", period, data)
}

// GenerateMonthly 生成月报
func (rg *ReportGenerator) GenerateMonthly(month time.Time) (*Report, error) {
	rg.mu.Lock()
	defer rg.mu.Unlock()

	start := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.Local)
	end := start.AddDate(0, 1, 0).Add(-time.Second)
	period := month.Format("2006-01")

	data, err := rg.collectReportData(fmt.Sprintf("月报 - %s", period), period, start, end)
	if err != nil {
		return nil, fmt.Errorf("collect data: %w", err)
	}

	return rg.generateAndSave("monthly", period, data)
}

// collectReportData 收集报表所需数据
func (rg *ReportGenerator) collectReportData(title, period string, start, end time.Time) (*ReportData, error) {
	data := &ReportData{
		Title:       title,
		Period:      period,
		GeneratedAt: time.Now(),
	}

	// 1. 流量摘要 - 从 flows 表统计
	err := rg.db.QueryRow(`
		SELECT COALESCE(SUM(bytes_sent + bytes_recv), 0),
		       COALESCE(SUM(packets_sent + packets_recv), 0),
		       COUNT(*),
		       COUNT(DISTINCT src_ip) + COUNT(DISTINCT dst_ip)
		FROM flows
		WHERE start_time >= ? AND start_time <= ?
	`, start, end).Scan(&data.Summary.TotalBytes, &data.Summary.TotalPackets,
		&data.Summary.TotalFlows, &data.Summary.UniqueHosts)
	if err != nil {
		log.Printf("[ReportGenerator] summary query error: %v", err)
	}

	// 计算平均和峰值带宽
	durationSec := end.Sub(start).Seconds()
	if durationSec > 0 && data.Summary.TotalBytes > 0 {
		data.Summary.AvgBandwidth = float64(data.Summary.TotalBytes) / durationSec
	}

	// 峰值带宽 - 从 timeseries 表查
	var peakBw sql.NullFloat64
	rg.db.QueryRow(`
		SELECT MAX(value) FROM timeseries
		WHERE metric_type = 'bandwidth' AND metric_key = 'bytes_per_sec'
		AND timestamp >= ? AND timestamp <= ?
	`, start, end).Scan(&peakBw)
	if peakBw.Valid {
		data.Summary.PeakBandwidth = peakBw.Float64
	}

	// 2. Top 10 主机
	hostRows, err := rg.db.Query(`
		WITH host_traffic AS (
			SELECT src_ip AS ip, SUM(bytes_sent) AS bytes_out, SUM(bytes_recv) AS bytes_in, COUNT(*) AS flows
			FROM flows WHERE start_time >= ? AND start_time <= ?
			GROUP BY src_ip
			UNION ALL
			SELECT dst_ip AS ip, SUM(bytes_recv) AS bytes_out, SUM(bytes_sent) AS bytes_in, COUNT(*) AS flows
			FROM flows WHERE start_time >= ? AND start_time <= ?
			GROUP BY dst_ip
		)
		SELECT ip, SUM(bytes_out + bytes_in) AS total, SUM(bytes_out) AS sent, SUM(bytes_in) AS recv, SUM(flows) AS flow_count
		FROM host_traffic
		GROUP BY ip
		ORDER BY total DESC
		LIMIT 10
	`, start, end, start, end)
	if err == nil {
		defer hostRows.Close()
		for hostRows.Next() {
			var h HostEntry
			hostRows.Scan(&h.IP, &h.TotalBytes, &h.BytesSent, &h.BytesRecv, &h.FlowCount)
			data.TopHosts = append(data.TopHosts, h)
		}
	}

	// 3. Top 10 协议
	protoRows, err := rg.db.Query(`
		SELECT COALESCE(l7_protocol, protocol) AS proto,
		       COALESCE(l7_category, '') AS category,
		       SUM(bytes_sent + bytes_recv) AS total_bytes,
		       COUNT(*) AS flow_count
		FROM flows
		WHERE start_time >= ? AND start_time <= ?
		GROUP BY proto, category
		ORDER BY total_bytes DESC
		LIMIT 10
	`, start, end)
	if err == nil {
		defer protoRows.Close()
		var protoTotal uint64
		var protos []ProtocolEntry
		for protoRows.Next() {
			var p ProtocolEntry
			protoRows.Scan(&p.Protocol, &p.Category, &p.TotalBytes, &p.FlowCount)
			protoTotal += p.TotalBytes
			protos = append(protos, p)
		}
		for i := range protos {
			if protoTotal > 0 {
				protos[i].Percentage = float64(protos[i].TotalBytes) / float64(protoTotal) * 100
			}
		}
		data.TopProtocols = protos
	}

	// 4. 告警摘要
	rg.db.QueryRow(`SELECT COUNT(*) FROM alerts WHERE triggered_at >= ? AND triggered_at <= ?`,
		start, end).Scan(&data.AlertSummary.Total)

	sevRows, err := rg.db.Query(`
		SELECT severity, COUNT(*) FROM alerts
		WHERE triggered_at >= ? AND triggered_at <= ?
		GROUP BY severity
	`, start, end)
	if err == nil {
		defer sevRows.Close()
		data.AlertSummary.BySeverity = make(map[string]int)
		for sevRows.Next() {
			var sev string
			var cnt int
			sevRows.Scan(&sev, &cnt)
			data.AlertSummary.BySeverity[sev] = cnt
		}
	}

	// 5. 每小时流量趋势
	hourlyRows, err := rg.db.Query(`
		SELECT strftime('%Y-%m-%d %H:00:00', start_time) AS hour_bucket,
		       SUM(bytes_sent + bytes_recv) AS total_bytes
		FROM flows
		WHERE start_time >= ? AND start_time <= ?
		GROUP BY hour_bucket
		ORDER BY hour_bucket ASC
	`, start, end)
	if err == nil {
		defer hourlyRows.Close()
		for hourlyRows.Next() {
			var hp HourlyPoint
			var hourStr string
			hourlyRows.Scan(&hourStr, &hp.Bytes)
			hp.Hour = hourStr
			data.HourlyTraffic = append(data.HourlyTraffic, hp)
		}
	}

	return data, nil
}

// generateAndSave 生成 HTML 并保存
func (rg *ReportGenerator) generateAndSave(reportType, period string, data *ReportData) (*Report, error) {
	html, err := GenerateHTML(data)
	if err != nil {
		return nil, fmt.Errorf("generate HTML: %w", err)
	}

	// Ensure output dir
	os.MkdirAll(rg.outputDir, 0755)

	filename := fmt.Sprintf("report_%s_%s.html", reportType, period)
	filePath := filepath.Join(rg.outputDir, filename)

	if err := os.WriteFile(filePath, []byte(html), 0644); err != nil {
		return nil, fmt.Errorf("write file: %w", err)
	}

	fi, _ := os.Stat(filePath)
	fileSize := int64(0)
	if fi != nil {
		fileSize = fi.Size()
	}

	id := fmt.Sprintf("%s_%s_%d", reportType, period, time.Now().UnixMilli())
	name := data.Title
	now := time.Now()

	_, err = rg.db.Exec(`
		INSERT INTO reports (id, name, type, period, generated_at, file_path, file_size, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, id, name, reportType, period, now, filePath, fileSize, now)
	if err != nil {
		return nil, fmt.Errorf("save report record: %w", err)
	}

	// Update config's last_gen_time
	rg.db.Exec(`UPDATE report_configs SET last_gen_time = ?, updated_at = ? WHERE type = ?`, now, now, reportType)

	log.Printf("[ReportGenerator] generated %s report: %s (%d bytes)", reportType, filePath, fileSize)

	return &Report{
		ID:          id,
		Name:        name,
		Type:        reportType,
		Period:      period,
		GeneratedAt: now,
		FilePath:    filePath,
		FileSize:    fileSize,
		CreatedAt:   now,
	}, nil
}

// GetReports 获取已生成的报表列表
func (rg *ReportGenerator) GetReports() ([]Report, error) {
	rows, err := rg.db.Query(`
		SELECT id, name, type, period, generated_at, file_path, file_size, created_at
		FROM reports
		ORDER BY generated_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("query reports: %w", err)
	}
	defer rows.Close()

	var reports []Report
	for rows.Next() {
		var r Report
		if err := rows.Scan(&r.ID, &r.Name, &r.Type, &r.Period, &r.GeneratedAt, &r.FilePath, &r.FileSize, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan report: %w", err)
		}
		reports = append(reports, r)
	}
	if reports == nil {
		reports = []Report{}
	}
	return reports, nil
}

// GetReportByID 获取单个报表
func (rg *ReportGenerator) GetReportByID(id string) (*Report, error) {
	var r Report
	err := rg.db.QueryRow(`
		SELECT id, name, type, period, generated_at, file_path, file_size, created_at
		FROM reports WHERE id = ?
	`, id).Scan(&r.ID, &r.Name, &r.Type, &r.Period, &r.GeneratedAt, &r.FilePath, &r.FileSize, &r.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("report not found: %w", err)
	}
	return &r, nil
}

// GetReportConfigs 获取报表配置
func (rg *ReportGenerator) GetReportConfigs() ([]ReportConfig, error) {
	rows, err := rg.db.Query(`
		SELECT id, name, type, enabled, output_dir, last_gen_time, created_at, updated_at
		FROM report_configs
		ORDER BY type ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query configs: %w", err)
	}
	defer rows.Close()

	var configs []ReportConfig
	for rows.Next() {
		var c ReportConfig
		var enabled int
		var lastGen *time.Time
		if err := rows.Scan(&c.ID, &c.Name, &c.Type, &enabled, &c.OutputDir, &lastGen, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan config: %w", err)
		}
		c.Enabled = enabled == 1
		c.LastGenTime = lastGen
		configs = append(configs, c)
	}
	if configs == nil {
		configs = []ReportConfig{}
	}
	return configs, nil
}

// SaveReportConfig 保存报表配置
func (rg *ReportGenerator) SaveReportConfig(config ReportConfig) error {
	enabled := 0
	if config.Enabled {
		enabled = 1
	}
	if config.OutputDir == "" {
		config.OutputDir = "reports"
	}
	now := time.Now()

	_, err := rg.db.Exec(`
		INSERT INTO report_configs (id, name, type, enabled, output_dir, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			enabled = excluded.enabled,
			output_dir = excluded.output_dir,
			updated_at = excluded.updated_at
	`, config.ID, config.Name, config.Type, enabled, config.OutputDir, now, now)
	if err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	return nil
}

// ensureDefaultConfigs 确保默认配置存在
func (rg *ReportGenerator) ensureDefaultConfigs() {
	defaults := []ReportConfig{
		{ID: "daily", Name: "日报", Type: "daily", Enabled: true, OutputDir: rg.outputDir},
		{ID: "weekly", Name: "周报", Type: "weekly", Enabled: true, OutputDir: rg.outputDir},
		{ID: "monthly", Name: "月报", Type: "monthly", Enabled: true, OutputDir: rg.outputDir},
	}
	for _, d := range defaults {
		var count int
		rg.db.QueryRow("SELECT COUNT(*) FROM report_configs WHERE id = ?", d.ID).Scan(&count)
		if count == 0 {
			rg.SaveReportConfig(d)
		}
	}
}
