package alerts

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// AlertEngine 告警引擎
type AlertEngine struct {
	db              *sql.DB
	checks          []Check
	rules           []*AlertRule
	cooldowns       map[string]time.Time // ruleID:entityID -> 上次告警时间
	lastCtx         *CheckContext
	notificationMgr *NotificationManager
	ruleManager     *RuleManager
	mu              sync.RWMutex
	stopCh          chan struct{}
}

// NewAlertEngine 创建告警引擎实例
func NewAlertEngine(db *sql.DB) *AlertEngine {
	return &AlertEngine{
		db:        db,
		checks:    make([]Check, 0),
		rules:     make([]*AlertRule, 0),
		cooldowns: make(map[string]time.Time),
		stopCh:    make(chan struct{}),
	}
}

// Start 启动告警引擎后台 goroutine，每 30 秒评估一次规则
func (e *AlertEngine) Start() {
	// Load rules from database
	rm := &RuleManager{db: e.db}
	e.ruleManager = rm
	rules, err := rm.LoadRules()
	if err != nil {
		log.Printf("[AlertEngine] failed to load rules: %v", err)
	} else {
		e.mu.Lock()
		e.rules = rules
		e.mu.Unlock()
		log.Printf("[AlertEngine] loaded %d rules", len(rules))
	}

	// Ensure default rules exist
	rm.EnsureDefaultRules()
	// Reload after ensuring defaults
	rules, err = rm.LoadRules()
	if err == nil {
		e.mu.Lock()
		e.rules = rules
		e.mu.Unlock()
	}

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		log.Println("[AlertEngine] started, evaluating rules every 30s")
		for {
			select {
			case <-ticker.C:
				e.evaluateRules()
			case <-e.stopCh:
				log.Println("[AlertEngine] stopped")
				return
			}
		}
	}()
}

// Stop 优雅停止告警引擎
func (e *AlertEngine) Stop() {
	close(e.stopCh)
}

// SetNotificationManager 设置通知管理器
func (e *AlertEngine) SetNotificationManager(mgr *NotificationManager) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.notificationMgr = mgr
}

// RegisterCheck 注册行为检查
func (e *AlertEngine) RegisterCheck(check Check) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.checks = append(e.checks, check)
	log.Printf("[AlertEngine] registered check: %s", check.Name())
}

