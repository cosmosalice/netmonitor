package alerts

import "time"

// AlertType 告警类型
type AlertType string

const (
	AlertTypeFlow      AlertType = "flow"
	AlertTypeHost      AlertType = "host"
	AlertTypeInterface AlertType = "interface"
	AlertTypeSystem    AlertType = "system"
)

// AlertSeverity 告警严重级别
type AlertSeverity string

const (
	SeverityInfo     AlertSeverity = "info"
	SeverityWarning  AlertSeverity = "warning"
	SeverityError    AlertSeverity = "error"
	SeverityCritical AlertSeverity = "critical"
)

// AlertStatus 告警状态
type AlertStatus string

const (
	StatusTriggered    AlertStatus = "triggered"
	StatusAcknowledged AlertStatus = "acknowledged"
	StatusResolved     AlertStatus = "resolved"
)

// Alert 告警实体
type Alert struct {
	ID          int64         `json:"id"`
	Type        AlertType     `json:"type"`
	Severity    AlertSeverity `json:"severity"`
	Status      AlertStatus   `json:"status"`
	RuleID      string        `json:"rule_id"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	EntityType  string        `json:"entity_type"`
	EntityID    string        `json:"entity_id"`
	Metadata    string        `json:"metadata"`
	TriggeredAt time.Time     `json:"triggered_at"`
	AckedAt     *time.Time    `json:"acked_at,omitempty"`
	ResolvedAt  *time.Time    `json:"resolved_at,omitempty"`
	CreatedAt   time.Time     `json:"created_at"`
}

// AlertRule 告警规则
type AlertRule struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Type        AlertType     `json:"type"`
	Severity    AlertSeverity `json:"severity"`
	Enabled     bool          `json:"enabled"`
	Condition   RuleCondition `json:"condition"`
	CooldownSec int           `json:"cooldown_sec"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

// RuleCondition 规则条件
type RuleCondition struct {
	Metric    string  `json:"metric"`
	Operator  string  `json:"operator"`
	Threshold float64 `json:"threshold"`
	WindowSec int     `json:"window_sec"`
	GroupBy   string  `json:"group_by,omitempty"`
}

// AlertFilter 告警查询过滤条件
type AlertFilter struct {
	Type     AlertType     `json:"type,omitempty"`
	Severity AlertSeverity `json:"severity,omitempty"`
	Status   AlertStatus   `json:"status,omitempty"`
	Start    *time.Time    `json:"start,omitempty"`
	End      *time.Time    `json:"end,omitempty"`
	EntityID string        `json:"entity_id,omitempty"`
	Limit    int           `json:"limit"`
	Offset   int           `json:"offset"`
}

// AlertStats 告警统计
type AlertStats struct {
	Total       int            `json:"total"`
	ByType      map[string]int `json:"by_type"`
	BySeverity  map[string]int `json:"by_severity"`
	ByStatus    map[string]int `json:"by_status"`
	Last24Hours int            `json:"last_24_hours"`
}

// Check 行为检查接口 — 后续 Task 会实现具体检查
type Check interface {
	Name() string
	Description() string
	Type() AlertType
	DefaultSeverity() AlertSeverity
	Check(ctx *CheckContext) []Alert
}

// CheckContext 检查上下文，提供检查所需的数据
type CheckContext struct {
	// 全局指标
	ActiveFlows   int
	ActiveHosts   int
	BytesPerSec   float64
	PacketsPerSec float64

	// Flow 数据（供 flow 级检查使用）
	Flows []FlowInfo
	// 主机数据（供 host 级检查使用）
	Hosts []HostInfo
}

// FlowInfo 流信息（简化类型，避免循环依赖）
type FlowInfo struct {
	FlowID      string
	SrcIP       string
	DstIP       string
	SrcPort     uint16
	DstPort     uint16
	Protocol    string
	L7Protocol  string
	BytesSent   uint64
	BytesRecv   uint64
	PacketsSent uint64
	PacketsRecv uint64
	StartTime   time.Time
	LastSeen    time.Time
	IsActive    bool
	// TCP 指标
	Retransmissions int
	RTTMs           float64
}

// HostInfo 主机信息（简化类型，避免循环依赖）
type HostInfo struct {
	IP          string
	BytesSent   uint64
	BytesRecv   uint64
	PacketsSent uint64
	PacketsRecv uint64
	FlowCount   int
	Protocols   map[string]uint64
	FirstSeen   time.Time
	LastSeen    time.Time
}
