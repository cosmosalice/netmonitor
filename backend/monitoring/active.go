package monitoring

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/netmonitor/backend/storage"
)

// MonitorTarget represents a monitoring target
type MonitorTarget struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"` // "ping", "tcp", "http"
	Host     string `json:"host"`
	Port     int    `json:"port,omitempty"`
	URL      string `json:"url,omitempty"`
	Interval int    `json:"interval_sec"` // 检测间隔秒
	Timeout  int    `json:"timeout_ms"`
	Enabled  bool   `json:"enabled"`
	Status   string `json:"status"` // "up", "down", "unknown"
}

// MonitorResult represents a single monitoring check result
type MonitorResult struct {
	Timestamp time.Time `json:"timestamp"`
	Status    string    `json:"status"`
	Latency   float64   `json:"latency_ms"`
	Error     string    `json:"error,omitempty"`
}

// ActiveMonitor manages active monitoring targets
type ActiveMonitor struct {
	mu         sync.RWMutex
	targets    map[string]*MonitorTarget
	results    map[string][]MonitorResult
	stopCh     chan struct{}
	db         *storage.Database
	httpClient *http.Client
}

// NewActiveMonitor creates a new active monitor
func NewActiveMonitor(db *storage.Database) *ActiveMonitor {
	return &ActiveMonitor{
		targets: make(map[string]*MonitorTarget),
		results: make(map[string][]MonitorResult),
		stopCh:  make(chan struct{}),
		db:      db,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				DisableKeepAlives: false,
				MaxIdleConns:      10,
				IdleConnTimeout:   30 * time.Second,
			},
		},
	}
}

// Start begins monitoring all enabled targets
func (m *ActiveMonitor) Start() error {
	// Load existing targets from database
	if err := m.loadTargets(); err != nil {
		log.Printf("Failed to load monitor targets: %v", err)
	}

	// Load existing results
	if err := m.loadResults(); err != nil {
		log.Printf("Failed to load monitor results: %v", err)
	}

	// Start monitoring goroutines for each enabled target
	m.mu.RLock()
	for _, target := range m.targets {
		if target.Enabled {
			go m.monitorLoop(target)
		}
	}
	m.mu.RUnlock()

	log.Println("Active monitor started")
	return nil
}

// Stop stops all monitoring goroutines
func (m *ActiveMonitor) Stop() {
	close(m.stopCh)
}

// AddTarget adds a new monitoring target
func (m *ActiveMonitor) AddTarget(target MonitorTarget) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Generate ID if not provided
	if target.ID == "" {
		target.ID = fmt.Sprintf("mon_%d", time.Now().UnixNano())
	}

	// Set defaults
	if target.Interval == 0 {
		target.Interval = 60
	}
	if target.Timeout == 0 {
		target.Timeout = 5000
	}
	target.Status = "unknown"

	// Validate target
	if err := m.validateTarget(&target); err != nil {
		return err
	}

	// Save to database
	if err := m.saveTargetToDB(&target); err != nil {
		return fmt.Errorf("failed to save target: %w", err)
	}

	m.targets[target.ID] = &target
	m.results[target.ID] = make([]MonitorResult, 0)

	log.Printf("Monitor target added: %s (%s)", target.Name, target.Host)

	// Start monitoring if enabled
	if target.Enabled {
		go m.monitorLoop(&target)
	}

	return nil
}

// RemoveTarget removes a monitoring target
func (m *ActiveMonitor) RemoveTarget(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.targets, id)
	delete(m.results, id)

	// Remove from database
	if m.db != nil {
		db := m.db.GetDB()
		_, err := db.Exec("DELETE FROM monitor_targets WHERE id = ?", id)
		if err != nil {
			log.Printf("Failed to delete monitor target from DB: %v", err)
		}
		_, _ = db.Exec("DELETE FROM monitor_results WHERE target_id = ?", id)
	}

	log.Printf("Monitor target removed: %s", id)
}

// GetTargets returns all monitoring targets
func (m *ActiveMonitor) GetTargets() []MonitorTarget {
	m.mu.RLock()
	defer m.mu.RUnlock()

	targets := make([]MonitorTarget, 0, len(m.targets))
	for _, t := range m.targets {
		targets = append(targets, *t)
	}
	return targets
}

// GetTarget returns a specific target by ID
func (m *ActiveMonitor) GetTarget(id string) *MonitorTarget {
	m.mu.RLock()
	defer m.mu.RUnlock()

	t, ok := m.targets[id]
	if !ok {
		return nil
	}
	targetCopy := *t
	return &targetCopy
}

