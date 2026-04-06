package capture

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

// Packet represents a captured packet with metadata
type Packet struct {
	Data        []byte
	Timestamp   time.Time
	CaptureInfo gopacket.CaptureInfo
	Interface   string
	VLANID      uint16 // 802.1Q VLAN ID (0 if no VLAN tag)
}

// CaptureEngine handles packet capture
type CaptureEngine struct {
	mu            sync.Mutex
	handle        *pcap.Handle
	interfaceName string
	bpfFilter     string
	packetChan    chan Packet
	errChan       chan error
	ctx           context.Context
	cancel        context.CancelFunc
	isRunning     bool
	snaplen       int32
	promisc       bool
	timeout       time.Duration
}

// CaptureConfig holds configuration for packet capture
type CaptureConfig struct {
	Interface  string
	BPFFilter  string
	Snaplen    int32
	Promisc    bool
	Timeout    time.Duration
	BufferSize int
}

// NewCaptureEngine creates a new capture engine
func NewCaptureEngine() *CaptureEngine {
	return &CaptureEngine{
		packetChan: make(chan Packet, 10000),
		errChan:    make(chan error, 10),
		snaplen:    65536,
		promisc:    true,
		timeout:    30 * time.Second,
	}
}

// Start begins packet capture on the specified interface
func (ce *CaptureEngine) Start(config CaptureConfig) error {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	if ce.isRunning {
		return fmt.Errorf("capture already running")
	}

	// Open capture handle
	log.Printf("[capture] Opening interface %s (snaplen=%d, promisc=%v, timeout=%v)",
		config.Interface, config.Snaplen, config.Promisc, config.Timeout)
	handle, err := pcap.OpenLive(
		config.Interface,
		config.Snaplen,
		config.Promisc,
		config.Timeout,
	)
	if err != nil {
		log.Printf("[capture] Failed to open interface %s: %v", config.Interface, err)
		return fmt.Errorf("failed to open interface %s: %w", config.Interface, err)
	}
	log.Printf("[capture] Interface opened successfully, LinkType=%v", handle.LinkType())

	// Set BPF filter if provided
	if config.BPFFilter != "" {
		err = handle.SetBPFFilter(config.BPFFilter)
		if err != nil {
			handle.Close()
			log.Printf("[capture] Failed to set BPF filter '%s': %v", config.BPFFilter, err)
			return fmt.Errorf("failed to set BPF filter: %w", err)
		}
		log.Printf("[capture] BPF filter set: %s", config.BPFFilter)
	} else {
		log.Println("[capture] No BPF filter applied")
	}

	ce.handle = handle
	ce.interfaceName = config.Interface
	ce.bpfFilter = config.BPFFilter
	ce.isRunning = true

	// Create context for cancellation
	ce.ctx, ce.cancel = context.WithCancel(context.Background())

	// Start capture goroutine
	go ce.captureLoop()

	log.Printf("[capture] Started capture on interface: %s", config.Interface)
	return nil
}

// Stop stops packet capture
func (ce *CaptureEngine) Stop() error {
	ce.mu.Lock()

	if !ce.isRunning {
		ce.mu.Unlock()
		return nil // already stopped, not an error
	}

	// 1. Mark as stopped first so captureLoop checks see it quickly
	ce.isRunning = false

	// 2. Cancel context to signal captureLoop to exit
	if ce.cancel != nil {
		ce.cancel()
	}

	// 3. Save handle reference and nil it under the lock, then close outside
	handle := ce.handle
	ce.handle = nil
	ce.mu.Unlock()

	if handle != nil {
		handle.Close()
	}

	// Give the goroutine a short grace period to exit cleanly
	time.Sleep(100 * time.Millisecond)

	log.Printf("[capture] Stopped capture on interface: %s", ce.interfaceName)
	return nil
}

// GetPacketChannel returns the channel for receiving captured packets
func (ce *CaptureEngine) GetPacketChannel() <-chan Packet {
	return ce.packetChan
}

// GetErrorChannel returns the channel for receiving errors
func (ce *CaptureEngine) GetErrorChannel() <-chan error {
	return ce.errChan
}

// IsRunning returns whether capture is currently active
func (ce *CaptureEngine) IsRunning() bool {
	ce.mu.Lock()
	defer ce.mu.Unlock()
	return ce.isRunning
}

// GetStats returns capture statistics
func (ce *CaptureEngine) GetStats() (map[string]uint64, error) {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	if ce.handle == nil {
		return nil, fmt.Errorf("capture not running")
	}

	stats, err := ce.handle.Stats()
	if err != nil {
		return nil, err
	}

	return map[string]uint64{
		"packets_received":   uint64(stats.PacketsReceived),
		"packets_dropped":    uint64(stats.PacketsDropped),
		"packets_if_dropped": uint64(stats.PacketsIfDropped),
	}, nil
}

// captureLoop is the main packet capture loop
func (ce *CaptureEngine) captureLoop() {
	log.Println("[capture] captureLoop started")
	defer log.Println("[capture] captureLoop exited")

	packetSource := gopacket.NewPacketSource(ce.handle, ce.handle.LinkType())

	var pktCount uint64

	for {
		// Check cancellation first
		select {
		case <-ce.ctx.Done():
			log.Printf("[capture] captureLoop cancelled after %d packets", pktCount)
			return
		default:
		}

		packet, err := packetSource.NextPacket()
		if err != nil {
			if ce.ctx.Err() != nil {
				log.Printf("[capture] captureLoop context done after %d packets", pktCount)
				return // Context cancelled, exit cleanly
			}
			// Timeout expired is normal when using packetSource.Timeout
			errMsg := err.Error()
			if strings.Contains(errMsg, "timeout expired") ||
				strings.Contains(errMsg, "Timeout Expired") {
				continue
			}
			ce.errChan <- fmt.Errorf("error reading packet: %w", err)
			continue
		}

		pktCount++
		if pktCount%1000 == 0 {
			log.Printf("[capture] captured %d packets so far", pktCount)
		}

		// Extract VLAN ID from 802.1Q tag
		var vlanID uint16 = 0
		if dot1q := packet.Layer(layers.LayerTypeDot1Q); dot1q != nil {
			if vlan, ok := dot1q.(*layers.Dot1Q); ok {
				vlanID = vlan.VLANIdentifier
			}
		}

		// Create packet wrapper
		pkt := Packet{
			Data:        packet.Data(),
			Timestamp:   packet.Metadata().Timestamp,
			CaptureInfo: packet.Metadata().CaptureInfo,
			Interface:   ce.interfaceName,
			VLANID:      vlanID,
		}

		// Send to channel (non-blocking)
		select {
		case ce.packetChan <- pkt:
		default:
			// Channel full, drop packet
			log.Println("[capture] Warning: packet channel full, dropping packet")
		}
	}
}
