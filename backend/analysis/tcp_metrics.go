package analysis

import (
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/netmonitor/backend/capture"
)

// TCPMetrics tracks TCP performance metrics for a single Flow
type TCPMetrics struct {
	RTT             float64 `json:"rtt_ms"`
	MinRTT          float64 `json:"min_rtt_ms"`
	MaxRTT          float64 `json:"max_rtt_ms"`
	AvgRTT          float64 `json:"avg_rtt_ms"`
	Retransmissions int     `json:"retransmissions"`
	OutOfOrder      int     `json:"out_of_order"`
	PacketLoss      int     `json:"packet_loss"`
	AvgWindowSize   int     `json:"avg_window_size"`
	MaxWindowSize   int     `json:"max_window_size"`

	// Internal tracking state (not stored to DB)
	synTime         time.Time
	synAckTime      time.Time
	hasSyn          bool
	lastSeqNumbers  map[uint32]time.Time // sequence number -> first seen time
	expectedNextSeq map[string]uint32    // direction -> expected next seq
	windowSizes     []int
	rttSamples      []float64
	pendingAcks     map[uint32]time.Time // seq -> time sent (for DATA-ACK RTT)
}

// NewTCPMetrics creates a new TCPMetrics instance
func NewTCPMetrics() *TCPMetrics {
	return &TCPMetrics{
		lastSeqNumbers:  make(map[uint32]time.Time),
		expectedNextSeq: make(map[string]uint32),
		windowSizes:     make([]int, 0, 100),
		rttSamples:      make([]float64, 0, 100),
		pendingAcks:     make(map[uint32]time.Time),
	}
}

// TCPMetricsTracker is the global TCP metrics tracker
type TCPMetricsTracker struct {
	mu      sync.RWMutex
	metrics map[string]*TCPMetrics // flowID -> metrics
}

// NewTCPMetricsTracker creates a new TCPMetricsTracker
func NewTCPMetricsTracker() *TCPMetricsTracker {
	return &TCPMetricsTracker{
		metrics: make(map[string]*TCPMetrics),
	}
}

// ProcessPacket processes a captured packet and updates TCP metrics
func (t *TCPMetricsTracker) ProcessPacket(flowID string, pkt capture.Packet) {
	packet := gopacket.NewPacket(pkt.Data, layers.LayerTypeEthernet, gopacket.NoCopy)

	tcpLayer := packet.Layer(layers.LayerTypeTCP)
	if tcpLayer == nil {
		return
	}
	tcp, ok := tcpLayer.(*layers.TCP)
	if !ok || tcp == nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	m, exists := t.metrics[flowID]
	if !exists {
		m = NewTCPMetrics()
		t.metrics[flowID] = m
	}

	now := pkt.Timestamp
	if now.IsZero() {
		now = time.Now()
	}

	// Track window size
	windowSize := int(tcp.Window)
	m.trackWindowSize(windowSize)

	// RTT calculation based on SYN / SYN-ACK
	if tcp.SYN && !tcp.ACK {
		// SYN packet
		m.synTime = now
		m.hasSyn = true
	} else if tcp.SYN && tcp.ACK {
		// SYN-ACK packet
		if m.hasSyn && !m.synTime.IsZero() {
			rtt := float64(now.Sub(m.synTime).Microseconds()) / 1000.0 // ms
			if rtt > 0 {
				m.RTT = rtt
				m.addRTTSample(rtt)
			}
		}
	}

	seq := tcp.Seq
	ack := tcp.Ack
	payloadLen := len(tcp.Payload)

	// Track DATA-ACK RTT: when we see data, record seq; when we see ACK for it, measure RTT
	if payloadLen > 0 {
		endSeq := seq + uint32(payloadLen)
		m.pendingAcks[endSeq] = now
		// Limit pending acks map size
		if len(m.pendingAcks) > 1000 {
			for k := range m.pendingAcks {
				delete(m.pendingAcks, k)
				break
			}
		}
	}

	// Check ACK for RTT measurement
	if tcp.ACK && !tcp.SYN {
		if sentTime, ok := m.pendingAcks[ack]; ok {
			rtt := float64(now.Sub(sentTime).Microseconds()) / 1000.0
			if rtt > 0 && rtt < 30000 { // sanity check: < 30s
				m.addRTTSample(rtt)
			}
			delete(m.pendingAcks, ack)
		}
	}

	// Retransmission detection: same sequence number seen before with payload
	if payloadLen > 0 {
		if firstSeen, exists := m.lastSeqNumbers[seq]; exists {
			// Same seq seen before - if enough time has passed, it's a retransmission
			if now.Sub(firstSeen) > time.Millisecond {
				m.Retransmissions++
			}
		} else {
			m.lastSeqNumbers[seq] = now
		}

		// Limit map size to prevent memory leak
		if len(m.lastSeqNumbers) > 5000 {
			// Remove oldest entries
			for k := range m.lastSeqNumbers {
				delete(m.lastSeqNumbers, k)
				if len(m.lastSeqNumbers) <= 4000 {
					break
				}
			}
		}
	}

	// Out-of-order detection
	// Use source IP + port as direction key to track expected sequence per direction
	dirKey := "fwd"
	if tcp.SrcPort > tcp.DstPort {
		dirKey = "rev"
	}

	if payloadLen > 0 {
		expectedNext, hasExpected := m.expectedNextSeq[dirKey]
		if hasExpected && seq != 0 && expectedNext != 0 {
			if seq < expectedNext && (expectedNext-seq) < 0x80000000 {
				// seq is less than expected and not a wrap-around
				// Check it's not a retransmission (already counted)
				if _, isRetrans := m.lastSeqNumbers[seq]; !isRetrans {
					m.OutOfOrder++
				}
			}
		}
		nextSeq := seq + uint32(payloadLen)
		if !hasExpected || nextSeq > expectedNext || (expectedNext-nextSeq) > 0x80000000 {
			m.expectedNextSeq[dirKey] = nextSeq
		}
	}

	// Estimate packet loss from retransmissions (simplified)
	m.PacketLoss = m.Retransmissions
}

