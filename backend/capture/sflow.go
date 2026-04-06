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

// SFlowDatagram represents an sFlow v5 datagram header
type SFlowDatagram struct {
	Version        uint32
	IPVersion      uint32
	AgentAddress   []byte
	SubAgentID     uint32
	SequenceNumber uint32
	SysUptime      uint32
	NumSamples     uint32
}

// SFlowSample represents an sFlow sample
type SFlowSample struct {
	Enterprise   uint32
	SampleType   uint32
	SampleLength uint32
}

// SFlowFlowSample represents a flow sample
type SFlowFlowSample struct {
	SequenceNumber  uint32
	SourceIDClass   uint32
	SamplingRate    uint32
	SamplePool      uint32
	Drops           uint32
	InputInterface  uint32
	OutputInterface uint32
	NumFlowRecords  uint32
}

// SFlowRawPacketHeader represents raw packet header flow record
type SFlowRawPacketHeader struct {
	Protocol     uint32
	FrameLength  uint32
	Stripped     uint32
	HeaderLength uint32
	HeaderData   []byte
}

// SFlowFlow represents a parsed sFlow flow
type SFlowFlow struct {
	SrcIP           string
	DstIP           string
	SrcPort         uint16
	DstPort         uint16
	Protocol        uint8
	Bytes           uint64
	Packets         uint64
	InputInterface  uint32
	OutputInterface uint32
	SamplingRate    uint32
	AgentAddress    string
	Timestamp       time.Time
}

// SFlowCollectorStats holds collector statistics
type SFlowCollectorStats struct {
	PacketsReceived uint64
	FlowsReceived   uint64
	BytesReceived   uint64
	Errors          uint64
	LastPacketTime  time.Time
}

// SFlowCollector handles sFlow v5 collection
type SFlowCollector struct {
	mu       sync.RWMutex
	conn     *net.UDPConn
	port     int
	running  bool
	stopChan chan struct{}
	stats    SFlowCollectorStats
	onFlow   func(*SFlowFlow)
}

// NewSFlowCollector creates a new sFlow collector
func NewSFlowCollector(onFlow func(*SFlowFlow)) *SFlowCollector {
	return &SFlowCollector{
		port:     6343,
		stopChan: make(chan struct{}),
		onFlow:   onFlow,
	}
}

// Start starts the sFlow collector on the specified port
func (sfc *SFlowCollector) Start(port int) error {
	sfc.mu.Lock()
	defer sfc.mu.Unlock()

	if sfc.running {
		return fmt.Errorf("sFlow collector already running")
	}

	if port > 0 {
		sfc.port = port
	}

	addr := &net.UDPAddr{Port: sfc.port}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on UDP port %d: %w", sfc.port, err)
	}

	sfc.conn = conn
	sfc.running = true

	// Start receiving goroutine
	go sfc.receiveLoop()

	log.Printf("[sFlow] Collector started on port %d", sfc.port)
	return nil
}

// Stop stops the sFlow collector
func (sfc *SFlowCollector) Stop() error {
	sfc.mu.Lock()
	defer sfc.mu.Unlock()

	if !sfc.running {
		return nil
	}

	sfc.running = false
	close(sfc.stopChan)

	if sfc.conn != nil {
		sfc.conn.Close()
	}

	log.Printf("[sFlow] Collector stopped")
	return nil
}

// IsRunning returns whether the collector is running
func (sfc *SFlowCollector) IsRunning() bool {
	sfc.mu.RLock()
	defer sfc.mu.RUnlock()
	return sfc.running
}

// GetStats returns collector statistics
func (sfc *SFlowCollector) GetStats() SFlowCollectorStats {
	sfc.mu.RLock()
	defer sfc.mu.RUnlock()
	return sfc.stats
}

// receiveLoop is the main packet receiving loop
func (sfc *SFlowCollector) receiveLoop() {
	buffer := make([]byte, 65535)

	for {
		select {
		case <-sfc.stopChan:
			return
		default:
		}

		sfc.conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, addr, err := sfc.conn.ReadFromUDP(buffer)
		if err != nil {
			if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
				continue
			}
			if sfc.running {
				log.Printf("[sFlow] Read error: %v", err)
				atomic.AddUint64(&sfc.stats.Errors, 1)
			}
			continue
		}

		atomic.AddUint64(&sfc.stats.PacketsReceived, 1)
		atomic.AddUint64(&sfc.stats.BytesReceived, uint64(n))
		sfc.stats.LastPacketTime = time.Now()

		if err := sfc.processPacket(buffer[:n], addr); err != nil {
			log.Printf("[sFlow] Process error: %v", err)
			atomic.AddUint64(&sfc.stats.Errors, 1)
		}
	}
}

