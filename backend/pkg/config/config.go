package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Config holds application configuration
type Config struct {
	mu       sync.RWMutex
	FilePath string `json:"-"`

	// Capture settings
	Interface   string `json:"interface"`
	BPFFilter   string `json:"bpf_filter"`
	PromiscMode bool   `json:"promisc_mode"`
	Snaplen     int    `json:"snaplen"`

	// Database settings
	DatabasePath string `json:"database_path"`

	// Retention settings
	RetentionHours int `json:"retention_hours"`

	// API settings
	APIPort int `json:"api_port"`

	// UI settings
	Theme string `json:"theme"`

	// GeoIP settings
	GeoIPCityDB string `json:"geoip_city_db"`
	GeoIPASNDB  string `json:"geoip_asn_db"`
}

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	return &Config{
		Interface:      "",
		BPFFilter:      "",
		PromiscMode:    true,
		Snaplen:        65536,
		DatabasePath:   "netmonitor.db",
		RetentionHours: 720, // 30 days
		APIPort:        8080,
		Theme:          "light",
		GeoIPCityDB:    "GeoLite2-City.mmdb",
		GeoIPASNDB:     "GeoLite2-ASN.mmdb",
	}
}

// LoadConfig loads configuration from file
func LoadConfig(path string) (*Config, error) {
	config := DefaultConfig()
	config.FilePath = path

	// Create default config if not exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := config.Save(); err != nil {
			return nil, err
		}
		return config, nil
	}

	// Read config file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, config); err != nil {
		return nil, err
	}

	return config, nil
}

// Save saves configuration to file
func (c *Config) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.FilePath == "" {
		c.FilePath = "config.json"
	}

	// Ensure directory exists
	dir := filepath.Dir(c.FilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(c.FilePath, data, 0644)
}

// Update updates configuration and saves
func (c *Config) Update(updates map[string]interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if iface, ok := updates["interface"].(string); ok {
		c.Interface = iface
	}
	if filter, ok := updates["bpf_filter"].(string); ok {
		c.BPFFilter = filter
	}
	if promisc, ok := updates["promisc_mode"].(bool); ok {
		c.PromiscMode = promisc
	}
	if dbPath, ok := updates["database_path"].(string); ok {
		c.DatabasePath = dbPath
	}
	if retention, ok := updates["retention_hours"].(float64); ok {
		c.RetentionHours = int(retention)
	}
	if apiPort, ok := updates["api_port"].(float64); ok {
		c.APIPort = int(apiPort)
	}
	if theme, ok := updates["theme"].(string); ok {
		c.Theme = theme
	}

	// Save to file
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(c.FilePath, data, 0644)
}

// Get returns a copy of current configuration
func (c *Config) Get() Config {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return *c
}
