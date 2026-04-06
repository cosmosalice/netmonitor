package capture

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// InterfaceStatus represents the status of a network interface
type InterfaceStatus struct {
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	IPAddress       string    `json:"ip_address"`
	MACAddress      string    `json:"mac_address"`
	IsUp            bool      `json:"is_up"`
	IsEnabled       bool      `json:"is_enabled"`
	IsCapturing     bool      `json:"is_capturing"`
	PacketsCaptured uint64    `json:"packets_captured"`
	BytesCaptured   uint64    `json:"bytes_captured"`
	LastStarted     time.Time `json:"last_started,omitempty"`
	Error           string    `json:"error,omitempty"`
}

// InterfaceManager manages multiple network interfaces for packet capture
type InterfaceManager struct {
	mu             sync.RWMutex
	interfaces     map[string]*InterfaceStatus
	engines        map[string]*CaptureEngine
	packetHandlers []func(Packet)
	ctx            context.Context
	cancel         context.CancelFunc
}

// NewInterfaceManager creates a new interface manager
func NewInterfaceManager() *InterfaceManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &InterfaceManager{
		interfaces:     make(map[string]*InterfaceStatus),
		engines:        make(map[string]*CaptureEngine),
		packetHandlers: make([]func(Packet), 0),
		ctx:            ctx,
		cancel:         cancel,
	}
}

// ListInterfaces returns all available network interfaces
func (im *InterfaceManager) ListInterfaces() ([]InterfaceStatus, error) {
	// Get all pcap interfaces
	pcapIfaces, err := ListInterfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to list interfaces: %w", err)
	}

	im.mu.Lock()
	defer im.mu.Unlock()

	// Update internal map with latest interface info
	result := make([]InterfaceStatus, 0, len(pcapIfaces))
	for _, iface := range pcapIfaces {
		status, exists := im.interfaces[iface.Name]
		if !exists {
			status = &InterfaceStatus{
				Name:        iface.Name,
				Description: iface.Description,
				IPAddress:   iface.IPAddress,
				MACAddress:  iface.MACAddress,
				IsUp:        iface.IsUp,
				IsEnabled:   false,
				IsCapturing: false,
			}
			im.interfaces[iface.Name] = status
		} else {
			// Update dynamic fields
			status.Description = iface.Description
			status.IPAddress = iface.IPAddress
			status.MACAddress = iface.MACAddress
			status.IsUp = iface.IsUp
		}
		result = append(result, *status)
	}

	return result, nil
}

// GetActiveInterfaces returns currently active (enabled) interfaces
func (im *InterfaceManager) GetActiveInterfaces() []InterfaceStatus {
	im.mu.RLock()
	defer im.mu.RUnlock()

	result := make([]InterfaceStatus, 0)
	for _, status := range im.interfaces {
		if status.IsEnabled {
			result = append(result, *status)
		}
	}
	return result
}

// EnableInterface enables packet capture on an interface
func (im *InterfaceManager) EnableInterface(name string, bpfFilter string) error {
	im.mu.Lock()
	defer im.mu.Unlock()

	status, exists := im.interfaces[name]
	if !exists {
		// Try to refresh interface list
		im.mu.Unlock()
		im.ListInterfaces()
		im.mu.Lock()
		status, exists = im.interfaces[name]
		if !exists {
			return fmt.Errorf("interface %s not found", name)
		}
	}

	if status.IsEnabled && status.IsCapturing {
		return nil // Already enabled
	}

	// Create new capture engine for this interface
	engine := NewCaptureEngine()
	cfg := CaptureConfig{
		Interface: name,
		BPFFilter: bpfFilter,
		Snaplen:   65536,
		Promisc:   true,
		Timeout:   1 * time.Second,
	}

	if err := engine.Start(cfg); err != nil {
		status.Error = err.Error()
		return fmt.Errorf("failed to start capture on %s: %w", name, err)
	}

	im.engines[name] = engine
	status.IsEnabled = true
	status.IsCapturing = true
	status.LastStarted = time.Now()
	status.Error = ""

	// Start packet processing goroutine for this interface
	go im.processPackets(name, engine)

	log.Printf("[InterfaceManager] Enabled interface: %s", name)
	return nil
}