// processPacket processes a single sFlow datagram
func (sfc *SFlowCollector) processPacket(data []byte, addr *net.UDPAddr) error {
	if len(data) < 28 {
		return fmt.Errorf("packet too short for sFlow header")
	}

	datagram, offset, err := sfc.parseDatagram(data)
	if err != nil {
		return err
	}

	agentAddr := ""
	if datagram.IPVersion == 1 && len(datagram.AgentAddress) == 4 {
		agentAddr = net.IP(datagram.AgentAddress).String()
	}

	// Process samples
	for i := uint32(0); i < datagram.NumSamples; i++ {
		if offset+8 > len(data) {
			return fmt.Errorf("packet truncated at sample %d", i)
		}

		sample, newOffset, err := sfc.parseSample(data[offset:])
		if err != nil {
			return err
		}
		offset += newOffset

		// Process flow samples (type 1 = flow sample, type 3 = expanded flow sample)
		sampleType := sample.SampleType & 0x0FFF
		if sampleType == 1 || sampleType == 3 {
			sampleLen := int(sample.SampleLength)
			flows, err := sfc.parseFlowSample(data[offset-sampleLen:offset], sample, agentAddr)
			if err != nil {
				log.Printf("[sFlow] Error parsing flow sample: %v", err)
				continue
			}
			for _, flow := range flows {
				if sfc.onFlow != nil {
					sfc.onFlow(flow)
				}
				atomic.AddUint64(&sfc.stats.FlowsReceived, 1)
			}
		}
	}

	return nil
}

// parseDatagram parses the sFlow datagram header
func (sfc *SFlowCollector) parseDatagram(data []byte) (*SFlowDatagram, int, error) {
	if len(data) < 28 {
		return nil, 0, fmt.Errorf("packet too short")
	}

	dg := &SFlowDatagram{
		Version:        binary.BigEndian.Uint32(data[0:4]),
		IPVersion:      binary.BigEndian.Uint32(data[4:8]),
		SequenceNumber: binary.BigEndian.Uint32(data[16:20]),
		SysUptime:      binary.BigEndian.Uint32(data[20:24]),
		NumSamples:     binary.BigEndian.Uint32(data[24:28]),
	}

	offset := 28

	// Parse agent address based on IP version
	if dg.IPVersion == 1 { // IPv4
		if len(data) < offset+4 {
			return nil, 0, fmt.Errorf("packet too short for IPv4 agent address")
		}
		dg.AgentAddress = data[offset : offset+4]
		dg.SubAgentID = binary.BigEndian.Uint32(data[offset+4 : offset+8])
		offset += 8
	} else if dg.IPVersion == 2 { // IPv6
		if len(data) < offset+16 {
			return nil, 0, fmt.Errorf("packet too short for IPv6 agent address")
		}
		dg.AgentAddress = data[offset : offset+16]
		dg.SubAgentID = binary.BigEndian.Uint32(data[offset+16 : offset+20])
		offset += 20
	} else {
		return nil, 0, fmt.Errorf("invalid IP version: %d", dg.IPVersion)
	}

	return dg, offset, nil
}

// parseSample parses an sFlow sample header
func (sfc *SFlowCollector) parseSample(data []byte) (*SFlowSample, int, error) {
	if len(data) < 8 {
		return nil, 0, fmt.Errorf("sample data too short")
	}

	format := binary.BigEndian.Uint32(data[0:4])
	sample := &SFlowSample{
		Enterprise:   format >> 12,
		SampleType:   format & 0x0FFF,
		SampleLength: binary.BigEndian.Uint32(data[4:8]),
	}

	return sample, 8 + int(sample.SampleLength), nil
}

