package alerts

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// ThreatIntelManager 威胁情报管理器
type ThreatIntelManager struct {
	mu           sync.RWMutex
	maliciousJA3 map[string]*JA3Entry
	engine       *AlertEngine
}

// JA3Entry JA3 指纹条目
type JA3Entry struct {
	Hash          string `json:"hash"`
	Description   string `json:"description"`
	MalwareFamily string `json:"malware_family,omitempty"`
	Source        string `json:"source"`
}

// NewThreatIntelManager 创建威胁情报管理器
func NewThreatIntelManager(engine *AlertEngine) *ThreatIntelManager {
	mgr := &ThreatIntelManager{
		maliciousJA3: make(map[string]*JA3Entry),
		engine:       engine,
	}
	mgr.LoadBuiltinJA3()
	return mgr
}

// CheckJA3 检查 JA3 指纹是否为已知恶意，命中时自动触发告警
func (t *ThreatIntelManager) CheckJA3(hash string) *JA3Entry {
	if hash == "" {
		return nil
	}

	t.mu.RLock()
	entry, ok := t.maliciousJA3[hash]
	t.mu.RUnlock()

	if !ok {
		return nil
	}

	// 触发告警
	if t.engine != nil {
		alert := Alert{
			Type:        AlertTypeFlow,
			Severity:    SeverityCritical,
			Status:      StatusTriggered,
			RuleID:      "malicious_ja3",
			Title:       fmt.Sprintf("[critical] Malicious JA3 fingerprint detected: %s", entry.MalwareFamily),
			Description: fmt.Sprintf("JA3 hash %s matched known malware: %s (%s)", hash, entry.Description, entry.MalwareFamily),
			EntityType:  "ja3",
			EntityID:    hash,
			TriggeredAt: time.Now(),
		}
		if err := t.engine.TriggerAlert(alert); err != nil {
			log.Printf("[ThreatIntelManager] failed to trigger alert for JA3 %s: %v", hash, err)
		}
	}

	return entry
}

// AddJA3Entry 手动添加 JA3 条目
func (t *ThreatIntelManager) AddJA3Entry(entry JA3Entry) {
	if entry.Source == "" {
		entry.Source = "manual"
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.maliciousJA3[entry.Hash] = &entry
}

// GetJA3Entries 获取所有 JA3 条目
func (t *ThreatIntelManager) GetJA3Entries() []JA3Entry {
	t.mu.RLock()
	defer t.mu.RUnlock()

	entries := make([]JA3Entry, 0, len(t.maliciousJA3))
	for _, e := range t.maliciousJA3 {
		entries = append(entries, *e)
	}
	return entries
}

// LoadBuiltinJA3 加载内置的已知恶意 JA3 指纹
func (t *ThreatIntelManager) LoadBuiltinJA3() {
	builtinEntries := []JA3Entry{
		{Hash: "51c64c77e60f3980eea90869b68c58a8", Description: "Tofsee Botnet", MalwareFamily: "Tofsee", Source: "builtin"},
		{Hash: "ec74a5c51106f0419184d0dd08fb05bc", Description: "Dridex Trojan", MalwareFamily: "Dridex", Source: "builtin"},
		{Hash: "6734f37431670b3ab4292b8f60f29984", Description: "TrickBot", MalwareFamily: "TrickBot", Source: "builtin"},
		{Hash: "e7d705a3286e19ea42f587b344ee6865", Description: "Emotet", MalwareFamily: "Emotet", Source: "builtin"},
		{Hash: "4d7a28d6f2263ed61de88ca66eb011e3", Description: "Emotet epoch 4", MalwareFamily: "Emotet", Source: "builtin"},
		{Hash: "c12f54a3f91dc7bafd92b1a4f5d08bdc", Description: "IcedID Banking Trojan", MalwareFamily: "IcedID", Source: "builtin"},
		{Hash: "37f463bf4616ecd445d4a1937da06e19", Description: "Gozi ISFB", MalwareFamily: "Gozi", Source: "builtin"},
		{Hash: "3b5074b1b5d032e5620f69f9f700ff0e", Description: "AsyncRAT", MalwareFamily: "AsyncRAT", Source: "builtin"},
		{Hash: "72a589da586844d7f0818ce684948eea", Description: "Metasploit", MalwareFamily: "Metasploit", Source: "builtin"},
		{Hash: "a0e9f5d64349fb13191bc781f81f42e1", Description: "CobaltStrike", MalwareFamily: "CobaltStrike", Source: "builtin"},
		{Hash: "72a589da586844d7f0818ce684948eeb", Description: "CobaltStrike HTTPS beacon", MalwareFamily: "CobaltStrike", Source: "builtin"},
		{Hash: "b20b44b18b853f3925171d072a5e9714", Description: "Cobalt Strike 4.x", MalwareFamily: "CobaltStrike", Source: "builtin"},
		{Hash: "f436b9416f37d134cadd04886327d3e8", Description: "QakBot", MalwareFamily: "QakBot", Source: "builtin"},
		{Hash: "c5b78899da36e4121b5765d1a4b32212", Description: "QakBot HTTPS", MalwareFamily: "QakBot", Source: "builtin"},
		{Hash: "2d110faa8d2c4aab9b4cf38b2e60e26b", Description: "BazarLoader", MalwareFamily: "BazarLoader", Source: "builtin"},
		{Hash: "8a8a05b29bf37bed79fede8d44cb5050", Description: "BumbleBee Loader", MalwareFamily: "BumbleBee", Source: "builtin"},
		{Hash: "7dcce5b76c8b17472d024758970a406b", Description: "Hancitor / Chanitor", MalwareFamily: "Hancitor", Source: "builtin"},
		{Hash: "3e9b20acd95937edfb6aea37e3e1f80a", Description: "Ursnif / Gozi", MalwareFamily: "Ursnif", Source: "builtin"},
		{Hash: "535aca3d99fc247509cd50933cd71d37", Description: "SocGholish FakeUpdate", MalwareFamily: "SocGholish", Source: "builtin"},
		{Hash: "bd0bf25947d4a37404f0424edf4db9ad", Description: "Agent Tesla", MalwareFamily: "AgentTesla", Source: "builtin"},
		{Hash: "fc54e0d16d9764783542f0146a98b300", Description: "Raccoon Stealer", MalwareFamily: "RaccoonStealer", Source: "builtin"},
		{Hash: "399840ee517ef43e7e68fe7a0dba530a", Description: "RedLine Stealer", MalwareFamily: "RedLine", Source: "builtin"},
		{Hash: "cd08e31494816f6cf8a8f6e8002f55e0", Description: "Formbook / XLoader", MalwareFamily: "Formbook", Source: "builtin"},
		{Hash: "0518561bed1ac0aca83917e63dab8f46", Description: "Sliver C2 framework", MalwareFamily: "Sliver", Source: "builtin"},
		{Hash: "d3c3ddf2ad7c4c77a4df3900031b0735", Description: "Havoc C2 framework", MalwareFamily: "Havoc", Source: "builtin"},
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	for i := range builtinEntries {
		t.maliciousJA3[builtinEntries[i].Hash] = &builtinEntries[i]
	}

	log.Printf("[ThreatIntelManager] loaded %d builtin malicious JA3 fingerprints", len(builtinEntries))
}
