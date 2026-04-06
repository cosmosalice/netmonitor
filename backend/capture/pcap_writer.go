package capture

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
)

// PCAPWriter manages packet buffering and PCAP file export
type PCAPWriter struct {
	mu     sync.Mutex
	buffer *PacketBuffer
}

// PacketBuffer is a ring buffer holding recent packets
type PacketBuffer struct {
	packets []gopacket.Packet
	maxSize int
	mu      sync.Mutex
	pos     int // current write position
	count   int // total packets written (for ring logic)
}

// NewPCAPWriter creates a new PCAP writer with a ring buffer
func NewPCAPWriter() *PCAPWriter {
	return &PCAPWriter{
		buffer: &PacketBuffer{
			packets: make([]gopacket.Packet, 10000),
			maxSize: 10000,
		},
	}
}

// AddPacket adds a packet to the ring buffer
func (pw *PCAPWriter) AddPacket(packet gopacket.Packet) {
	pw.buffer.mu.Lock()
	defer pw.buffer.mu.Unlock()

	pw.buffer.packets[pw.buffer.pos] = packet
	pw.buffer.pos = (pw.buffer.pos + 1) % pw.buffer.maxSize
	pw.buffer.count++
}

// getPackets returns a snapshot of all buffered packets in chronological order
func (pw *PCAPWriter) getPackets() []gopacket.Packet {
	pw.buffer.mu.Lock()
	defer pw.buffer.mu.Unlock()

	total := pw.buffer.count
	if total == 0 {
		return nil
	}

	size := pw.buffer.maxSize
	if total < size {
		size = total
	}

	result := make([]gopacket.Packet, 0, size)

	if total >= pw.buffer.maxSize {
		// Ring buffer is full, read from pos (oldest) to pos-1 (newest)
		for i := 0; i < pw.buffer.maxSize; i++ {
			idx := (pw.buffer.pos + i) % pw.buffer.maxSize
			if pw.buffer.packets[idx] != nil {
				result = append(result, pw.buffer.packets[idx])
			}
		}
	} else {
		// Buffer not yet full
		for i := 0; i < total; i++ {
			if pw.buffer.packets[i] != nil {
				result = append(result, pw.buffer.packets[i])
			}
		}
	}

	return result
}

// WritePCAP writes filtered packets to PCAP format
func (pw *PCAPWriter) WritePCAP(w io.Writer, filter func(gopacket.Packet) bool) error {
	pcapWriter := pcapgo.NewWriter(w)
	if err := pcapWriter.WriteFileHeader(65536, layers.LinkTypeEthernet); err != nil {
		return fmt.Errorf("failed to write pcap header: %w", err)
	}

	packets := pw.getPackets()
	for _, pkt := range packets {
		if filter != nil && !filter(pkt) {
			continue
		}
		ci := pkt.Metadata().CaptureInfo
		if ci.CaptureLength == 0 {
			ci.CaptureLength = len(pkt.Data())
			ci.Length = len(pkt.Data())
		}
		if ci.Timestamp.IsZero() {
			ci.Timestamp = time.Now()
		}
		if err := pcapWriter.WritePacket(ci, pkt.Data()); err != nil {
			return fmt.Errorf("failed to write packet: %w", err)
		}
	}

	return nil
}