// DisableInterface disables packet capture on an interface
func (im *InterfaceManager) DisableInterface(name string) error {
	im.mu.Lock()
	defer im.mu.Unlock()

	status, exists := im.interfaces[name]
	if !exists {
		return fmt.Errorf("interface %s not found", name)
	}

	if !status.IsEnabled {
		return nil // Already disabled
	}

	engine, exists := im.engines[name]
	if exists && engine != nil {
		if err := engine.Stop(); err != nil {
			log.Printf("[InterfaceManager] Error stopping capture on %s: %v", name, err)
		}
		delete(im.engines, name)
	}

	status.IsEnabled = false
	status.IsCapturing = false
	status.Error = ""

	log.Printf("[InterfaceManager] Disabled interface: %s", name)
	return nil
}

// GetInterfaceStats returns statistics for a specific interface
func (im *InterfaceManager) GetInterfaceStats(name string) (*InterfaceStatus, error) {
	im.mu.RLock()
	defer im.mu.RUnlock()

	status, exists := im.interfaces[name]
	if !exists {
		return nil, fmt.Errorf("interface %s not found", name)
	}

	// Update capture stats if engine exists
	if engine, ok := im.engines[name]; ok && engine != nil {
		if stats, err := engine.GetStats(); err == nil {
			status.PacketsCaptured = stats["packets_received"]
		}
	}

	result := *status
	return &result, nil
}

// RegisterPacketHandler registers a callback for captured packets
func (im *InterfaceManager) RegisterPacketHandler(handler func(Packet)) {
	im.mu.Lock()
	defer im.mu.Unlock()
	im.packetHandlers = append(im.packetHandlers, handler)
}

// processPackets processes packets from a specific interface
func (im *InterfaceManager) processPackets(name string, engine *CaptureEngine) {
	packetCh := engine.GetPacketChannel()
	errCh := engine.GetErrorChannel()

	for {
		select {
		case <-im.ctx.Done():
			return
		case pkt, ok := <-packetCh:
			if !ok {
				return
			}
			// Mark packet with interface name
			pkt.Interface = name
			// Notify all handlers
			im.mu.RLock()
			handlers := make([]func(Packet), len(im.packetHandlers))
			copy(handlers, im.packetHandlers)
			im.mu.RUnlock()

			for _, handler := range handlers {
				handler(pkt)
			}

			// Update stats
			im.mu.Lock()
			if status, ok := im.interfaces[name]; ok {
				status.PacketsCaptured++
				status.BytesCaptured += uint64(len(pkt.Data))
			}
			im.mu.Unlock()

		case err := <-errCh:
			log.Printf("[InterfaceManager] Capture error on %s: %v", name, err)
			im.mu.Lock()
			if status, ok := im.interfaces[name]; ok {
				status.Error = err.Error()
			}
			im.mu.Unlock()
		}
	}
}

// Stop stops all interface captures
func (im *InterfaceManager) Stop() {
	im.cancel()

	im.mu.Lock()
	defer im.mu.Unlock()

	for name, engine := range im.engines {
		if engine != nil {
			if err := engine.Stop(); err != nil {
				log.Printf("[InterfaceManager] Error stopping %s: %v", name, err)
			}
		}
	}

	im.engines = make(map[string]*CaptureEngine)
	for _, status := range im.interfaces {
		status.IsEnabled = false
		status.IsCapturing = false
	}
}

// GetTotalStats returns aggregated statistics across all interfaces
func (im *InterfaceManager) GetTotalStats() map[string]interface{} {
	im.mu.RLock()
	defer im.mu.RUnlock()

	var totalPackets, totalBytes uint64
	activeCount := 0

	for _, status := range im.interfaces {
		if status.IsEnabled {
			activeCount++
			totalPackets += status.PacketsCaptured
			totalBytes += status.BytesCaptured
		}
	}

	return map[string]interface{}{
		"active_interfaces": activeCount,
		"total_packets":     totalPackets,
		"total_bytes":       totalBytes,
	}
}
