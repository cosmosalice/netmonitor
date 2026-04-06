package capture

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// NetFlowV5Header represents NetFlow v5 packet header
type NetFlowV5Header struct {
	Version      uint16
	Count        uint16
	SysUptime    uint32
	UnixSecs     uint32
	UnixNsecs    uint32
	FlowSequence uint32
	EngineType   uint8
	EngineID     uint8
	SamplingMode uint16 // actually 2 bits + 14 bits
}

// NetFlowV5Record represents a single NetFlow v5 flow record
type NetFlowV5Record struct {
	SrcAddr  uint32
	DstAddr  uint32
	NextHop  uint32
	Input    uint16
	Output   uint16
	Packets  uint32
	Octets   uint32
	First    uint32
	Last     uint32
	SrcPort  uint16
	DstPort  uint16
	Pad1     uint8
	TCPFlags uint8
	Protocol uint8
	TOS      uint8
	SrcAS    uint16
	DstAS    uint16
	SrcMask  uint8
	DstMask  uint8
	Pad2     uint16
}

// NetFlowFlow represents a parsed NetFlow flow
type NetFlowFlow struct {
	SrcIP       string
	DstIP       string
	SrcPort     uint16
	DstPort     uint16
	Protocol    uint8
	Bytes       uint64
	Packets     uint64
	FirstSeen   time.Time
	LastSeen    time.Time
	TCPFlags    uint8
	TOS         uint8
	InputIface  uint16
	OutputIface uint16
	SrcAS       uint16
	DstAS       uint16
}

// NetFlowCollectorStats holds collector statistics
type NetFlowCollectorStats struct {
	PacketsReceived uint64
	FlowsReceived   uint64
	BytesReceived   uint64
	Errors          uint64
	LastPacketTime  time.Time
}

// NetFlowCollector handles NetFlow v5/v9 collection
type NetFlowCollector struct {
	mu       sync.RWMutex
	conn     *net.UDPConn
	port     int
	running  bool
	stopChan chan struct{}
	stats    NetFlowCollectorStats
	onFlow   func(*NetFlowFlow)
}

// NewNetFlowCollector creates a new NetFlow collector
func NewNetFlowCollector(onFlow func(*NetFlowFlow)) *NetFlowCollector {
	return &NetFlowCollector{
		port:     2055,
		stopChan: make(chan struct{}),
		onFlow:   onFlow,
	}
}

// Start starts the NetFlow collector on the specified port
func (nfc *NetFlowCollector) Start(port int) error {
	nfc.mu.Lock()
	defer nfc.mu.Unlock()

	if nfc.running {
		return fmt.Errorf("NetFlow collector already running")
	}

	if port > 0 {
		nfc.port = port
	}

	addr := &net.UDPAddr{Port: nfc.port}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on UDP port %d: %w", nfc.port, err)
	}

	nfc.conn = conn
	nfc.running = true

	// Start receiving goroutine
	go nfc.receiveLoop()

	log.Printf("[NetFlow] Collector started on port %d", nfc.port)
	return nil
}

// Stop stops the NetFlow collector
func (nfc *NetFlowCollector) Stop() error {
	nfc.mu.Lock()
	defer nfc.mu.Unlock()

	if !nfc.running {
		return nil
	}

	nfc.running = false
	close(nfc.stopChan)

	if nfc.conn != nil {
		nfc.conn.Close()
	}

	log.Printf("[NetFlow] Collector stopped")
	return nil
}

// IsRunning returns whether the collector is running
func (nfc *NetFlowCollector) IsRunning() bool {
	nfc.mu.RLock()
	defer nfc.mu.RUnlock()
	return nfc.running
}

// GetStats returns collector statistics
func (nfc *NetFlowCollector) GetStats() NetFlowCollectorStats {
	nfc.mu.RLock()
	defer nfc.mu.RUnlock()
	return nfc.stats
}

// receiveLoop is the main packet receiving loop
func (nfc *NetFlowCollector) receiveLoop() {
	buffer := make([]byte, 65535)

	for {
		select {
		case <-nfc.stopChan:
			return
		default:
		}

		nfc.conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, addr, err := nfc.conn.ReadFromUDP(buffer)
		if err != nil {
			if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
				continue
			}
			if nfc.running {
				log.Printf("[NetFlow] Read error: %v", err)
				atomic.AddUint64(&nfc.stats.Errors, 1)
			}
			continue
		}

		atomic.AddUint64(&nfc.stats.PacketsReceived, 1)
		atomic.AddUint64(&nfc.stats.BytesReceived, uint64(n))
		nfc.stats.LastPacketTime = time.Now()

		if err := nfc.processPacket(buffer[:n], addr); err != nil {
			log.Printf("[NetFlow] Process error: %v", err)
			atomic.AddUint64(&nfc.stats.Errors, 1)
		}
	}
}

