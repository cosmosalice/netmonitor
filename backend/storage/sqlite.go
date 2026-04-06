package storage

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "modernc.org/sqlite"
)

// Database wraps SQLite database connection
type Database struct {
	db *sql.DB
}

// NewDatabase creates a new database connection
func NewDatabase(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	d := &Database{db: db}

	// Initialize schema
	if err := d.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	log.Println("Database initialized successfully")
	return d, nil
}

// Close closes the database connection
func (d *Database) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

// GetDB returns the underlying database connection
func (d *Database) GetDB() *sql.DB {
	return d.db
}

// initSchema creates database tables and indexes
func (d *Database) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS hosts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		ip TEXT UNIQUE NOT NULL,
		mac TEXT,
		hostname TEXT,
		first_seen TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		last_seen TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		bytes_sent INTEGER DEFAULT 0,
		bytes_recv INTEGER DEFAULT 0,
		packets_sent INTEGER DEFAULT 0,
		packets_recv INTEGER DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS flows (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		flow_id TEXT UNIQUE NOT NULL,
		src_ip TEXT,
		dst_ip TEXT,
		src_port INTEGER,
		dst_port INTEGER,
		protocol TEXT,
		vlan_id INTEGER DEFAULT 0,
		l7_protocol TEXT,
		l7_category TEXT,
		bytes_sent INTEGER DEFAULT 0,
		bytes_recv INTEGER DEFAULT 0,
		packets_sent INTEGER DEFAULT 0,
		packets_recv INTEGER DEFAULT 0,
		start_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		end_time TIMESTAMP,
		is_active BOOLEAN DEFAULT 1,
		rtt_ms REAL DEFAULT 0,
		min_rtt_ms REAL DEFAULT 0,
		max_rtt_ms REAL DEFAULT 0,
		retransmissions INTEGER DEFAULT 0,
		out_of_order INTEGER DEFAULT 0,
		packet_loss INTEGER DEFAULT 0,
		avg_window_size INTEGER DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS timeseries (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TIMESTAMP NOT NULL,
		metric_type TEXT NOT NULL,
		metric_key TEXT,
		value REAL NOT NULL
	);

	CREATE TABLE IF NOT EXISTS protocol_stats (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		protocol TEXT,
		category TEXT,
		bytes INTEGER DEFAULT 0,
		packets INTEGER DEFAULT 0,
		flow_count INTEGER DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS timeseries_5min (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TIMESTAMP NOT NULL,
		metric_type TEXT NOT NULL,
		metric_key TEXT,
		avg_value REAL NOT NULL,
		max_value REAL NOT NULL,
		min_value REAL NOT NULL,
		p95_value REAL NOT NULL,
		sample_count INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS timeseries_1h (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TIMESTAMP NOT NULL,
		metric_type TEXT NOT NULL,
		metric_key TEXT,
		avg_value REAL NOT NULL,
		max_value REAL NOT NULL,
		min_value REAL NOT NULL,
		p95_value REAL NOT NULL,
		sample_count INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS timeseries_1d (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp TIMESTAMP NOT NULL,
		metric_type TEXT NOT NULL,
		metric_key TEXT,
		avg_value REAL NOT NULL,
		max_value REAL NOT NULL,
		min_value REAL NOT NULL,
		p95_value REAL NOT NULL,
		sample_count INTEGER NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_timeseries_timestamp ON timeseries(timestamp);
	CREATE INDEX IF NOT EXISTS idx_timeseries_type ON timeseries(metric_type);
	CREATE INDEX IF NOT EXISTS idx_timeseries_type_key_ts ON timeseries(metric_type, metric_key, timestamp);
	CREATE INDEX IF NOT EXISTS idx_ts5min_type_key_ts ON timeseries_5min(metric_type, metric_key, timestamp);
	CREATE INDEX IF NOT EXISTS idx_ts1h_type_key_ts ON timeseries_1h(metric_type, metric_key, timestamp);
	CREATE INDEX IF NOT EXISTS idx_ts1d_type_key_ts ON timeseries_1d(metric_type, metric_key, timestamp);
	CREATE INDEX IF NOT EXISTS idx_flows_flow_id ON flows(flow_id);
	CREATE INDEX IF NOT EXISTS idx_flows_active ON flows(is_active);
	CREATE INDEX IF NOT EXISTS idx_hosts_ip ON hosts(ip);
	CREATE INDEX IF NOT EXISTS idx_flows_start_time ON flows(start_time);
	CREATE INDEX IF NOT EXISTS idx_flows_src_ip ON flows(src_ip);
	CREATE INDEX IF NOT EXISTS idx_flows_dst_ip ON flows(dst_ip);
	CREATE INDEX IF NOT EXISTS idx_flows_l7_protocol ON flows(l7_protocol);

	CREATE TABLE IF NOT EXISTS alerts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		type TEXT NOT NULL,
		severity TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'triggered',
		rule_id TEXT,
		title TEXT NOT NULL,
		description TEXT,
		entity_type TEXT,
		entity_id TEXT,
		metadata TEXT,
		triggered_at DATETIME NOT NULL,
		acked_at DATETIME,
		resolved_at DATETIME,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS alert_rules (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		description TEXT,
		type TEXT NOT NULL,
		severity TEXT NOT NULL DEFAULT 'warning',
		enabled INTEGER NOT NULL DEFAULT 1,
		condition_json TEXT NOT NULL,
		cooldown_sec INTEGER NOT NULL DEFAULT 300,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_alerts_type_severity_status ON alerts(type, severity, status);
	CREATE INDEX IF NOT EXISTS idx_alerts_triggered_at ON alerts(triggered_at);
	CREATE INDEX IF NOT EXISTS idx_alerts_entity_id ON alerts(entity_id);

	CREATE TABLE IF NOT EXISTS notification_endpoints (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		type TEXT NOT NULL,
		enabled INTEGER NOT NULL DEFAULT 1,
		config_json TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS reports (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		type TEXT NOT NULL,
		period TEXT NOT NULL,
		generated_at DATETIME NOT NULL,
		file_path TEXT NOT NULL,
		file_size INTEGER DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS report_configs (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		type TEXT NOT NULL,
		enabled INTEGER NOT NULL DEFAULT 1,
		output_dir TEXT DEFAULT 'reports',
		last_gen_time DATETIME,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_reports_type_period ON reports(type, period);
	CREATE INDEX IF NOT EXISTS idx_reports_generated_at ON reports(generated_at);

	-- SNMP devices table
	CREATE TABLE IF NOT EXISTS snmp_devices (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		ip TEXT NOT NULL,
		community TEXT DEFAULT 'public',
		version TEXT DEFAULT 'v2c',
		port INTEGER DEFAULT 161,
		enabled INTEGER DEFAULT 1,
		status TEXT DEFAULT 'unknown',
		last_polled DATETIME,
		sys_descr TEXT,
		sys_name TEXT,
		interfaces TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_snmp_devices_ip ON snmp_devices(ip);
	CREATE INDEX IF NOT EXISTS idx_snmp_devices_status ON snmp_devices(status);

	-- SNMP interfaces table
	CREATE TABLE IF NOT EXISTS snmp_interfaces (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		device_id TEXT NOT NULL,
		interface_index INTEGER NOT NULL,
		name TEXT,
		status TEXT,
		in_octets INTEGER DEFAULT 0,
		out_octets INTEGER DEFAULT 0,
		speed INTEGER DEFAULT 0,
		updated_at DATETIME,
		FOREIGN KEY (device_id) REFERENCES snmp_devices(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_snmp_interfaces_device ON snmp_interfaces(device_id);

	-- Monitor targets table
	CREATE TABLE IF NOT EXISTS monitor_targets (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		type TEXT NOT NULL,
		host TEXT,
		port INTEGER,
		url TEXT,
		interval_sec INTEGER DEFAULT 60,
		timeout_ms INTEGER DEFAULT 5000,
		enabled INTEGER DEFAULT 1,
		status TEXT DEFAULT 'unknown',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_monitor_targets_type ON monitor_targets(type);
	CREATE INDEX IF NOT EXISTS idx_monitor_targets_status ON monitor_targets(status);

	-- Monitor results table
	CREATE TABLE IF NOT EXISTS monitor_results (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		target_id TEXT NOT NULL,
		timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		status TEXT NOT NULL,
		latency_ms REAL DEFAULT 0,
		error TEXT,
		FOREIGN KEY (target_id) REFERENCES monitor_targets(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_monitor_results_target ON monitor_results(target_id);
	CREATE INDEX IF NOT EXISTS idx_monitor_results_timestamp ON monitor_results(timestamp);

	-- Devices table for MAC address tracking
	CREATE TABLE IF NOT EXISTS devices (
		mac TEXT PRIMARY KEY,
		name TEXT DEFAULT '',
		vendor TEXT DEFAULT '',
		ips TEXT DEFAULT '[]',
		first_seen DATETIME,
		last_seen DATETIME,
		bytes_sent INTEGER DEFAULT 0,
		bytes_recv INTEGER DEFAULT 0,
		flow_count INTEGER DEFAULT 0,
		device_type TEXT DEFAULT 'unknown'
	);

	CREATE INDEX IF NOT EXISTS idx_devices_vendor ON devices(vendor);
	CREATE INDEX IF NOT EXISTS idx_devices_device_type ON devices(device_type);
	CREATE INDEX IF NOT EXISTS idx_devices_last_seen ON devices(last_seen);

	-- Users table for authentication
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password TEXT NOT NULL,
		role TEXT DEFAULT 'viewer',
		email TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_login DATETIME
	);

	CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
	CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);

	-- Integrations table
	CREATE TABLE IF NOT EXISTS integrations (
		id TEXT PRIMARY KEY,
		type TEXT NOT NULL,
		config TEXT DEFAULT '{}',
		enabled INTEGER DEFAULT 0,
		last_error TEXT DEFAULT '',
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Dashboards table
	CREATE TABLE IF NOT EXISTS dashboards (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		config TEXT DEFAULT '{}',
		is_default INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_integrations_type ON integrations(type);
	CREATE INDEX IF NOT EXISTS idx_dashboards_is_default ON dashboards(is_default);
	`

	_, err := d.db.Exec(schema)
	if err != nil {
		return err
	}

	// Migrate: add TCP metrics columns to existing flows table (idempotent)
	migrations := []string{
		"ALTER TABLE flows ADD COLUMN rtt_ms REAL DEFAULT 0",
		"ALTER TABLE flows ADD COLUMN min_rtt_ms REAL DEFAULT 0",
		"ALTER TABLE flows ADD COLUMN max_rtt_ms REAL DEFAULT 0",
		"ALTER TABLE flows ADD COLUMN retransmissions INTEGER DEFAULT 0",
		"ALTER TABLE flows ADD COLUMN out_of_order INTEGER DEFAULT 0",
		"ALTER TABLE flows ADD COLUMN packet_loss INTEGER DEFAULT 0",
		"ALTER TABLE flows ADD COLUMN avg_window_size INTEGER DEFAULT 0",
		// GeoIP columns on hosts table
		"ALTER TABLE hosts ADD COLUMN country TEXT DEFAULT ''",
		"ALTER TABLE hosts ADD COLUMN city TEXT DEFAULT ''",
		"ALTER TABLE hosts ADD COLUMN latitude REAL DEFAULT 0",
		"ALTER TABLE hosts ADD COLUMN longitude REAL DEFAULT 0",
		"ALTER TABLE hosts ADD COLUMN asn INTEGER DEFAULT 0",
		"ALTER TABLE hosts ADD COLUMN as_org TEXT DEFAULT ''",
		// VLAN ID column on flows table
		"ALTER TABLE flows ADD COLUMN vlan_id INTEGER DEFAULT 0",
	}
	for _, m := range migrations {
		// Ignore errors (column already exists)
		d.db.Exec(m)
	}

	return nil
}
