package analysis

import (
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/netmonitor/backend/capture"
)

// SNIStats tracks statistics for a single SNI domain
type SNIStats struct {
	Domain    string    `json:"domain"`
	Count     int64     `json:"count"`
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
}

// JA3Stats tracks statistics for a single JA3 hash
type JA3Stats struct {
	Hash       string   `json:"hash"`
	Count      int64    `json:"count"`
	UserAgents []string `json:"user_agents,omitempty"`
}

// TLSSummary returns TLS summary statistics
type TLSSummary struct {
	TotalHandshakes int64            `json:"total_handshakes"`
	UniqueSNI       int              `json:"unique_sni"`
	UniqueJA3       int              `json:"unique_ja3"`
	TLSVersions     map[string]int64 `json:"tls_versions"`
}

// TLSStats tracks TLS handshake statistics
type TLSStats struct {
	mu              sync.RWMutex
	sniDomains      map[string]*SNIStats
	ja3Hashes       map[string]*JA3Stats
	tlsVersions     map[string]int64
	totalHandshakes int64
}

// NewTLSStats creates a new TLS stats tracker
func NewTLSStats() *TLSStats {
	return &TLSStats{
		sniDomains:  make(map[string]*SNIStats),
		ja3Hashes:   make(map[string]*JA3Stats),
		tlsVersions: make(map[string]int64),
	}
}

// IsTLSClientHello checks if TCP payload starts with a TLS ClientHello
func IsTLSClientHello(payload []byte) bool {
	if len(payload) < 6 {
		return false
	}
	// TLS Record: ContentType=Handshake(0x16), Version=0x03xx
	// Handshake: Type=ClientHello(0x01)
	return payload[0] == 0x16 &&
		payload[1] == 0x03 &&
		payload[5] == 0x01
}

// ProcessPacket analyzes a TCP packet for TLS ClientHello
func (t *TLSStats) ProcessPacket(pkt capture.Packet) {
	packet := gopacket.NewPacket(pkt.Data, layers.LayerTypeEthernet, gopacket.NoCopy)
	tcpLayer := packet.Layer(layers.LayerTypeTCP)
	if tcpLayer == nil {
		return
	}
	tcp, _ := tcpLayer.(*layers.TCP)
	payload := tcp.Payload
	if !IsTLSClientHello(payload) {
		return
	}

	t.parseClientHello(payload)
}

func (t *TLSStats) parseClientHello(data []byte) {
	// TLS Record Header (5 bytes)
	if len(data) < 5 {
		return
	}
	recordLen := int(binary.BigEndian.Uint16(data[3:5]))
	if len(data) < 5+recordLen {
		// Truncated, work with what we have
		if len(data) < 44 {
			return
		}
	}

	handshake := data[5:]
	if len(handshake) < 39 {
		return
	}

	// Handshake header: type(1) + length(3) = 4 bytes
	// handshake[0] = 0x01 (ClientHello)
	// handshake[1:4] = length (3 bytes)

	// ClientHello body starts at offset 4
	hello := handshake[4:]
	if len(hello) < 35 {
		return
	}

	// Client version (2 bytes)
	tlsVersion := binary.BigEndian.Uint16(hello[0:2])
	versionStr := tlsVersionString(tlsVersion)

	// Random (32 bytes) -> skip
	// Session ID length at offset 34
	pos := 34
	if pos >= len(hello) {
		return
	}
	sessionIDLen := int(hello[pos])
	pos += 1 + sessionIDLen

	// Cipher suites
	if pos+2 > len(hello) {
		return
	}
	cipherSuitesLen := int(binary.BigEndian.Uint16(hello[pos : pos+2]))
	pos += 2
	if pos+cipherSuitesLen > len(hello) {
		cipherSuitesLen = len(hello) - pos
	}

	cipherSuites := make([]uint16, 0, cipherSuitesLen/2)
	for i := 0; i+1 < cipherSuitesLen; i += 2 {
		cs := binary.BigEndian.Uint16(hello[pos+i : pos+i+2])
		// Skip GREASE values
		if !isGREASE(cs) {
			cipherSuites = append(cipherSuites, cs)
		}
	}
	pos += cipherSuitesLen

	// Compression methods
	if pos >= len(hello) {
		return
	}
	compMethodsLen := int(hello[pos])
	pos += 1 + compMethodsLen

	// Extensions
	if pos+2 > len(hello) {
		return
	}
	extensionsLen := int(binary.BigEndian.Uint16(hello[pos : pos+2]))
	pos += 2

	var sni string
	extensionTypes := make([]uint16, 0, 20)
	var ellipticCurves []uint16
	var ecPointFormats []uint8

	extEnd := pos + extensionsLen
	if extEnd > len(hello) {
		extEnd = len(hello)
	}

	for pos+4 <= extEnd {
		extType := binary.BigEndian.Uint16(hello[pos : pos+2])
		extLen := int(binary.BigEndian.Uint16(hello[pos+2 : pos+4]))
		pos += 4

		if pos+extLen > extEnd {
			break
		}

		if !isGREASE(extType) {
			extensionTypes = append(extensionTypes, extType)
		}

		extData := hello[pos : pos+extLen]

		switch extType {
		case 0x0000: // server_name (SNI)
			sni = parseSNI(extData)

		case 0x000a: // supported_groups (elliptic_curves)
			if len(extData) >= 2 {
				listLen := int(binary.BigEndian.Uint16(extData[0:2]))
				for i := 2; i+1 < 2+listLen && i+1 < len(extData); i += 2 {
					curve := binary.BigEndian.Uint16(extData[i : i+2])
					if !isGREASE(curve) {
						ellipticCurves = append(ellipticCurves, curve)
					}
				}
			}

		case 0x000b: // ec_point_formats
			if len(extData) >= 1 {
				fmtLen := int(extData[0])
				for i := 1; i < 1+fmtLen && i < len(extData); i++ {
					ecPointFormats = append(ecPointFormats, extData[i])
				}
			}
		}

		pos += extLen
	}

	// Compute JA3
	ja3Hash := computeJA3(tlsVersion, cipherSuites, extensionTypes, ellipticCurves, ecPointFormats)

	// Update stats
	t.mu.Lock()
	defer t.mu.Unlock()

	t.totalHandshakes++
	t.tlsVersions[versionStr]++

	if sni != "" {
		stats, ok := t.sniDomains[sni]
		if !ok {
			stats = &SNIStats{
				Domain:    sni,
				FirstSeen: time.Now(),
			}
			t.sniDomains[sni] = stats
		}
		stats.Count++
		stats.LastSeen = time.Now()
	}

	if ja3Hash != "" {
		stats, ok := t.ja3Hashes[ja3Hash]
		if !ok {
			stats = &JA3Stats{Hash: ja3Hash}
			t.ja3Hashes[ja3Hash] = stats
		}
		stats.Count++
	}
}