// processPacket processes a single NetFlow packet
func (nfc *NetFlowCollector) processPacket(data []byte, addr *net.UDPAddr) error {
	if len(data) < 4 {
		return fmt.Errorf("packet too short")
	}

	version := binary.BigEndian.Uint16(data[0:2])

	switch version {
	case 5:
		return nfc.processV5Packet(data, addr)
	case 9:
		return nfc.processV9Packet(data, addr)
	default:
		return fmt.Errorf("unsupported NetFlow version: %d", version)
	}
}

// processV5Packet processes a NetFlow v5 packet
func (nfc *NetFlowCollector) processV5Packet(data []byte, addr *net.UDPAddr) error {
	if len(data) < 24 {
		return fmt.Errorf("v5 packet too short for header")
	}

	header := NetFlowV5Header{
		Version:      binary.BigEndian.Uint16(data[0:2]),
		Count:        binary.BigEndian.Uint16(data[2:4]),
		SysUptime:    binary.BigEndian.Uint32(data[4:8]),
		UnixSecs:     binary.BigEndian.Uint32(data[8:12]),
		UnixNsecs:    binary.BigEndian.Uint32(data[12:16]),
		FlowSequence: binary.BigEndian.Uint32(data[16:20]),
		EngineType:   data[20],
		EngineID:     data[21],
		SamplingMode: binary.BigEndian.Uint16(data[22:24]),
	}

	recordSize := 48 // NetFlow v5 record size
	offset := 24     // Header size

	for i := uint16(0); i < header.Count; i++ {
		if offset+recordSize > len(data) {
			return fmt.Errorf("packet truncated at record %d", i)
		}

		record := nfc.parseV5Record(data[offset:offset+recordSize], header)
		if record != nil && nfc.onFlow != nil {
			nfc.onFlow(record)
		}

		atomic.AddUint64(&nfc.stats.FlowsReceived, 1)
		offset += recordSize
	}

	return nil
}

// parseV5Record parses a single NetFlow v5 record
func (nfc *NetFlowCollector) parseV5Record(data []byte, header NetFlowV5Header) *NetFlowFlow {
	if len(data) < 48 {
		return nil
	}

	record := NetFlowV5Record{
		SrcAddr:  binary.BigEndian.Uint32(data[0:4]),
		DstAddr:  binary.BigEndian.Uint32(data[4:8]),
		NextHop:  binary.BigEndian.Uint32(data[8:12]),
		Input:    binary.BigEndian.Uint16(data[12:14]),
		Output:   binary.BigEndian.Uint16(data[14:16]),
		Packets:  binary.BigEndian.Uint32(data[16:20]),
		Octets:   binary.BigEndian.Uint32(data[20:24]),
		First:    binary.BigEndian.Uint32(data[24:28]),
		Last:     binary.BigEndian.Uint32(data[28:32]),
		SrcPort:  binary.BigEndian.Uint16(data[32:34]),
		DstPort:  binary.BigEndian.Uint16(data[34:36]),
		Pad1:     data[36],
		TCPFlags: data[37],
		Protocol: data[38],
		TOS:      data[39],
		SrcAS:    binary.BigEndian.Uint16(data[40:42]),
		DstAS:    binary.BigEndian.Uint16(data[42:44]),
		SrcMask:  data[44],
		DstMask:  data[45],
		Pad2:     binary.BigEndian.Uint16(data[46:48]),
	}

	// Convert to NetFlowFlow
	baseTime := time.Unix(int64(header.UnixSecs), int64(header.UnixNsecs))
	uptime := time.Duration(header.SysUptime) * time.Millisecond

	firstDuration := time.Duration(record.First) * time.Millisecond
	lastDuration := time.Duration(record.Last) * time.Millisecond

	flow := &NetFlowFlow{
		SrcIP:       intToIP(record.SrcAddr).String(),
		DstIP:       intToIP(record.DstAddr).String(),
		SrcPort:     record.SrcPort,
		DstPort:     record.DstPort,
		Protocol:    record.Protocol,
		Bytes:       uint64(record.Octets),
		Packets:     uint64(record.Packets),
		FirstSeen:   baseTime.Add(firstDuration - uptime),
		LastSeen:    baseTime.Add(lastDuration - uptime),
		TCPFlags:    record.TCPFlags,
		TOS:         record.TOS,
		InputIface:  record.Input,
		OutputIface: record.Output,
		SrcAS:       record.SrcAS,
		DstAS:       record.DstAS,
	}

	return flow
}

// processV9Packet processes a NetFlow v9 packet (basic support)
func (nfc *NetFlowCollector) processV9Packet(data []byte, addr *net.UDPAddr) error {
	// NetFlow v9 is more complex with templates
	// For now, just log that we received a v9 packet
	// Full v9 support would require template management
	log.Printf("[NetFlow] Received v9 packet from %s (v9 full parsing not implemented)", addr)
	return nil
}

// intToIP converts a uint32 IP to net.IP
func intToIP(ip uint32) net.IP {
	result := make(net.IP, 4)
	binary.BigEndian.PutUint32(result, ip)
	return result
}

// ipToInt converts net.IP to uint32
func ipToInt(ip net.IP) uint32 {
	if len(ip) == 16 {
		return binary.BigEndian.Uint32(ip[12:16])
	}
	return binary.BigEndian.Uint32(ip)
}
