package alerts

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// BlacklistManager IP 黑名单管理器
type BlacklistManager struct {
	mu          sync.RWMutex
	ipBlacklist map[string]*BlacklistEntry
	cidrList    []*cidrEntry // CIDR 范围匹配
	sources     []BlacklistSource
	engine      *AlertEngine
	stopCh      chan struct{}
}

// cidrEntry 存储 CIDR 范围及其对应的黑名单条目
type cidrEntry struct {
	network *net.IPNet
	entry   *BlacklistEntry
}

// BlacklistEntry 黑名单条目
type BlacklistEntry struct {
	IP          string     `json:"ip"`
	Source      string     `json:"source"`
	Category    string     `json:"category"`
	Description string     `json:"description"`
	AddedAt     time.Time  `json:"added_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

// BlacklistSource 黑名单远程数据源
type BlacklistSource struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	Format  string `json:"format"` // "plain_text" 或 "csv"
	Enabled bool   `json:"enabled"`
}

// NewBlacklistManager 创建黑名单管理器
func NewBlacklistManager(engine *AlertEngine) *BlacklistManager {
	mgr := &BlacklistManager{
		ipBlacklist: make(map[string]*BlacklistEntry),
		sources: []BlacklistSource{
			{Name: "Abuse.ch Feodo Tracker", URL: "https://feodotracker.abuse.ch/downloads/ipblocklist.txt", Format: "plain_text", Enabled: false},
			{Name: "Emerging Threats", URL: "https://rules.emergingthreats.net/blockrules/compromised-ips.txt", Format: "plain_text", Enabled: false},
		},
		engine: engine,
		stopCh: make(chan struct{}),
	}
	mgr.loadBuiltinBlacklist()
	return mgr
}

// Start 启动定期更新 goroutine（每 6 小时从远程 URL 更新黑名单）
func (m *BlacklistManager) Start() {
	go func() {
		// 启动时立即尝试一次更新
		m.updateAllSources()

		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()

		log.Println("[BlacklistManager] started, updating every 6 hours")
		for {
			select {
			case <-ticker.C:
				m.updateAllSources()
			case <-m.stopCh:
				log.Println("[BlacklistManager] stopped")
				return
			}
		}
	}()
}

// Stop 停止黑名单管理器
func (m *BlacklistManager) Stop() {
	close(m.stopCh)
}

// CheckIP 检查 IP 是否在黑名单中，命中时自动触发告警
func (m *BlacklistManager) CheckIP(ip string) *BlacklistEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 1. 精确匹配
	if entry, ok := m.ipBlacklist[ip]; ok {
		// 检查是否过期
		if entry.ExpiresAt != nil && time.Now().After(*entry.ExpiresAt) {
			return nil
		}
		m.triggerBlacklistAlert(ip, entry)
		return entry
	}

	// 2. CIDR 范围匹配
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return nil
	}
	for _, cidr := range m.cidrList {
		if cidr.network.Contains(parsedIP) {
			if cidr.entry.ExpiresAt != nil && time.Now().After(*cidr.entry.ExpiresAt) {
				continue
			}
			m.triggerBlacklistAlert(ip, cidr.entry)
			return cidr.entry
		}
	}

	return nil
}

// triggerBlacklistAlert 触发黑名单告警（内部使用，调用时需已持有读锁）
func (m *BlacklistManager) triggerBlacklistAlert(ip string, entry *BlacklistEntry) {
	if m.engine == nil {
		return
	}
	alert := Alert{
		Type:        AlertTypeHost,
		Severity:    SeverityCritical,
		Status:      StatusTriggered,
		RuleID:      "blacklist_hit",
		Title:       fmt.Sprintf("[critical] Blacklisted IP detected: %s", ip),
		Description: fmt.Sprintf("IP %s matched blacklist entry from source '%s', category: %s. %s", ip, entry.Source, entry.Category, entry.Description),
		EntityType:  "host",
		EntityID:    ip,
		TriggeredAt: time.Now(),
	}
	if err := m.engine.TriggerAlert(alert); err != nil {
		log.Printf("[BlacklistManager] failed to trigger alert for IP %s: %v", ip, err)
	}
}

// AddManualEntry 手动添加黑名单条目
func (m *BlacklistManager) AddManualEntry(entry BlacklistEntry) {
	if entry.AddedAt.IsZero() {
		entry.AddedAt = time.Now()
	}
	if entry.Source == "" {
		entry.Source = "manual"
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 判断是 CIDR 还是单 IP
	if strings.Contains(entry.IP, "/") {
		_, network, err := net.ParseCIDR(entry.IP)
		if err != nil {
			log.Printf("[BlacklistManager] invalid CIDR: %s", entry.IP)
			return
		}
		m.cidrList = append(m.cidrList, &cidrEntry{network: network, entry: &entry})
	} else {
		m.ipBlacklist[entry.IP] = &entry
	}
}

// RemoveEntry 移除黑名单条目
func (m *BlacklistManager) RemoveEntry(ip string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.ipBlacklist, ip)

	// 同时检查 CIDR 列表
	filtered := m.cidrList[:0]
	for _, c := range m.cidrList {
		if c.entry.IP != ip {
			filtered = append(filtered, c)
		}
	}
	m.cidrList = filtered
}

// GetEntries 获取所有黑名单条目
func (m *BlacklistManager) GetEntries() []BlacklistEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entries := make([]BlacklistEntry, 0, len(m.ipBlacklist)+len(m.cidrList))
	for _, e := range m.ipBlacklist {
		entries = append(entries, *e)
	}
	for _, c := range m.cidrList {
		entries = append(entries, *c.entry)
	}
	return entries
}

// GetSources 获取所有黑名单源配置
func (m *BlacklistManager) GetSources() []BlacklistSource {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]BlacklistSource, len(m.sources))
	copy(result, m.sources)
	return result
}

// updateAllSources 更新所有已启用的远程源
func (m *BlacklistManager) updateAllSources() {
	m.mu.RLock()
	sources := make([]BlacklistSource, len(m.sources))
	copy(sources, m.sources)
	m.mu.RUnlock()

	for _, src := range sources {
		if !src.Enabled {
			continue
		}
		if err := m.updateFromSource(src); err != nil {
			log.Printf("[BlacklistManager] failed to update from %s: %v", src.Name, err)
		} else {
			log.Printf("[BlacklistManager] updated from %s", src.Name)
		}
	}
}

// updateFromSource 从远程源更新黑名单
func (m *BlacklistManager) updateFromSource(source BlacklistSource) error {
	client := &http.Client{Timeout: 30 * time.Second}

	resp, err := client.Get(source.URL)
	if err != nil {
		return fmt.Errorf("HTTP GET %s: %w", source.URL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, source.URL)
	}

	scanner := bufio.NewScanner(resp.Body)
	now := time.Now()
	count := 0

	m.mu.Lock()
	defer m.mu.Unlock()

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// 跳过空行和注释
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}

		// CSV 格式取第一列
		if source.Format == "csv" {
			parts := strings.SplitN(line, ",", 2)
			line = strings.TrimSpace(parts[0])
		}

		// 解析 IP 或 CIDR
		if strings.Contains(line, "/") {
			_, network, err := net.ParseCIDR(line)
			if err != nil {
				continue
			}
			entry := &BlacklistEntry{
				IP:       line,
				Source:   source.Name,
				Category: "threat_feed",
				AddedAt:  now,
			}
			m.cidrList = append(m.cidrList, &cidrEntry{network: network, entry: entry})
			count++
		} else {
			ip := net.ParseIP(line)
			if ip == nil {
				continue
			}
			ipStr := ip.String()
			if _, exists := m.ipBlacklist[ipStr]; !exists {
				m.ipBlacklist[ipStr] = &BlacklistEntry{
					IP:       ipStr,
					Source:   source.Name,
					Category: "threat_feed",
					AddedAt:  now,
				}
				count++
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	log.Printf("[BlacklistManager] loaded %d entries from %s", count, source.Name)
	return nil
}

// loadBuiltinBlacklist 加载内置已知恶意 IP
func (m *BlacklistManager) loadBuiltinBlacklist() {
	builtinIPs := []BlacklistEntry{
		{IP: "185.220.101.1", Source: "builtin", Category: "tor_exit", Description: "Known Tor exit node"},
		{IP: "185.220.101.34", Source: "builtin", Category: "tor_exit", Description: "Known Tor exit node"},
		{IP: "89.234.157.254", Source: "builtin", Category: "tor_exit", Description: "Known Tor exit node"},
		{IP: "62.102.148.68", Source: "builtin", Category: "scanner", Description: "Known malicious scanner"},
		{IP: "80.82.77.139", Source: "builtin", Category: "scanner", Description: "Known malicious scanner (Censys)"},
		{IP: "80.82.77.33", Source: "builtin", Category: "scanner", Description: "Known malicious scanner"},
		{IP: "71.6.135.131", Source: "builtin", Category: "scanner", Description: "Known malicious scanner (BinaryEdge)"},
		{IP: "71.6.146.185", Source: "builtin", Category: "scanner", Description: "Known malicious scanner (BinaryEdge)"},
		{IP: "198.108.67.16", Source: "builtin", Category: "scanner", Description: "Known malicious scanner (Censys)"},
		{IP: "167.94.138.0", Source: "builtin", Category: "scanner", Description: "Known malicious scanner (Censys)"},
		{IP: "45.148.10.240", Source: "builtin", Category: "botnet", Description: "Known botnet C2 server"},
		{IP: "194.26.192.64", Source: "builtin", Category: "botnet", Description: "Known botnet C2 server"},
		{IP: "5.188.206.14", Source: "builtin", Category: "brute_force", Description: "Known brute-force attacker"},
		{IP: "45.155.205.233", Source: "builtin", Category: "malware", Description: "Known malware distribution host"},
		{IP: "193.42.33.10", Source: "builtin", Category: "malware", Description: "Known malware C2 server"},
		{IP: "91.240.118.172", Source: "builtin", Category: "spam", Description: "Known spam source"},
		{IP: "185.56.83.83", Source: "builtin", Category: "spam", Description: "Known spam source"},
		{IP: "104.168.145.186", Source: "builtin", Category: "phishing", Description: "Known phishing server"},
		{IP: "192.42.116.16", Source: "builtin", Category: "tor_exit", Description: "Known Tor exit node"},
		{IP: "176.10.104.240", Source: "builtin", Category: "tor_exit", Description: "Known Tor exit node"},
	}

	now := time.Now()
	for i := range builtinIPs {
		builtinIPs[i].AddedAt = now
		m.ipBlacklist[builtinIPs[i].IP] = &builtinIPs[i]
	}

	log.Printf("[BlacklistManager] loaded %d builtin blacklist entries", len(builtinIPs))
}
