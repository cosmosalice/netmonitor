package analysis

import (
	"fmt"
	"sync"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/netmonitor/backend/capture"
)

// Protocol represents a detected L7 protocol
type Protocol struct {
	ID       uint16 `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
}

// ProtocolDetector detects L7 protocols using port-based heuristics
// (fallback implementation when nDPI C library is unavailable)
type ProtocolDetector struct {
	mu      sync.Mutex
	portMap map[uint16]Protocol
}

// NewProtocolDetector creates a new port-based protocol detector
func NewProtocolDetector() (*ProtocolDetector, error) {
	pd := &ProtocolDetector{
		portMap: buildPortMap(),
	}
	return pd, nil
}

// DetectProtocol detects the L7 protocol of a packet using port heuristics
func (pd *ProtocolDetector) DetectProtocol(pkt capture.Packet) (*Protocol, error) {
	packet := gopacket.NewPacket(pkt.Data, layers.LayerTypeEthernet, gopacket.NoCopy)

	// Get TCP/UDP layer to extract ports
	if tcpLayer := packet.Layer(layers.LayerTypeTCP); tcpLayer != nil {
		tcp, _ := tcpLayer.(*layers.TCP)
		if tcp != nil {
			if p, ok := pd.lookupPort(uint16(tcp.DstPort)); ok {
				return &p, nil
			}
			if p, ok := pd.lookupPort(uint16(tcp.SrcPort)); ok {
				return &p, nil
			}
			return &Protocol{ID: 0, Name: "TCP", Category: "Network"}, nil
		}
	}

	if udpLayer := packet.Layer(layers.LayerTypeUDP); udpLayer != nil {
		udp, _ := udpLayer.(*layers.UDP)
		if udp != nil {
			if p, ok := pd.lookupPort(uint16(udp.DstPort)); ok {
				return &p, nil
			}
			if p, ok := pd.lookupPort(uint16(udp.SrcPort)); ok {
				return &p, nil
			}
			return &Protocol{ID: 0, Name: "UDP", Category: "Network"}, nil
		}
	}

	return &Protocol{ID: 0, Name: "Other", Category: "Network"}, nil
}

// ParsePacketInfo extracts IP/port/protocol/size info from a raw packet
func ParsePacketInfo(pkt capture.Packet) (srcIP, dstIP string, srcPort, dstPort uint16, l4Proto string, payloadLen uint64, err error) {
	packet := gopacket.NewPacket(pkt.Data, layers.LayerTypeEthernet, gopacket.NoCopy)

	// Extract IP layer
	if ipv4Layer := packet.Layer(layers.LayerTypeIPv4); ipv4Layer != nil {
		ipv4, _ := ipv4Layer.(*layers.IPv4)
		if ipv4 != nil {
			srcIP = ipv4.SrcIP.String()
			dstIP = ipv4.DstIP.String()
			payloadLen = uint64(ipv4.Length)
		}
	} else if ipv6Layer := packet.Layer(layers.LayerTypeIPv6); ipv6Layer != nil {
		ipv6, _ := ipv6Layer.(*layers.IPv6)
		if ipv6 != nil {
			srcIP = ipv6.SrcIP.String()
			dstIP = ipv6.DstIP.String()
			payloadLen = uint64(ipv6.Length)
		}
	} else {
		err = fmt.Errorf("no IP layer found")
		return
	}

	// Extract transport layer
	if tcpLayer := packet.Layer(layers.LayerTypeTCP); tcpLayer != nil {
		tcp, _ := tcpLayer.(*layers.TCP)
		if tcp != nil {
			srcPort = uint16(tcp.SrcPort)
			dstPort = uint16(tcp.DstPort)
			l4Proto = "TCP"
		}
	} else if udpLayer := packet.Layer(layers.LayerTypeUDP); udpLayer != nil {
		udp, _ := udpLayer.(*layers.UDP)
		if udp != nil {
			srcPort = uint16(udp.SrcPort)
			dstPort = uint16(udp.DstPort)
			l4Proto = "UDP"
		}
	} else {
		l4Proto = "Other"
	}

	if payloadLen == 0 {
		payloadLen = uint64(len(pkt.Data))
	}
	return
}

// Close cleans up resources
func (pd *ProtocolDetector) Close() {
	// No resources to clean up in port-based detector
}

func (pd *ProtocolDetector) lookupPort(port uint16) (Protocol, bool) {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	p, ok := pd.portMap[port]
	return p, ok
}

func buildPortMap() map[uint16]Protocol {
	return map[uint16]Protocol{
		20:    {ID: 1, Name: "FTP-Data", Category: "FileTransfer"},
		21:    {ID: 2, Name: "FTP", Category: "FileTransfer"},
		22:    {ID: 3, Name: "SSH", Category: "RemoteAccess"},
		23:    {ID: 4, Name: "Telnet", Category: "RemoteAccess"},
		25:    {ID: 5, Name: "SMTP", Category: "Email"},
		53:    {ID: 6, Name: "DNS", Category: "Network"},
		67:    {ID: 7, Name: "DHCP", Category: "Network"},
		68:    {ID: 7, Name: "DHCP", Category: "Network"},
		80:    {ID: 8, Name: "HTTP", Category: "Web"},
		110:   {ID: 9, Name: "POP3", Category: "Email"},
		123:   {ID: 10, Name: "NTP", Category: "Network"},
		143:   {ID: 11, Name: "IMAP", Category: "Email"},
		443:   {ID: 12, Name: "HTTPS", Category: "Web"},
		445:   {ID: 13, Name: "SMB", Category: "FileTransfer"},
		993:   {ID: 14, Name: "IMAPS", Category: "Email"},
		995:   {ID: 15, Name: "POP3S", Category: "Email"},
		1080:  {ID: 16, Name: "SOCKS", Category: "Proxy"},
		1194:  {ID: 17, Name: "OpenVPN", Category: "VPN"},
		1433:  {ID: 18, Name: "MSSQL", Category: "Database"},
		1723:  {ID: 19, Name: "PPTP", Category: "VPN"},
		3306:  {ID: 20, Name: "MySQL", Category: "Database"},
		3389:  {ID: 21, Name: "RDP", Category: "RemoteAccess"},
		5432:  {ID: 22, Name: "PostgreSQL", Category: "Database"},
		5900:  {ID: 23, Name: "VNC", Category: "RemoteAccess"},
		6379:  {ID: 24, Name: "Redis", Category: "Database"},
		8080:  {ID: 25, Name: "HTTP-Alt", Category: "Web"},
		8443:  {ID: 26, Name: "HTTPS-Alt", Category: "Web"},
		27017: {ID: 27, Name: "MongoDB", Category: "Database"},
	}
}
