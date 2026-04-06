package snmp

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/netmonitor/backend/storage"
)

// SNMPInterface represents a network interface on an SNMP device
type SNMPInterface struct {
	Index     int    `json:"index"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	InOctets  uint64 `json:"in_octets"`
	OutOctets uint64 `json:"out_octets"`
	Speed     uint64 `json:"speed"`
}

// SNMPDevice represents an SNMP-enabled network device
type SNMPDevice struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	IP         string          `json:"ip"`
	Community  string          `json:"community"`
	Version    string          `json:"version"` // "v1", "v2c"
	Port       uint16          `json:"port"`
	Enabled    bool            `json:"enabled"`
	Status     string          `json:"status"` // "up", "down", "unknown"
	LastPolled *time.Time      `json:"last_polled"`
	SysDescr   string          `json:"sys_descr"`
	SysName    string          `json:"sys_name"`
	Interfaces []SNMPInterface `json:"interfaces"`
}

// SNMPManager manages SNMP devices and polling
type SNMPManager struct {
	mu       sync.RWMutex
	devices  map[string]*SNMPDevice
	stopCh   chan struct{}
	db       *storage.Database
	interval time.Duration
}

// NewSNMPManager creates a new SNMP manager
func NewSNMPManager(db *storage.Database) *SNMPManager {
	return &SNMPManager{
		devices:  make(map[string]*SNMPDevice),
		stopCh:   make(chan struct{}),
		db:       db,
		interval: 60 * time.Second,
	}
}

// Start begins the periodic polling goroutine
func (m *SNMPManager) Start() error {
	// Load existing devices from database
	if err := m.loadDevices(); err != nil {
		log.Printf("Failed to load SNMP devices: %v", err)
	}

	// Start polling loop
	go m.pollingLoop()
	log.Println("SNMP manager started")
	return nil
}

// Stop stops the polling goroutine
func (m *SNMPManager) Stop() {
	close(m.stopCh)
}

// AddDevice adds a new SNMP device
func (m *SNMPManager) AddDevice(device SNMPDevice) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Generate ID if not provided
	if device.ID == "" {
		device.ID = fmt.Sprintf("snmp_%d", time.Now().UnixNano())
	}

	// Set defaults
	if device.Port == 0 {
		device.Port = 161
	}
	if device.Version == "" {
		device.Version = "v2c"
	}
	if device.Community == "" {
		device.Community = "public"
	}
	device.Status = "unknown"

	// Save to database
	if err := m.saveDeviceToDB(&device); err != nil {
		return fmt.Errorf("failed to save device: %w", err)
	}

	m.devices[device.ID] = &device
	log.Printf("SNMP device added: %s (%s)", device.Name, device.IP)

	// Poll immediately
	go m.PollDevice(&device)

	return nil
}

// RemoveDevice removes an SNMP device
func (m *SNMPManager) RemoveDevice(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.devices, id)

	// Remove from database
	if m.db != nil {
		db := m.db.GetDB()
		_, err := db.Exec("DELETE FROM snmp_devices WHERE id = ?", id)
		if err != nil {
			log.Printf("Failed to delete SNMP device from DB: %v", err)
		}
		_, _ = db.Exec("DELETE FROM snmp_interfaces WHERE device_id = ?", id)
	}

	log.Printf("SNMP device removed: %s", id)
}

// GetDevices returns all SNMP devices
func (m *SNMPManager) GetDevices() []SNMPDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]SNMPDevice, 0, len(m.devices))
	for _, d := range m.devices {
		// Load interfaces from DB for each device
		deviceCopy := *d
		if m.db != nil {
			interfaces, err := m.loadInterfacesFromDB(d.ID)
			if err == nil {
				deviceCopy.Interfaces = interfaces
			}
		}
		devices = append(devices, deviceCopy)
	}
	return devices
}

// GetDevice returns a specific device by ID
func (m *SNMPManager) GetDevice(id string) *SNMPDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()

	d, ok := m.devices[id]
	if !ok {
		return nil
	}

	deviceCopy := *d
	if m.db != nil {
		interfaces, err := m.loadInterfacesFromDB(d.ID)
		if err == nil {
			deviceCopy.Interfaces = interfaces
		}
	}
	return &deviceCopy
}

// PollDevice polls a single SNMP device
func (m *SNMPManager) PollDevice(device *SNMPDevice) error {
	g := &gosnmp.GoSNMP{
		Target:    device.IP,
		Port:      device.Port,
		Community: device.Community,
		Version:   gosnmp.Version2c,
		Timeout:   5 * time.Second,
		Retries:   2,
	}

	if device.Version == "v1" {
		g.Version = gosnmp.Version1
	}

	// Try to connect
	if err := g.Connect(); err != nil {
		device.Status = "down"
		m.updateDeviceStatus(device)
		return fmt.Errorf("failed to connect to %s: %w", device.IP, err)
	}
	defer g.Conn.Close()

	// Query system OIDs
	sysOIDs := []string{
		".1.3.6.1.2.1.1.1.0", // sysDescr
		".1.3.6.1.2.1.1.5.0", // sysName
	}

	result, err := g.Get(sysOIDs)
	if err != nil {
		device.Status = "down"
		m.updateDeviceStatus(device)
		return fmt.Errorf("SNMP get failed for %s: %w", device.IP, err)
	}

	// Parse results
	for _, v := range result.Variables {
		switch v.Name {
		case ".1.3.6.1.2.1.1.1.0":
			if v.Type == gosnmp.OctetString {
				device.SysDescr = string(v.Value.([]byte))
			}
		case ".1.3.6.1.2.1.1.5.0":
			if v.Type == gosnmp.OctetString {
				device.SysName = string(v.Value.([]byte))
			}
		}
	}

	// Query interface table
	interfaces, err := m.pollInterfaces(g)
	if err != nil {
		log.Printf("Failed to poll interfaces for %s: %v", device.IP, err)
	} else {
		device.Interfaces = interfaces
		// Save interfaces to DB
		if m.db != nil {
			m.saveInterfacesToDB(device.ID, interfaces)
		}
	}

	// Update status
	now := time.Now()
	device.LastPolled = &now
	device.Status = "up"
	m.updateDeviceStatus(device)

	return nil
}

// pollInterfaces polls the interface table from a device
func (m *SNMPManager) pollInterfaces(g *gosnmp.GoSNMP) ([]SNMPInterface, error) {
	interfaces := make(map[int]*SNMPInterface)

	// Walk ifDescr (interface descriptions)
	err := g.BulkWalk(".1.3.6.1.2.1.2.2.1.2", func(pdu gosnmp.SnmpPDU) error {
		idx := extractIndex(pdu.Name)
		if idx > 0 {
			if _, ok := interfaces[idx]; !ok {
				interfaces[idx] = &SNMPInterface{Index: idx}
			}
			if pdu.Type == gosnmp.OctetString {
				interfaces[idx].Name = string(pdu.Value.([]byte))
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Walk ifOperStatus
	_ = g.BulkWalk(".1.3.6.1.2.1.2.2.1.8", func(pdu gosnmp.SnmpPDU) error {
		idx := extractIndex(pdu.Name)
		if idx > 0 && interfaces[idx] != nil {
			if pdu.Type == gosnmp.Integer {
				status := pdu.Value.(int)
				switch status {
				case 1:
					interfaces[idx].Status = "up"
				case 2:
					interfaces[idx].Status = "down"
				default:
					interfaces[idx].Status = "unknown"
				}
			}
		}
		return nil
	})

	// Walk ifInOctets
	_ = g.BulkWalk(".1.3.6.1.2.1.2.2.1.10", func(pdu gosnmp.SnmpPDU) error {
		idx := extractIndex(pdu.Name)
		if idx > 0 && interfaces[idx] != nil {
			if pdu.Type == gosnmp.Counter32 {
				interfaces[idx].InOctets = uint64(pdu.Value.(uint32))
			}
		}
		return nil
	})

	// Walk ifOutOctets
	_ = g.BulkWalk(".1.3.6.1.2.1.2.2.1.16", func(pdu gosnmp.SnmpPDU) error {
		idx := extractIndex(pdu.Name)
		if idx > 0 && interfaces[idx] != nil {
			if pdu.Type == gosnmp.Counter32 {
				interfaces[idx].OutOctets = uint64(pdu.Value.(uint32))
			}
		}
		return nil
	})

	// Walk ifSpeed
	_ = g.BulkWalk(".1.3.6.1.2.1.2.2.1.5", func(pdu gosnmp.SnmpPDU) error {
		idx := extractIndex(pdu.Name)
		if idx > 0 && interfaces[idx] != nil {
			if pdu.Type == gosnmp.Gauge32 {
				interfaces[idx].Speed = uint64(pdu.Value.(uint32))
			}
		}
		return nil
	})

	// Convert map to slice
	result := make([]SNMPInterface, 0, len(interfaces))
	for _, iface := range interfaces {
		result = append(result, *iface)
	}

	return result, nil
}

// extractIndex extracts the index from an OID string
func extractIndex(oid string) int {
	// Parse last number from OID like .1.3.6.1.2.1.2.2.1.2.1
	var idx int
	if _, err := fmt.Sscanf(oid, "%*s.%d", &idx); err != nil {
		// Try alternative parsing
		parts := make([]string, 0)
		for i := len(oid) - 1; i >= 0; i-- {
			if oid[i] == '.' {
				if len(parts) > 0 {
					fmt.Sscanf(oid[i+1:], "%d", &idx)
					return idx
				}
				parts = append(parts, string(oid[i+1:]))
			}
		}
	}
	return idx
}

// pollingLoop runs the periodic polling
func (m *SNMPManager) pollingLoop() {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.pollAllDevices()
		}
	}
}

// pollAllDevices polls all enabled devices
func (m *SNMPManager) pollAllDevices() {
	m.mu.RLock()
	devices := make([]*SNMPDevice, 0, len(m.devices))
	for _, d := range m.devices {
		if d.Enabled {
			devices = append(devices, d)
		}
	}
	m.mu.RUnlock()

	for _, device := range devices {
		go func(d *SNMPDevice) {
			if err := m.PollDevice(d); err != nil {
				log.Printf("Failed to poll device %s: %v", d.IP, err)
			}
		}(device)
	}
}

// updateDeviceStatus updates device status in memory and DB
func (m *SNMPManager) updateDeviceStatus(device *SNMPDevice) {
	m.mu.Lock()
	m.devices[device.ID] = device
	m.mu.Unlock()

	if m.db != nil {
		m.saveDeviceToDB(device)
	}
}

// saveDeviceToDB saves a device to the database
func (m *SNMPManager) saveDeviceToDB(device *SNMPDevice) error {
	if m.db == nil {
		return nil
	}

	db := m.db.GetDB()
	interfacesJSON, _ := json.Marshal(device.Interfaces)

	_, err := db.Exec(`
		INSERT INTO snmp_devices (id, name, ip, community, version, port, enabled, status, last_polled, sys_descr, sys_name, interfaces)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			ip = excluded.ip,
			community = excluded.community,
			version = excluded.version,
			port = excluded.port,
			enabled = excluded.enabled,
			status = excluded.status,
			last_polled = excluded.last_polled,
			sys_descr = excluded.sys_descr,
			sys_name = excluded.sys_name,
			interfaces = excluded.interfaces
	`, device.ID, device.Name, device.IP, device.Community, device.Version,
		device.Port, device.Enabled, device.Status, device.LastPolled,
		device.SysDescr, device.SysName, interfacesJSON)

	return err
}

// loadDevices loads devices from the database
func (m *SNMPManager) loadDevices() error {
	if m.db == nil {
		return nil
	}

	db := m.db.GetDB()
	rows, err := db.Query(`
		SELECT id, name, ip, community, version, port, enabled, status, last_polled, sys_descr, sys_name, interfaces
		FROM snmp_devices
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var d SNMPDevice
		var interfacesJSON []byte
		var lastPolled interface{}

		err := rows.Scan(&d.ID, &d.Name, &d.IP, &d.Community, &d.Version, &d.Port,
			&d.Enabled, &d.Status, &lastPolled, &d.SysDescr, &d.SysName, &interfacesJSON)
		if err != nil {
			log.Printf("Failed to scan SNMP device: %v", err)
			continue
		}

		if lastPolled != nil {
			if t, ok := lastPolled.(time.Time); ok {
				d.LastPolled = &t
			}
		}

		if len(interfacesJSON) > 0 {
			json.Unmarshal(interfacesJSON, &d.Interfaces)
		}

		m.devices[d.ID] = &d
	}

	return rows.Err()
}