// TriggerAlert 触发告警（写入数据库，检查冷却期）
func (e *AlertEngine) TriggerAlert(alert Alert) error {
	// Check cooldown
	if alert.RuleID != "" && e.checkCooldown(alert.RuleID, alert.EntityID) {
		return nil // still in cooldown, skip
	}

	// Set defaults
	if alert.Status == "" {
		alert.Status = StatusTriggered
	}
	if alert.TriggeredAt.IsZero() {
		alert.TriggeredAt = time.Now()
	}

	// Insert into database
	result, err := e.db.Exec(`
		INSERT INTO alerts (type, severity, status, rule_id, title, description,
			entity_type, entity_id, metadata, triggered_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, alert.Type, alert.Severity, alert.Status, alert.RuleID, alert.Title,
		alert.Description, alert.EntityType, alert.EntityID, alert.Metadata,
		alert.TriggeredAt)
	if err != nil {
		return fmt.Errorf("insert alert: %w", err)
	}

	id, _ := result.LastInsertId()
	alert.ID = id
	log.Printf("[AlertEngine] alert triggered: id=%d type=%s severity=%s title=%s",
		id, alert.Type, alert.Severity, alert.Title)

	// Send notifications (async, non-blocking)
	if e.notificationMgr != nil {
		e.notificationMgr.NotifyAll(&alert)
	}

	// Update cooldown
	if alert.RuleID != "" {
		cooldownKey := alert.RuleID + ":" + alert.EntityID
		e.mu.Lock()
		e.cooldowns[cooldownKey] = time.Now()
		e.mu.Unlock()
	}

	return nil
}

// AcknowledgeAlert 确认告警
func (e *AlertEngine) AcknowledgeAlert(id int64) error {
	now := time.Now()
	result, err := e.db.Exec(`
		UPDATE alerts SET status = ?, acked_at = ? WHERE id = ? AND status = ?
	`, StatusAcknowledged, now, id, StatusTriggered)
	if err != nil {
		return fmt.Errorf("acknowledge alert: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("alert %d not found or not in triggered state", id)
	}
	return nil
}

// ResolveAlert 解决告警
func (e *AlertEngine) ResolveAlert(id int64) error {
	now := time.Now()
	result, err := e.db.Exec(`
		UPDATE alerts SET status = ?, resolved_at = ? WHERE id = ? AND status IN (?, ?)
	`, StatusResolved, now, id, StatusTriggered, StatusAcknowledged)
	if err != nil {
		return fmt.Errorf("resolve alert: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("alert %d not found or already resolved", id)
	}
	return nil
}

// GetAlerts 查询告警列表（带过滤和分页）
func (e *AlertEngine) GetAlerts(filter AlertFilter) ([]Alert, int, error) {
	whereClauses := []string{"1=1"}
	args := []interface{}{}

	if filter.Type != "" {
		whereClauses = append(whereClauses, "type = ?")
		args = append(args, filter.Type)
	}
	if filter.Severity != "" {
		whereClauses = append(whereClauses, "severity = ?")
		args = append(args, filter.Severity)
	}
	if filter.Status != "" {
		whereClauses = append(whereClauses, "status = ?")
		args = append(args, filter.Status)
	}
	if filter.EntityID != "" {
		whereClauses = append(whereClauses, "entity_id = ?")
		args = append(args, filter.EntityID)
	}
	if filter.Start != nil {
		whereClauses = append(whereClauses, "triggered_at >= ?")
		args = append(args, *filter.Start)
	}
	if filter.End != nil {
		whereClauses = append(whereClauses, "triggered_at <= ?")
		args = append(args, *filter.End)
	}

	whereStr := strings.Join(whereClauses, " AND ")

	// Count total
	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM alerts WHERE %s", whereStr)
	if err := e.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count alerts: %w", err)
	}

	// Apply pagination defaults
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	// Fetch data
	dataQuery := fmt.Sprintf(`
		SELECT id, type, severity, status, rule_id, title, description,
			entity_type, entity_id, metadata, triggered_at, acked_at, resolved_at, created_at
		FROM alerts
		WHERE %s
		ORDER BY triggered_at DESC
		LIMIT ? OFFSET ?
	`, whereStr)
	dataArgs := append(args, limit, offset)

	rows, err := e.db.Query(dataQuery, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query alerts: %w", err)
	}
	defer rows.Close()

	var alerts []Alert
	for rows.Next() {
		var a Alert
		var ruleID, description, entityType, entityID, metadata sql.NullString
		var ackedAt, resolvedAt *time.Time

		if err := rows.Scan(&a.ID, &a.Type, &a.Severity, &a.Status,
			&ruleID, &a.Title, &description,
			&entityType, &entityID, &metadata,
			&a.TriggeredAt, &ackedAt, &resolvedAt, &a.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan alert: %w", err)
		}

		if ruleID.Valid {
			a.RuleID = ruleID.String
		}
		if description.Valid {
			a.Description = description.String
		}
		if entityType.Valid {
			a.EntityType = entityType.String
		}
		if entityID.Valid {
			a.EntityID = entityID.String
		}
		if metadata.Valid {
			a.Metadata = metadata.String
		}
		a.AckedAt = ackedAt
		a.ResolvedAt = resolvedAt

		alerts = append(alerts, a)
	}

	return alerts, total, nil
}

// GetAlertStats 告警统计（按类型、严重级别分组计数）
func (e *AlertEngine) GetAlertStats() (*AlertStats, error) {
	stats := &AlertStats{
		ByType:     make(map[string]int),
		BySeverity: make(map[string]int),
		ByStatus:   make(map[string]int),
	}

	// Total count
	if err := e.db.QueryRow("SELECT COUNT(*) FROM alerts").Scan(&stats.Total); err != nil {
		return nil, fmt.Errorf("count total alerts: %w", err)
	}

	// By type
	rows, err := e.db.Query("SELECT type, COUNT(*) FROM alerts GROUP BY type")
	if err != nil {
		return nil, fmt.Errorf("count by type: %w", err)
	}
	for rows.Next() {
		var t string
		var c int
		rows.Scan(&t, &c)
		stats.ByType[t] = c
	}
	rows.Close()

	// By severity
	rows, err = e.db.Query("SELECT severity, COUNT(*) FROM alerts GROUP BY severity")
	if err != nil {
		return nil, fmt.Errorf("count by severity: %w", err)
	}
	for rows.Next() {
		var s string
		var c int
		rows.Scan(&s, &c)
		stats.BySeverity[s] = c
	}
	rows.Close()

	// By status
	rows, err = e.db.Query("SELECT status, COUNT(*) FROM alerts GROUP BY status")
	if err != nil {
		return nil, fmt.Errorf("count by status: %w", err)
	}
	for rows.Next() {
		var s string
		var c int
		rows.Scan(&s, &c)
		stats.ByStatus[s] = c
	}
	rows.Close()

	// Last 24 hours
	since := time.Now().Add(-24 * time.Hour)
	if err := e.db.QueryRow("SELECT COUNT(*) FROM alerts WHERE triggered_at >= ?", since).Scan(&stats.Last24Hours); err != nil {
		return nil, fmt.Errorf("count last 24h alerts: %w", err)
	}

	return stats, nil
}

// GetActiveAlertCount 获取活跃告警数量（triggered + acknowledged）
func (e *AlertEngine) GetActiveAlertCount() int {
	var count int
	err := e.db.QueryRow("SELECT COUNT(*) FROM alerts WHERE status IN (?, ?)",
		StatusTriggered, StatusAcknowledged).Scan(&count)
	if err != nil {
		return 0
	}
	return count
}

// evaluateRules 评估所有规则（定期调用）
func (e *AlertEngine) evaluateRules() {
	e.mu.RLock()
	rules := make([]*AlertRule, len(e.rules))
	copy(rules, e.rules)
	checks := make([]Check, len(e.checks))
	copy(checks, e.checks)
	e.mu.RUnlock()

	// Use stored check context or empty one
	e.mu.RLock()
	ctx := e.lastCtx
	e.mu.RUnlock()
	if ctx == nil {
		ctx = &CheckContext{}
	}

	// Run registered checks
	for _, check := range checks {
		alertsFromCheck := check.Check(ctx)
		for _, a := range alertsFromCheck {
			if err := e.TriggerAlert(a); err != nil {
				log.Printf("[AlertEngine] failed to trigger alert from check %s: %v", check.Name(), err)
			}
		}
	}

	// Evaluate threshold rules
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		e.evaluateThresholdRule(rule, ctx)
	}
}

// evaluateThresholdRule 评估单个阈值规则
func (e *AlertEngine) evaluateThresholdRule(rule *AlertRule, ctx *CheckContext) {
	var currentValue float64
	var entityID string

	switch rule.Condition.Metric {
	case "bandwidth":
		currentValue = ctx.BytesPerSec
		entityID = "system"
	case "connections", "flows":
		currentValue = float64(ctx.ActiveFlows)
		entityID = "system"
	case "hosts":
		currentValue = float64(ctx.ActiveHosts)
		entityID = "system"
	case "packets":
		currentValue = ctx.PacketsPerSec
		entityID = "system"
	default:
		return // unknown metric
	}

	triggered := false
	switch rule.Condition.Operator {
	case "gt":
		triggered = currentValue > rule.Condition.Threshold
	case "gte":
		triggered = currentValue >= rule.Condition.Threshold
	case "lt":
		triggered = currentValue < rule.Condition.Threshold
	case "lte":
		triggered = currentValue <= rule.Condition.Threshold
	case "eq":
		triggered = currentValue == rule.Condition.Threshold
	}

	if triggered {
		alert := Alert{
			Type:        rule.Type,
			Severity:    rule.Severity,
			Status:      StatusTriggered,
			RuleID:      rule.ID,
			Title:       fmt.Sprintf("[%s] %s", rule.Severity, rule.Name),
			Description: fmt.Sprintf("%s: current value %.2f exceeds threshold %.2f", rule.Condition.Metric, currentValue, rule.Condition.Threshold),
			EntityType:  string(rule.Type),
			EntityID:    entityID,
			TriggeredAt: time.Now(),
		}
		if err := e.TriggerAlert(alert); err != nil {
			log.Printf("[AlertEngine] failed to trigger rule %s: %v", rule.ID, err)
		}
	}
}

// UpdateCheckContext 更新检查上下文（由外部定期调用）
func (e *AlertEngine) UpdateCheckContext(ctx *CheckContext) {
	e.mu.Lock()
	defer e.mu.Unlock()
	// Store for use in evaluateRules
	e.lastCtx = ctx
}

// GetRuleManager 返回规则管理器
func (e *AlertEngine) GetRuleManager() *RuleManager {
	return e.ruleManager
}

// GetNotificationManager 返回通知管理器
func (e *AlertEngine) GetNotificationManager() *NotificationManager {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.notificationMgr
}

// checkCooldown 检查冷却期，返回 true 表示仍在冷却中
func (e *AlertEngine) checkCooldown(ruleID, entityID string) bool {
	cooldownKey := ruleID + ":" + entityID
	e.mu.RLock()
	lastTime, ok := e.cooldowns[cooldownKey]
	e.mu.RUnlock()

	if !ok {
		return false
	}

	// Find the rule's cooldown duration
	e.mu.RLock()
	var cooldownSec int
	for _, r := range e.rules {
		if r.ID == ruleID {
			cooldownSec = r.CooldownSec
			break
		}
	}
	e.mu.RUnlock()

	if cooldownSec <= 0 {
		cooldownSec = 300 // default 5 minutes
	}

	return time.Since(lastTime) < time.Duration(cooldownSec)*time.Second
}
