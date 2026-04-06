package reports

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
)

// ReportData 报表数据
type ReportData struct {
	Title         string
	Period        string
	GeneratedAt   interface{} // time.Time
	Summary       TrafficSummary
	TopHosts      []HostEntry
	TopProtocols  []ProtocolEntry
	AlertSummary  AlertSummaryData
	HourlyTraffic []HourlyPoint
}

// TrafficSummary 流量摘要
type TrafficSummary struct {
	TotalBytes    uint64  `json:"total_bytes"`
	TotalPackets  uint64  `json:"total_packets"`
	TotalFlows    int     `json:"total_flows"`
	UniqueHosts   int     `json:"unique_hosts"`
	AvgBandwidth  float64 `json:"avg_bandwidth"`
	PeakBandwidth float64 `json:"peak_bandwidth"`
}

// HostEntry 主机条目
type HostEntry struct {
	IP         string `json:"ip"`
	TotalBytes uint64 `json:"total_bytes"`
	BytesSent  uint64 `json:"bytes_sent"`
	BytesRecv  uint64 `json:"bytes_recv"`
	FlowCount  int    `json:"flow_count"`
}

// ProtocolEntry 协议条目
type ProtocolEntry struct {
	Protocol   string  `json:"protocol"`
	Category   string  `json:"category"`
	TotalBytes uint64  `json:"total_bytes"`
	FlowCount  int     `json:"flow_count"`
	Percentage float64 `json:"percentage"`
}

// AlertSummaryData 告警摘要
type AlertSummaryData struct {
	Total      int            `json:"total"`
	BySeverity map[string]int `json:"by_severity"`
}

// HourlyPoint 每小时流量
type HourlyPoint struct {
	Hour  string `json:"hour"`
	Bytes uint64 `json:"bytes"`
}

