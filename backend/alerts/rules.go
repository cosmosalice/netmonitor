package alerts

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// RuleManager 规则管理
type RuleManager struct {
	db *sql.DB
}

// NewRuleManager 创建规则管理器
func NewRuleManager(db *sql.DB) *RuleManager {
	return &RuleManager{db: db}
}

// LoadRules 从数据库加载所有规则
func (rm *RuleManager) LoadRules() ([]*AlertRule, error) {
	rows, err := rm.db.Query(`
		SELECT id, name, description, type, severity, enabled,
			condition_json, cooldown_sec, created_at, updated_at
		FROM alert_rules
	`)
	if err != nil {
		return nil, fmt.Errorf("query rules: %w", err)
	}
	defer rows.Close()

	var rules []*AlertRule
	for rows.Next() {
		var r AlertRule
		var condJSON string
		var enabled int
		var description sql.NullString

		if err := rows.Scan(&r.ID, &r.Name, &description, &r.Type, &r.Severity,
			&enabled, &condJSON, &r.CooldownSec, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan rule: %w", err)
		}

		if description.Valid {
			r.Description = description.String
		}
		r.Enabled = enabled == 1

		if err := json.Unmarshal([]byte(condJSON), &r.Condition); err != nil {
			log.Printf("[RuleManager] failed to parse condition for rule %s: %v", r.ID, err)
			continue
		}

		rules = append(rules, &r)
	}

	return rules, nil
}

// SaveRule 保存/更新规则
func (rm *RuleManager) SaveRule(rule *AlertRule) error {
	condJSON, err := json.Marshal(rule.Condition)
	if err != nil {
		return fmt.Errorf("marshal condition: %w", err)
	}

	enabled := 0
	if rule.Enabled {
		enabled = 1
	}

	now := time.Now()
	_, err = rm.db.Exec(`
		INSERT INTO alert_rules (id, name, description, type, severity, enabled,
			condition_json, cooldown_sec, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			type = excluded.type,
			severity = excluded.severity,
			enabled = excluded.enabled,
			condition_json = excluded.condition_json,
			cooldown_sec = excluded.cooldown_sec,
			updated_at = excluded.updated_at
	`, rule.ID, rule.Name, rule.Description, rule.Type, rule.Severity,
		enabled, string(condJSON), rule.CooldownSec, now, now)
	if err != nil {
		return fmt.Errorf("save rule: %w", err)
	}

	return nil
}

// DeleteRule 删除规则
func (rm *RuleManager) DeleteRule(id string) error {
	result, err := rm.db.Exec("DELETE FROM alert_rules WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete rule: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("rule %s not found", id)
	}
	return nil
}

// GetRules 获取所有规则
func (rm *RuleManager) GetRules() []*AlertRule {
	rules, err := rm.LoadRules()
	if err != nil {
		log.Printf("[RuleManager] failed to load rules: %v", err)
		return nil
	}
	return rules
}

// GetRule 获取单个规则
func (rm *RuleManager) GetRule(id string) (*AlertRule, error) {
	var r AlertRule
	var condJSON string
	var enabled int
	var description sql.NullString

	err := rm.db.QueryRow(`
		SELECT id, name, description, type, severity, enabled,
			condition_json, cooldown_sec, created_at, updated_at
		FROM alert_rules WHERE id = ?
	`, id).Scan(&r.ID, &r.Name, &description, &r.Type, &r.Severity,
		&enabled, &condJSON, &r.CooldownSec, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("rule not found: %w", err)
	}

	if description.Valid {
		r.Description = description.String
	}
	r.Enabled = enabled == 1

	if err := json.Unmarshal([]byte(condJSON), &r.Condition); err != nil {
		return nil, fmt.Errorf("parse condition: %w", err)
	}

	return &r, nil
}

// EnsureDefaultRules 确保默认规则存在（首次启动时自动创建）
func (rm *RuleManager) EnsureDefaultRules() {
	defaultRules := []*AlertRule{
		{
			ID:          "high_bandwidth",
			Name:        "High Bandwidth Alert",
			Description: "Bandwidth exceeds 100MB/s",
			Type:        AlertTypeSystem,
			Severity:    SeverityWarning,
			Enabled:     true,
			Condition: RuleCondition{
				Metric:    "bandwidth",
				Operator:  "gt",
				Threshold: 100 * 1024 * 1024, // 100MB/s in bytes
				WindowSec: 60,
			},
			CooldownSec: 300,
		},
		{
			ID:          "too_many_flows",
			Name:        "Too Many Active Flows",
			Description: "Active flows exceed 50000",
			Type:        AlertTypeFlow,
			Severity:    SeverityWarning,
			Enabled:     true,
			Condition: RuleCondition{
				Metric:    "flows",
				Operator:  "gt",
				Threshold: 50000,
				WindowSec: 60,
			},
			CooldownSec: 300,
		},
		{
			ID:          "too_many_hosts",
			Name:        "Too Many Active Hosts",
			Description: "Active hosts exceed 1000",
			Type:        AlertTypeHost,
			Severity:    SeverityInfo,
			Enabled:     true,
			Condition: RuleCondition{
				Metric:    "hosts",
				Operator:  "gt",
				Threshold: 1000,
				WindowSec: 60,
			},
			CooldownSec: 300,
		},
	}

	for _, rule := range defaultRules {
		// Check if rule already exists
		_, err := rm.GetRule(rule.ID)
		if err != nil {
			// Rule doesn't exist, create it
			if err := rm.SaveRule(rule); err != nil {
				log.Printf("[RuleManager] failed to create default rule %s: %v", rule.ID, err)
			} else {
				log.Printf("[RuleManager] created default rule: %s", rule.ID)
			}
		}
	}
}