// saveInterfacesToDB saves interfaces to the database
func (m *SNMPManager) saveInterfacesToDB(deviceID string, interfaces []SNMPInterface) error {
	if m.db == nil {
		return nil
	}

	db := m.db.GetDB()

	// Delete old interfaces
	_, err := db.Exec("DELETE FROM snmp_interfaces WHERE device_id = ?", deviceID)
	if err != nil {
		return err
	}

	// Insert new interfaces
	for _, iface := range interfaces {
		_, err := db.Exec(`
			INSERT INTO snmp_interfaces (device_id, interface_index, name, status, in_octets, out_octets, speed, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, deviceID, iface.Index, iface.Name, iface.Status, iface.InOctets, iface.OutOctets, iface.Speed, time.Now())
		if err != nil {
			log.Printf("Failed to save interface: %v", err)
		}
	}

	return nil
}

// loadInterfacesFromDB loads interfaces from the database
func (m *SNMPManager) loadInterfacesFromDB(deviceID string) ([]SNMPInterface, error) {
	if m.db == nil {
		return nil, nil
	}

	db := m.db.GetDB()
	rows, err := db.Query(`
		SELECT interface_index, name, status, in_octets, out_octets, speed
		FROM snmp_interfaces
		WHERE device_id = ?
		ORDER BY interface_index
	`, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var interfaces []SNMPInterface
	for rows.Next() {
		var iface SNMPInterface
		err := rows.Scan(&iface.Index, &iface.Name, &iface.Status, &iface.InOctets, &iface.OutOctets, &iface.Speed)
		if err != nil {
			continue
		}
		interfaces = append(interfaces, iface)
	}

	return interfaces, rows.Err()
}

// TestConnection tests connectivity to a device without adding it
func TestConnection(ip, community, version string, port uint16) error {
	if port == 0 {
		port = 161
	}

	g := &gosnmp.GoSNMP{
		Target:    ip,
		Port:      port,
		Community: community,
		Version:   gosnmp.Version2c,
		Timeout:   5 * time.Second,
		Retries:   1,
	}

	if version == "v1" {
		g.Version = gosnmp.Version1
	}

	if err := g.Connect(); err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer g.Conn.Close()

	// Try to get sysDescr
	_, err := g.Get([]string{".1.3.6.1.2.1.1.1.0"})
	if err != nil {
		return fmt.Errorf("SNMP query failed: %w", err)
	}

	return nil
}

// IsValidIP checks if the given string is a valid IP address
func IsValidIP(ip string) bool {
	return net.ParseIP(ip) != nil
}