// FilterByFlow returns a filter function matching packets belonging to a specific flow ID
// Flow ID format: "srcIP:srcPort->dstIP:dstPort"
func (pw *PCAPWriter) FilterByFlow(flowID string) func(gopacket.Packet) bool {
	// Parse flow ID to extract src/dst IP:port
	parts := strings.Split(flowID, "->")
	if len(parts) != 2 {
		// Try alternate format with arrow
		parts = strings.Split(flowID, "→")
	}
	if len(parts) != 2 {
		return func(gopacket.Packet) bool { return false }
	}

	srcParts := strings.Split(strings.TrimSpace(parts[0]), ":")
	dstParts := strings.Split(strings.TrimSpace(parts[1]), ":")

	if len(srcParts) < 2 || len(dstParts) < 2 {
		return func(gopacket.Packet) bool { return false }
	}

	srcIP := srcParts[0]
	srcPort := srcParts[1]
	dstIP := dstParts[0]
	dstPort := dstParts[1]

	return func(pkt gopacket.Packet) bool {
		netFlow, transportFlow := extractFlowInfo(pkt)
		if netFlow == nil {
			return false
		}

		pSrcIP := netFlow.Src().String()
		pDstIP := netFlow.Dst().String()
		pSrcPort := ""
		pDstPort := ""
		if transportFlow != nil {
			pSrcPort = transportFlow.Src().String()
			pDstPort = transportFlow.Dst().String()
		}

		// Match in either direction
		return (pSrcIP == srcIP && pDstIP == dstIP && pSrcPort == srcPort && pDstPort == dstPort) ||
			(pSrcIP == dstIP && pDstIP == srcIP && pSrcPort == dstPort && pDstPort == srcPort)
	}
}

// FilterByHost returns a filter function matching packets involving a specific IP
func (pw *PCAPWriter) FilterByHost(ip string) func(gopacket.Packet) bool {
	return func(pkt gopacket.Packet) bool {
		netFlow, _ := extractFlowInfo(pkt)
		if netFlow == nil {
			return false
		}
		return netFlow.Src().String() == ip || netFlow.Dst().String() == ip
	}
}

// FilterByDuration returns a filter for packets within the last N seconds
func (pw *PCAPWriter) FilterByDuration(seconds int) func(gopacket.Packet) bool {
	cutoff := time.Now().Add(-time.Duration(seconds) * time.Second)
	return func(pkt gopacket.Packet) bool {
		ts := pkt.Metadata().Timestamp
		if ts.IsZero() {
			return true
		}
		return ts.After(cutoff)
	}
}

// FilterByBPF performs simple IP/port matching (simplified BPF-like filter)
func (pw *PCAPWriter) FilterByBPF(filter string) func(gopacket.Packet) bool {
	if filter == "" {
		return func(gopacket.Packet) bool { return true }
	}

	// Simple parsing: "host X.X.X.X", "port XXXX", "host X.X.X.X and port XXXX"
	filter = strings.TrimSpace(strings.ToLower(filter))

	var hostFilter string
	var portFilter string

	parts := strings.Split(filter, " and ")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "host ") {
			hostFilter = strings.TrimSpace(strings.TrimPrefix(part, "host "))
		} else if strings.HasPrefix(part, "port ") {
			portFilter = strings.TrimSpace(strings.TrimPrefix(part, "port "))
		}
	}

	return func(pkt gopacket.Packet) bool {
		netFlow, transportFlow := extractFlowInfo(pkt)

		if hostFilter != "" {
			if netFlow == nil {
				return false
			}
			srcIP := netFlow.Src().String()
			dstIP := netFlow.Dst().String()
			if srcIP != hostFilter && dstIP != hostFilter {
				return false
			}
		}

		if portFilter != "" {
			if transportFlow == nil {
				return false
			}
			srcPort := transportFlow.Src().String()
			dstPort := transportFlow.Dst().String()
			if srcPort != portFilter && dstPort != portFilter {
				return false
			}
		}

		return true
	}
}

// extractFlowInfo extracts network and transport flow from a packet
func extractFlowInfo(pkt gopacket.Packet) (netFlow *gopacket.Flow, transportFlow *gopacket.Flow) {
	if nl := pkt.NetworkLayer(); nl != nil {
		f := nl.NetworkFlow()
		netFlow = &f
	}
	if tl := pkt.TransportLayer(); tl != nil {
		f := tl.TransportFlow()
		transportFlow = &f
	}
	return
}

// PacketFromRaw creates a gopacket.Packet from raw Packet data
func PacketFromRaw(pkt Packet) gopacket.Packet {
	p := gopacket.NewPacket(pkt.Data, layers.LinkTypeEthernet, gopacket.NoCopy)
	md := p.Metadata()
	md.Timestamp = pkt.Timestamp
	md.CaptureInfo = pkt.CaptureInfo
	md.CaptureLength = len(pkt.Data)
	md.Length = len(pkt.Data)
	return p
}
