package integrations

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/netmonitor/backend/analysis"
)

// ESConfig Elasticsearch 配置
type ESConfig struct {
	URL           string `json:"url"`            // ES 地址
	IndexPrefix   string `json:"index_prefix"`   // 索引前缀
	Username      string `json:"username"`       // 用户名
	Password      string `json:"password"`       // 密码
	BatchSize     int    `json:"batch_size"`     // 批量大小
	FlushInterval int    `json:"flush_interval"` // 刷新间隔（秒）
	Enabled       bool   `json:"enabled"`        // 是否启用
}

// ESFlowDoc 存储到 ES 的 Flow 文档
type ESFlowDoc struct {
	Timestamp   time.Time `json:"@timestamp"`
	FlowID      string    `json:"flow_id"`
	SrcIP       string    `json:"src_ip"`
	DstIP       string    `json:"dst_ip"`
	SrcPort     uint16    `json:"src_port"`
	DstPort     uint16    `json:"dst_port"`
	Protocol    string    `json:"protocol"`
	VLANID      uint16    `json:"vlan_id"`
	L7Protocol  string    `json:"l7_protocol"`
	L7Category  string    `json:"l7_category"`
	BytesSent   uint64    `json:"bytes_sent"`
	BytesRecv   uint64    `json:"bytes_recv"`
	PacketsSent uint64    `json:"packets_sent"`
	PacketsRecv uint64    `json:"packets_recv"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time,omitempty"`
	Duration    float64   `json:"duration_sec"`
	IsActive    bool      `json:"is_active"`
}

// ElasticsearchExporter Elasticsearch 导出器
type ElasticsearchExporter struct {
	config      ESConfig
	buffer      []*analysis.Flow
	mu          sync.RWMutex
	running     bool
	stopCh      chan struct{}
	flushTicker *time.Ticker
	httpClient  *http.Client
	lastError   string
}

// NewElasticsearchExporter 创建新的 ES 导出器
func NewElasticsearchExporter(config ESConfig) *ElasticsearchExporter {
	if config.BatchSize <= 0 {
		config.BatchSize = 100
	}
	if config.FlushInterval <= 0 {
		config.FlushInterval = 30
	}
	if config.IndexPrefix == "" {
		config.IndexPrefix = "netmonitor"
	}

	return &ElasticsearchExporter{
		config: config,
		buffer: make([]*analysis.Flow, 0, config.BatchSize),
		stopCh: make(chan struct{}),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Start 启动导出器
func (e *ElasticsearchExporter) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return nil
	}

	if !e.config.Enabled {
		log.Println("Elasticsearch exporter is disabled")
		return nil
	}

	e.running = true
	e.stopCh = make(chan struct{})
	e.flushTicker = time.NewTicker(time.Duration(e.config.FlushInterval) * time.Second)

	// 启动后台刷新 goroutine
	go e.flushLoop()

	log.Println("Elasticsearch exporter started")
	return nil
}

// Stop 停止导出器
func (e *ElasticsearchExporter) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return nil
	}

	e.running = false
	close(e.stopCh)

	if e.flushTicker != nil {
		e.flushTicker.Stop()
	}

	// 刷新剩余数据
	e.flushUnlocked()

	log.Println("Elasticsearch exporter stopped")
	return nil
}

// Export 导出 flow 数据
func (e *ElasticsearchExporter) Export(flow *analysis.Flow) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running || !e.config.Enabled {
		return nil
	}

	e.buffer = append(e.buffer, flow)

	// 达到批量大小，立即刷新
	if len(e.buffer) >= e.config.BatchSize {
		return e.flushUnlocked()
	}

	return nil
}

// ExportBatch 批量导出
func (e *ElasticsearchExporter) ExportBatch(flows []*analysis.Flow) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running || !e.config.Enabled {
		return nil
	}

	for _, flow := range flows {
		e.buffer = append(e.buffer, flow)

		if len(e.buffer) >= e.config.BatchSize {
			if err := e.flushUnlocked(); err != nil {
				return err
			}
		}
	}

	return nil
}

// flushLoop 定期刷新循环
func (e *ElasticsearchExporter) flushLoop() {
	for {
		select {
		case <-e.stopCh:
			return
		case <-e.flushTicker.C:
			e.mu.Lock()
			if len(e.buffer) > 0 {
				e.flushUnlocked()
			}
			e.mu.Unlock()
		}
	}
}

// flushUnlocked 刷新缓冲区（必须在锁内调用）
func (e *ElasticsearchExporter) flushUnlocked() error {
	if len(e.buffer) == 0 {
		return nil
	}

	flows := make([]*analysis.Flow, len(e.buffer))
	copy(flows, e.buffer)
	e.buffer = e.buffer[:0]

	go e.sendToES(flows)
	return nil
}