// GetMetrics returns a copy of TCPMetrics for a given flowID
func (t *TCPMetricsTracker) GetMetrics(flowID string) *TCPMetrics {
	t.mu.RLock()
	defer t.mu.RUnlock()

	m, exists := t.metrics[flowID]
	if !exists {
		return nil
	}

	// Return a copy with computed fields
	return &TCPMetrics{
		RTT:             m.RTT,
		MinRTT:          m.MinRTT,
		MaxRTT:          m.MaxRTT,
		AvgRTT:          m.AvgRTT,
		Retransmissions: m.Retransmissions,
		OutOfOrder:      m.OutOfOrder,
		PacketLoss:      m.PacketLoss,
		AvgWindowSize:   m.AvgWindowSize,
		MaxWindowSize:   m.MaxWindowSize,
	}
}

// Cleanup removes metrics for a given flowID
func (t *TCPMetricsTracker) Cleanup(flowID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.metrics, flowID)
}

// addRTTSample adds an RTT sample and updates min/max/avg
func (m *TCPMetrics) addRTTSample(rtt float64) {
	m.rttSamples = append(m.rttSamples, rtt)

	// Keep at most 200 samples
	if len(m.rttSamples) > 200 {
		m.rttSamples = m.rttSamples[len(m.rttSamples)-200:]
	}

	// Update min
	if m.MinRTT == 0 || rtt < m.MinRTT {
		m.MinRTT = rtt
	}
	// Update max
	if rtt > m.MaxRTT {
		m.MaxRTT = rtt
	}
	// Update avg
	sum := 0.0
	for _, s := range m.rttSamples {
		sum += s
	}
	m.AvgRTT = sum / float64(len(m.rttSamples))
}

// trackWindowSize tracks TCP window sizes and updates avg/max
func (m *TCPMetrics) trackWindowSize(windowSize int) {
	m.windowSizes = append(m.windowSizes, windowSize)

	// Keep at most 200 samples
	if len(m.windowSizes) > 200 {
		m.windowSizes = m.windowSizes[len(m.windowSizes)-200:]
	}

	// Update max
	if windowSize > m.MaxWindowSize {
		m.MaxWindowSize = windowSize
	}

	// Update avg
	sum := 0
	for _, w := range m.windowSizes {
		sum += w
	}
	m.AvgWindowSize = sum / len(m.windowSizes)
}