// formatBytes 格式化字节数
func formatBytes(b uint64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)
	switch {
	case b >= TB:
		return fmt.Sprintf("%.2f TB", float64(b)/float64(TB))
	case b >= GB:
		return fmt.Sprintf("%.2f GB", float64(b)/float64(GB))
	case b >= MB:
		return fmt.Sprintf("%.2f MB", float64(b)/float64(MB))
	case b >= KB:
		return fmt.Sprintf("%.2f KB", float64(b)/float64(KB))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// formatBandwidth 格式化带宽
func formatBandwidth(bps float64) string {
	if bps >= 1e9 {
		return fmt.Sprintf("%.2f Gbps", bps*8/1e9)
	} else if bps >= 1e6 {
		return fmt.Sprintf("%.2f Mbps", bps*8/1e6)
	} else if bps >= 1e3 {
		return fmt.Sprintf("%.2f Kbps", bps*8/1e3)
	}
	return fmt.Sprintf("%.0f bps", bps*8)
}

// GenerateHTML 生成 HTML 格式报表
func GenerateHTML(data *ReportData) (string, error) {
	funcMap := template.FuncMap{
		"formatBytes":     formatBytes,
		"formatBandwidth": formatBandwidth,
		"barWidth": func(pct float64) string {
			if pct > 100 {
				pct = 100
			}
			return fmt.Sprintf("%.1f%%", pct)
		},
		"hostBarWidth": func(bytes uint64, hosts []HostEntry) string {
			if len(hosts) == 0 || hosts[0].TotalBytes == 0 {
				return "0%"
			}
			pct := float64(bytes) / float64(hosts[0].TotalBytes) * 100
			return fmt.Sprintf("%.1f%%", pct)
		},
		"severityColor": func(sev string) string {
			switch strings.ToLower(sev) {
			case "critical":
				return "#ff4444"
			case "error":
				return "#ff6b35"
			case "warning":
				return "#ffa500"
			case "info":
				return "#00d4ff"
			default:
				return "#888"
			}
		},
		"hourlyBarHeight": func(bytes uint64, points []HourlyPoint) string {
			if len(points) == 0 {
				return "0%"
			}
			var maxBytes uint64
			for _, p := range points {
				if p.Bytes > maxBytes {
					maxBytes = p.Bytes
				}
			}
			if maxBytes == 0 {
				return "0%"
			}
			pct := float64(bytes) / float64(maxBytes) * 100
			if pct < 2 {
				pct = 2
			}
			return fmt.Sprintf("%.1f%%", pct)
		},
		"shortHour": func(h string) string {
			// "2026-04-05 14:00:00" -> "14:00"
			if len(h) >= 16 {
				return h[11:16]
			}
			return h
		},
		"add": func(a, b int) int { return a + b },
	}

	tmpl, err := template.New("report").Funcs(funcMap).Parse(reportTemplate)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}

const reportTemplate = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>{{.Title}}</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{background:#1a1a2e;color:#e0e0e0;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;line-height:1.6;padding:20px}
.container{max-width:1100px;margin:0 auto}
.header{text-align:center;padding:30px 0;border-bottom:2px solid #0f3460;margin-bottom:30px}
.header h1{color:#00d4ff;font-size:28px;margin-bottom:8px}
.header .subtitle{color:#888;font-size:14px}
.section{background:#16213e;border-radius:10px;padding:24px;margin-bottom:24px;border:1px solid #0f3460}
.section h2{color:#00d4ff;font-size:18px;margin-bottom:16px;padding-bottom:8px;border-bottom:1px solid #0f346080}
.summary-grid{display:grid;grid-template-columns:repeat(3,1fr);gap:16px}
.stat-card{background:#1a1a2e;border-radius:8px;padding:16px;text-align:center;border:1px solid #0f346060}
.stat-card .value{font-size:24px;font-weight:700;color:#00d4ff}
.stat-card .label{font-size:12px;color:#888;margin-top:4px}
table{width:100%;border-collapse:collapse;margin-top:8px}
th{text-align:left;padding:10px 12px;background:#1a1a2e;color:#00d4ff;font-size:13px;border-bottom:2px solid #0f3460}
td{padding:10px 12px;border-bottom:1px solid #0f346040;font-size:13px}
tr:hover{background:#1a1a2e80}
.bar-container{background:#1a1a2e;border-radius:4px;height:20px;overflow:hidden;position:relative}
.bar-fill{height:100%;background:linear-gradient(90deg,#0f3460,#00d4ff);border-radius:4px;transition:width .3s}
.chart-container{display:flex;align-items:flex-end;gap:2px;height:150px;padding:10px 0;border-bottom:1px solid #0f346060}
.chart-bar-wrapper{flex:1;display:flex;flex-direction:column;align-items:center;height:100%}
.chart-bar{width:100%;background:linear-gradient(180deg,#00d4ff,#0f3460);border-radius:3px 3px 0 0;margin-top:auto;min-width:8px;transition:height .3s}
.chart-label{font-size:9px;color:#888;margin-top:4px;transform:rotate(-45deg);white-space:nowrap}
.alert-badges{display:flex;gap:10px;flex-wrap:wrap;margin-top:8px}
.badge{padding:6px 16px;border-radius:6px;font-size:13px;font-weight:600}
.footer{text-align:center;padding:20px 0;color:#555;font-size:12px;border-top:1px solid #0f346040;margin-top:30px}
</style>
</head>
<body>
<div class="container">
<div class="header">
<h1>{{.Title}}</h1>
<div class="subtitle">时段: {{.Period}} | 生成时间: {{.GeneratedAt}}</div>
</div>

<div class="section">
<h2>📊 流量摘要</h2>
<div class="summary-grid">
<div class="stat-card"><div class="value">{{formatBytes .Summary.TotalBytes}}</div><div class="label">总流量</div></div>
<div class="stat-card"><div class="value">{{.Summary.TotalPackets}}</div><div class="label">总数据包</div></div>
<div class="stat-card"><div class="value">{{.Summary.TotalFlows}}</div><div class="label">总连接数</div></div>
<div class="stat-card"><div class="value">{{.Summary.UniqueHosts}}</div><div class="label">独立主机</div></div>
<div class="stat-card"><div class="value">{{formatBandwidth .Summary.AvgBandwidth}}</div><div class="label">平均带宽</div></div>
<div class="stat-card"><div class="value">{{formatBandwidth .Summary.PeakBandwidth}}</div><div class="label">峰值带宽</div></div>
</div>
</div>

{{if .HourlyTraffic}}
<div class="section">
<h2>📈 每小时流量趋势</h2>
<div class="chart-container">
{{range .HourlyTraffic}}
<div class="chart-bar-wrapper">
<div class="chart-bar" style="height:{{hourlyBarHeight .Bytes $.HourlyTraffic}}" title="{{shortHour .Hour}}: {{formatBytes .Bytes}}"></div>
<div class="chart-label">{{shortHour .Hour}}</div>
</div>
{{end}}
</div>
</div>
{{end}}

{{if .TopHosts}}
<div class="section">
<h2>💻 Top 10 主机</h2>
<table>
<thead><tr><th>#</th><th>IP 地址</th><th>总流量</th><th>发送</th><th>接收</th><th>连接数</th><th>占比</th></tr></thead>
<tbody>
{{range $i, $h := .TopHosts}}
<tr>
<td>{{add $i 1}}</td>
<td style="font-family:monospace">{{$h.IP}}</td>
<td>{{formatBytes $h.TotalBytes}}</td>
<td>{{formatBytes $h.BytesSent}}</td>
<td>{{formatBytes $h.BytesRecv}}</td>
<td>{{$h.FlowCount}}</td>
<td><div class="bar-container"><div class="bar-fill" style="width:{{hostBarWidth $h.TotalBytes $.TopHosts}}"></div></div></td>
</tr>
{{end}}
</tbody>
</table>
</div>
{{end}}

{{if .TopProtocols}}
<div class="section">
<h2>🔍 Top 10 协议分布</h2>
<table>
<thead><tr><th>#</th><th>协议</th><th>类别</th><th>流量</th><th>连接数</th><th>占比</th></tr></thead>
<tbody>
{{range $i, $p := .TopProtocols}}
<tr>
<td>{{add $i 1}}</td>
<td style="font-weight:600;color:#00d4ff">{{$p.Protocol}}</td>
<td>{{$p.Category}}</td>
<td>{{formatBytes $p.TotalBytes}}</td>
<td>{{$p.FlowCount}}</td>
<td>
<div style="display:flex;align-items:center;gap:8px">
<div class="bar-container" style="flex:1"><div class="bar-fill" style="width:{{barWidth $p.Percentage}}"></div></div>
<span style="font-size:12px;min-width:45px;text-align:right">{{printf "%.1f%%" $p.Percentage}}</span>
</div>
</td>
</tr>
{{end}}
</tbody>
</table>
</div>
{{end}}

<div class="section">
<h2>🚨 告警摘要</h2>
<p style="font-size:16px;margin-bottom:12px">时段内共 <strong style="color:#00d4ff">{{.AlertSummary.Total}}</strong> 条告警</p>
{{if .AlertSummary.BySeverity}}
<div class="alert-badges">
{{range $sev, $cnt := .AlertSummary.BySeverity}}
<div class="badge" style="background:{{severityColor $sev}}22;border:1px solid {{severityColor $sev}};color:{{severityColor $sev}}">{{$sev}}: {{$cnt}}</div>
{{end}}
</div>
{{else}}
<p style="color:#888">暂无告警数据</p>
{{end}}
</div>

<div class="footer">
NetMonitor Report — Generated automatically
</div>
</div>
</body>
</html>`