// sendToES 发送数据到 Elasticsearch
func (e *ElasticsearchExporter) sendToES(flows []*analysis.Flow) {
	if len(flows) == 0 {
		return
	}

	// 构建 bulk 请求体
	var buf bytes.Buffer
	now := time.Now()
	indexName := fmt.Sprintf("%s-%s", e.config.IndexPrefix, now.Format("2006.01.02"))

	for _, flow := range flows {
		// 索引操作行
		meta := map[string]interface{}{
			"index": map[string]string{
				"_index": indexName,
				"_id":    flow.ID,
			},
		}
		metaJSON, _ := json.Marshal(meta)
		buf.Write(metaJSON)
		buf.WriteByte('\n')

		// 文档数据
		doc := e.flowToDoc(flow)
		docJSON, _ := json.Marshal(doc)
		buf.Write(docJSON)
		buf.WriteByte('\n')
	}

	// 发送请求
	url := fmt.Sprintf("%s/_bulk", e.config.URL)
	req, err := http.NewRequest("POST", url, &buf)
	if err != nil {
		e.mu.Lock()
		e.lastError = err.Error()
		e.mu.Unlock()
		log.Printf("ES bulk request creation failed: %v", err)
		return
	}

	req.Header.Set("Content-Type", "application/x-ndjson")

	// 认证
	if e.config.Username != "" && e.config.Password != "" {
		auth := base64.StdEncoding.EncodeToString([]byte(e.config.Username + ":" + e.config.Password))
		req.Header.Set("Authorization", "Basic "+auth)
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		e.mu.Lock()
		e.lastError = err.Error()
		e.mu.Unlock()
		log.Printf("ES bulk request failed: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		e.mu.Lock()
		e.lastError = fmt.Sprintf("HTTP %d", resp.StatusCode)
		e.mu.Unlock()
		log.Printf("ES bulk request failed: HTTP %d", resp.StatusCode)
		return
	}

	// 解析响应检查错误
	var result struct {
		Errors bool `json:"errors"`
		Items  []struct {
			Index struct {
				Error struct {
					Reason string `json:"reason"`
				} `json:"error"`
			} `json:"index"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("ES response decode failed: %v", err)
		return
	}

	if result.Errors {
		var errorCount int
		for _, item := range result.Items {
			if item.Index.Error.Reason != "" {
				errorCount++
			}
		}
		if errorCount > 0 {
			e.mu.Lock()
			e.lastError = fmt.Sprintf("%d items failed to index", errorCount)
			e.mu.Unlock()
			log.Printf("ES bulk indexing had %d errors", errorCount)
		}
	} else {
		e.mu.Lock()
		e.lastError = ""
		e.mu.Unlock()
		log.Printf("ES exported %d flows to %s", len(flows), indexName)
	}
}

// flowToDoc 转换 Flow 为 ES 文档
func (e *ElasticsearchExporter) flowToDoc(flow *analysis.Flow) *ESFlowDoc {
	doc := &ESFlowDoc{
		Timestamp:   time.Now(),
		FlowID:      flow.ID,
		SrcIP:       flow.SrcIP,
		DstIP:       flow.DstIP,
		SrcPort:     flow.SrcPort,
		DstPort:     flow.DstPort,
		Protocol:    flow.Protocol,
		VLANID:      flow.VLANID,
		L7Protocol:  flow.L7Protocol,
		L7Category:  flow.L7Category,
		BytesSent:   flow.BytesSent,
		BytesRecv:   flow.BytesRecv,
		PacketsSent: flow.PacketsSent,
		PacketsRecv: flow.PacketsRecv,
		StartTime:   flow.StartTime,
		IsActive:    flow.IsActive,
	}

	// 计算持续时间
	if flow.IsActive {
		doc.Duration = time.Since(flow.StartTime).Seconds()
	} else {
		doc.Duration = flow.LastSeen.Sub(flow.StartTime).Seconds()
		doc.EndTime = flow.LastSeen
	}

	return doc
}

// GetStatus 获取导出器状态
func (e *ElasticsearchExporter) GetStatus() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return map[string]interface{}{
		"enabled":        e.config.Enabled,
		"running":        e.running,
		"url":            e.config.URL,
		"index_prefix":   e.config.IndexPrefix,
		"batch_size":     e.config.BatchSize,
		"flush_interval": e.config.FlushInterval,
		"buffer_size":    len(e.buffer),
		"last_error":     e.lastError,
	}
}

// UpdateConfig 更新配置
func (e *ElasticsearchExporter) UpdateConfig(config ESConfig) error {
	wasRunning := e.running

	// 停止当前导出器
	if wasRunning {
		e.Stop()
	}

	// 更新配置
	e.mu.Lock()
	e.config = config
	if config.BatchSize <= 0 {
		e.config.BatchSize = 100
	}
	if config.FlushInterval <= 0 {
		e.config.FlushInterval = 30
	}
	if config.IndexPrefix == "" {
		e.config.IndexPrefix = "netmonitor"
	}
	e.buffer = make([]*analysis.Flow, 0, e.config.BatchSize)
	e.mu.Unlock()

	// 如果之前正在运行，重新启动
	if wasRunning && config.Enabled {
		return e.Start()
	}

	return nil
}

// TestConnection 测试 ES 连接
func (e *ElasticsearchExporter) TestConnection() error {
	url := fmt.Sprintf("%s/_cluster/health", e.config.URL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	// 认证
	if e.config.Username != "" && e.config.Password != "" {
		auth := base64.StdEncoding.EncodeToString([]byte(e.config.Username + ":" + e.config.Password))
		req.Header.Set("Authorization", "Basic "+auth)
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// 解析响应
	var result struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	if result.Status == "red" {
		return fmt.Errorf("cluster health is red")
	}

	return nil
}

// ForceFlush 强制刷新缓冲区
func (e *ElasticsearchExporter) ForceFlush() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.flushUnlocked()
}