func parseSNI(data []byte) string {
	// SNI extension data:
	// ServerNameList length (2 bytes)
	// NameType (1 byte, 0x00 = hostname)
	// HostName length (2 bytes)
	// HostName (variable)
	if len(data) < 5 {
		return ""
	}
	// listLen := binary.BigEndian.Uint16(data[0:2])
	nameType := data[2]
	if nameType != 0x00 {
		return ""
	}
	nameLen := int(binary.BigEndian.Uint16(data[3:5]))
	if 5+nameLen > len(data) {
		return ""
	}
	return string(data[5 : 5+nameLen])
}

func computeJA3(version uint16, ciphers, extensions []uint16, curves []uint16, pointFormats []uint8) string {
	// JA3 = MD5(TLSVersion,Ciphers,Extensions,EllipticCurves,EllipticCurvePointFormats)
	// Each list is joined by "-", lists are joined by ","

	parts := make([]string, 5)
	parts[0] = fmt.Sprintf("%d", version)

	cipherStrs := make([]string, len(ciphers))
	for i, c := range ciphers {
		cipherStrs[i] = fmt.Sprintf("%d", c)
	}
	parts[1] = strings.Join(cipherStrs, "-")

	extStrs := make([]string, len(extensions))
	for i, e := range extensions {
		extStrs[i] = fmt.Sprintf("%d", e)
	}
	parts[2] = strings.Join(extStrs, "-")

	curveStrs := make([]string, len(curves))
	for i, c := range curves {
		curveStrs[i] = fmt.Sprintf("%d", c)
	}
	parts[3] = strings.Join(curveStrs, "-")

	pointStrs := make([]string, len(pointFormats))
	for i, p := range pointFormats {
		pointStrs[i] = fmt.Sprintf("%d", p)
	}
	parts[4] = strings.Join(pointStrs, "-")

	ja3String := strings.Join(parts, ",")
	hash := md5.Sum([]byte(ja3String))
	return fmt.Sprintf("%x", hash)
}

func isGREASE(val uint16) bool {
	// GREASE values: 0x0a0a, 0x1a1a, ..., 0xfafa
	return (val & 0x0f0f) == 0x0a0a
}

func tlsVersionString(ver uint16) string {
	switch ver {
	case 0x0301:
		return "TLS 1.0"
	case 0x0302:
		return "TLS 1.1"
	case 0x0303:
		return "TLS 1.2"
	case 0x0304:
		return "TLS 1.3"
	case 0x0300:
		return "SSL 3.0"
	default:
		return fmt.Sprintf("Unknown (0x%04x)", ver)
	}
}

// GetSummary returns TLS summary statistics
func (t *TLSStats) GetSummary() TLSSummary {
	t.mu.RLock()
	defer t.mu.RUnlock()

	versions := make(map[string]int64, len(t.tlsVersions))
	for k, v := range t.tlsVersions {
		versions[k] = v
	}

	return TLSSummary{
		TotalHandshakes: t.totalHandshakes,
		UniqueSNI:       len(t.sniDomains),
		UniqueJA3:       len(t.ja3Hashes),
		TLSVersions:     versions,
	}
}

// GetTopSNI returns top SNI domains by count
func (t *TLSStats) GetTopSNI(limit int) []*SNIStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	all := make([]*SNIStats, 0, len(t.sniDomains))
	for _, s := range t.sniDomains {
		cp := *s
		all = append(all, &cp)
	}

	// Sort by count descending
	for i := 0; i < len(all); i++ {
		for j := i + 1; j < len(all); j++ {
			if all[j].Count > all[i].Count {
				all[i], all[j] = all[j], all[i]
			}
		}
	}

	if limit > 0 && limit < len(all) {
		all = all[:limit]
	}
	return all
}

// GetTopJA3 returns top JA3 hashes by count
func (t *TLSStats) GetTopJA3(limit int) []*JA3Stats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	all := make([]*JA3Stats, 0, len(t.ja3Hashes))
	for _, s := range t.ja3Hashes {
		cp := *s
		all = append(all, &cp)
	}

	for i := 0; i < len(all); i++ {
		for j := i + 1; j < len(all); j++ {
			if all[j].Count > all[i].Count {
				all[i], all[j] = all[j], all[i]
			}
		}
	}

	if limit > 0 && limit < len(all) {
		all = all[:limit]
	}
	return all
}

// GetTLSVersions returns TLS version distribution
func (t *TLSStats) GetTLSVersions() map[string]int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	result := make(map[string]int64, len(t.tlsVersions))
	for k, v := range t.tlsVersions {
		result[k] = v
	}
	return result
}