// GetResults returns monitoring results for a target
func (m *ActiveMonitor) GetResults(targetID string, limit int) []MonitorResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results, ok := m.results[targetID]
	if !ok {
		return []MonitorResult{}
	}

	if limit <= 0 || limit > len(results) {
		limit = len(results)
	}

	// Return most recent results
	start := len(results) - limit
	if start < 0 {
		start = 0
	}

	resultCopy := make([]MonitorResult, limit)
	for i := 0; i < limit; i++ {
		resultCopy[i] = results[start+i]
	}
	return resultCopy
}

// GetSummary returns monitoring summary statistics
func (m *ActiveMonitor) GetSummary() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	upCount := 0
	downCount := 0
	unknownCount := 0

	for _, t := range m.targets {
		switch t.Status {
		case "up":
			upCount++
		case "down":
			downCount++
		default:
			unknownCount++
		}
	}

	return map[string]interface{}{
		"total":   len(m.targets),
		"up":      upCount,
		"down":    downCount,
		"unknown": unknownCount,
	}
}

// CheckTarget performs a single check on a target
func (m *ActiveMonitor) CheckTarget(target *MonitorTarget) MonitorResult {
	start := time.Now()
	result := MonitorResult{
		Timestamp: start,
		Status:    "unknown",
	}

	timeout := time.Duration(target.Timeout) * time.Millisecond

	switch target.Type {
	case "ping":
		result = m.checkPing(target.Host, timeout)
	case "tcp":
		result = m.checkTCP(target.Host, target.Port, timeout)
	case "http":
		result = m.checkHTTP(target.URL, timeout)
	default:
		result.Status = "unknown"
		result.Error = "unknown target type"
	}

	return result
}

// checkPing performs a ping check (using TCP fallback on Windows)
func (m *ActiveMonitor) checkPing(host string, timeout time.Duration) MonitorResult {
	result := MonitorResult{
		Timestamp: time.Now(),
		Status:    "down",
	}

	// Try ICMP ping first
	start := time.Now()
	conn, err := net.DialTimeout("ip4:icmp", host, timeout)
	if err == nil {
		defer conn.Close()
		result.Latency = float64(time.Since(start).Milliseconds())
		result.Status = "up"
		return result
	}

	// Fallback to TCP ping on port 80
	start = time.Now()
	conn, err = net.DialTimeout("tcp", net.JoinHostPort(host, "80"), timeout)
	if err == nil {
		conn.Close()
		result.Latency = float64(time.Since(start).Milliseconds())
		result.Status = "up"
		return result
	}

	// Try port 443
	start = time.Now()
	conn, err = net.DialTimeout("tcp", net.JoinHostPort(host, "443"), timeout)
	if err == nil {
		conn.Close()
		result.Latency = float64(time.Since(start).Milliseconds())
		result.Status = "up"
		return result
	}

	result.Error = err.Error()
	return result
}

// checkTCP performs a TCP connection check
func (m *ActiveMonitor) checkTCP(host string, port int, timeout time.Duration) MonitorResult {
	result := MonitorResult{
		Timestamp: time.Now(),
		Status:    "down",
	}

	if port == 0 {
		result.Error = "port not specified"
		return result
	}

	start := time.Now()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, fmt.Sprintf("%d", port)), timeout)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer conn.Close()

	result.Latency = float64(time.Since(start).Milliseconds())
	result.Status = "up"
	return result
}

// checkHTTP performs an HTTP check
func (m *ActiveMonitor) checkHTTP(url string, timeout time.Duration) MonitorResult {
	result := MonitorResult{
		Timestamp: time.Now(),
		Status:    "down",
	}

	if url == "" {
		result.Error = "URL not specified"
		return result
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer resp.Body.Close()

	// Read body to ensure connection is complete
	io.Copy(io.Discard, resp.Body)

	result.Latency = float64(time.Since(start).Milliseconds())

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		result.Status = "up"
	} else {
		result.Status = "down"
		result.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}

	return result
}

// monitorLoop runs the monitoring loop for a target
func (m *ActiveMonitor) monitorLoop(target *MonitorTarget) {
	interval := time.Duration(target.Interval) * time.Second
	if interval < 5*time.Second {
		interval = 5 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Check immediately
	m.performCheck(target)

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			// Check if target still exists and is enabled
			m.mu.RLock()
			currentTarget, exists := m.targets[target.ID]
			m.mu.RUnlock()

			if !exists || !currentTarget.Enabled {
				return
			}

			m.performCheck(currentTarget)
		}
	}
}