// parseFlowSample parses a flow sample
func (sfc *SFlowCollector) parseFlowSample(data []byte, sample *SFlowSample, agentAddr string) ([]*SFlowFlow, error) {
	if len(data) < int(sample.SampleLength) {
		return nil, fmt.Errorf("flow sample data too short")
	}

	offset := 8 // Skip sample header (already parsed)

	// Parse flow sample header
	if offset+28 > len(data) {
		return nil, fmt.Errorf("flow sample header too short")
	}

	flowSample := SFlowFlowSample{
		SequenceNumber:  binary.BigEndian.Uint32(data[offset : offset+4]),
		SourceIDClass:   binary.BigEndian.Uint32(data[offset+4 : offset+8]),
		SamplingRate:    binary.BigEndian.Uint32(data[offset+8 : offset+12]),
		SamplePool:      binary.BigEndian.Uint32(data[offset+12 : offset+16]),
		Drops:           binary.BigEndian.Uint32(data[offset+16 : offset+20]),
		InputInterface:  binary.BigEndian.Uint32(data[offset+20 : offset+24]),
		OutputInterface: binary.BigEndian.Uint32(data[offset+24 : offset+28]),
	}
	offset += 28

	if offset+4 > len(data) {
		return nil, fmt.Errorf("flow sample num records field missing")
	}
	flowSample.NumFlowRecords = binary.BigEndian.Uint32(data[offset : offset+4])
	offset += 4

	flows := make([]*SFlowFlow, 0)

	// Parse flow records
	for i := uint32(0); i < flowSample.NumFlowRecords; i++ {
		if offset+8 > len(data) {
			break
		}

		recordFormat := binary.BigEndian.Uint32(data[offset : offset+4])
		recordLength := binary.BigEndian.Uint32(data[offset+4 : offset+8])
		offset += 8

		recordType := recordFormat & 0x0FFF

		if recordType == 1 { // Raw packet header
			if offset+int(recordLength) > len(data) {
				break
			}
			flow := sfc.parseRawPacketHeader(data[offset:offset+int(recordLength)], &flowSample, agentAddr)
			if flow != nil {
				flows = append(flows, flow)
			}
		}

		offset += int(recordLength)
	}

	return flows, nil
}

// parseRawPacketHeader parses raw packet header flow record
func (sfc *SFlowCollector) parseRawPacketHeader(data []byte, sample *SFlowFlowSample, agentAddr string) *SFlowFlow {
	if len(data) < 16 {
		return nil
	}

	header := SFlowRawPacketHeader{
		Protocol:     binary.BigEndian.Uint32(data[0:4]),
		FrameLength:  binary.BigEndian.Uint32(data[4:8]),
		Stripped:     binary.BigEndian.Uint32(data[8:12]),
		HeaderLength: binary.BigEndian.Uint32(data[12:16]),
	}

	if len(data) < 16+int(header.HeaderLength) {
		return nil
	}
	header.HeaderData = data[16 : 16+int(header.HeaderLength)]

	flow := &SFlowFlow{
		Bytes:           uint64(header.FrameLength),
		Packets:         1,
		InputInterface:  sample.InputInterface,
		OutputInterface: sample.OutputInterface,
		SamplingRate:    sample.SamplingRate,
		AgentAddress:    agentAddr,
		Timestamp:       time.Now(),
	}

	// Parse Ethernet header
	if header.Protocol == 1 && len(header.HeaderData) >= 14 { // Ethernet
		etherType := binary.BigEndian.Uint16(header.HeaderData[12:14])
		offset := 14

		// Handle VLAN tag
		if etherType == 0x8100 && len(header.HeaderData) >= 18 {
			etherType = binary.BigEndian.Uint16(header.HeaderData[16:18])
			offset = 18
		}

		// Parse IP header
		if etherType == 0x0800 && len(header.HeaderData) >= offset+20 { // IPv4
			ipHeader := header.HeaderData[offset:]
			flow.Protocol = ipHeader[9]
			flow.SrcIP = net.IP(ipHeader[12:16]).String()
			flow.DstIP = net.IP(ipHeader[16:20]).String()

			// Parse TCP/UDP ports
			ipHeaderLen := int((ipHeader[0] & 0x0F) * 4)
			if len(ipHeader) >= ipHeaderLen+4 {
				transport := ipHeader[ipHeaderLen:]
				flow.SrcPort = binary.BigEndian.Uint16(transport[0:2])
				flow.DstPort = binary.BigEndian.Uint16(transport[2:4])
			}
		}
	}

	return flow
}