// performCheck performs a check and stores the result
func (m *ActiveMonitor) performCheck(target *MonitorTarget) {
	result := m.CheckTarget(target)

	m.mu.Lock()
	// Update target status
	target.Status = result.Status

	// Store result
	results := m.results[target.ID]
	results = append(results, result)

	// Keep only last 100 results
	if len(results) > 100 {
		results = results[len(results)-100:]
	}
	m.results[target.ID] = results
	m.mu.Unlock()

	// Save to database
	if m.db != nil {
		m.saveResultToDB(target.ID, &result)
		m.updateTargetStatusInDB(target)
	}
}

// validateTarget validates a monitoring target
func (m *ActiveMonitor) validateTarget(target *MonitorTarget) error {
	switch target.Type {
	case "ping":
		if target.Host == "" {
			return fmt.Errorf("host is required for ping target")
		}
	case "tcp":
		if target.Host == "" {
			return fmt.Errorf("host is required for TCP target")
		}
		if target.Port == 0 {
			return fmt.Errorf("port is required for TCP target")
		}
	case "http":
		if target.URL == "" {
			return fmt.Errorf("URL is required for HTTP target")
		}
	default:
		return fmt.Errorf("unknown target type: %s", target.Type)
	}
	return nil
}

// saveTargetToDB saves a target to the database
func (m *ActiveMonitor) saveTargetToDB(target *MonitorTarget) error {
	if m.db == nil {
		return nil
	}

	db := m.db.GetDB()
	_, err := db.Exec(`
		INSERT INTO monitor_targets (id, name, type, host, port, url, interval_sec, timeout_ms, enabled, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			type = excluded.type,
			host = excluded.host,
			port = excluded.port,
			url = excluded.url,
			interval_sec = excluded.interval_sec,
			timeout_ms = excluded.timeout_ms,
			enabled = excluded.enabled,
			status = excluded.status
	`, target.ID, target.Name, target.Type, target.Host, target.Port, target.URL,
		target.Interval, target.Timeout, target.Enabled, target.Status)

	return err
}

// saveResultToDB saves a result to the database
func (m *ActiveMonitor) saveResultToDB(targetID string, result *MonitorResult) error {
	if m.db == nil {
		return nil
	}

	db := m.db.GetDB()
	_, err := db.Exec(`
		INSERT INTO monitor_results (target_id, timestamp, status, latency_ms, error)
		VALUES (?, ?, ?, ?, ?)
	`, targetID, result.Timestamp, result.Status, result.Latency, result.Error)

	return err
}

// updateTargetStatusInDB updates target status in database
func (m *ActiveMonitor) updateTargetStatusInDB(target *MonitorTarget) error {
	if m.db == nil {
		return nil
	}

	db := m.db.GetDB()
	_, err := db.Exec("UPDATE monitor_targets SET status = ? WHERE id = ?", target.Status, target.ID)
	return err
}

// loadTargets loads targets from the database
func (m *ActiveMonitor) loadTargets() error {
	if m.db == nil {
		return nil
	}

	db := m.db.GetDB()
	rows, err := db.Query(`
		SELECT id, name, type, host, port, url, interval_sec, timeout_ms, enabled, status
		FROM monitor_targets
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var t MonitorTarget
		err := rows.Scan(&t.ID, &t.Name, &t.Type, &t.Host, &t.Port, &t.URL,
			&t.Interval, &t.Timeout, &t.Enabled, &t.Status)
		if err != nil {
			log.Printf("Failed to scan monitor target: %v", err)
			continue
		}
		m.targets[t.ID] = &t
		m.results[t.ID] = make([]MonitorResult, 0)
	}

	return rows.Err()
}

// loadResults loads recent results from the database
func (m *ActiveMonitor) loadResults() error {
	if m.db == nil {
		return nil
	}

	db := m.db.GetDB()
	rows, err := db.Query(`
		SELECT target_id, timestamp, status, latency_ms, error
		FROM monitor_results
		WHERE timestamp > datetime('now', '-1 day')
		ORDER BY timestamp
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var targetID string
		var r MonitorResult
		err := rows.Scan(&targetID, &r.Timestamp, &r.Status, &r.Latency, &r.Error)
		if err != nil {
			continue
		}

		if _, ok := m.results[targetID]; ok {
			m.results[targetID] = append(m.results[targetID], r)
		}
	}

	// Trim to last 100 per target
	for id, results := range m.results {
		if len(results) > 100 {
			m.results[id] = results[len(results)-100:]
		}
	}

	return rows.Err()
}
