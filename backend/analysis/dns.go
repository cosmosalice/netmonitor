package analysis

import (
	"fmt"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/google/gopacket/layers"
)

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// DNSSummary provides an overview of DNS activity.
type DNSSummary struct {
	TotalQueries   int64   `json:"total_queries"`
	TotalResponses int64   `json:"total_responses"`
	UniqueDomains  int     `json:"unique_domains"`
	UniqueServers  int     `json:"unique_servers"`
	NXDomainCount  int64   `json:"nxdomain_count"`
	NXDomainPct    float64 `json:"nxdomain_pct"`
}

// DomainStats holds per-domain statistics.
type DomainStats struct {
	Domain      string   `json:"domain"`
	QueryCount  int64    `json:"query_count"`
	FirstSeen   string   `json:"first_seen"`
	LastSeen    string   `json:"last_seen"`
	QueryTypes  []string `json:"query_types"`
	ResponseIPs []string `json:"response_ips"`
}

// DNSServerStats holds per-DNS-server statistics.
type DNSServerStats struct {
	IP         string  `json:"ip"`
	QueryCount int64   `json:"query_count"`
	AvgLatency float64 `json:"avg_latency_ms"`
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

type domainRecord struct {
	domain      string
	queryCount  int64
	firstSeen   time.Time
	lastSeen    time.Time
	queryTypes  map[string]struct{}
	responseIPs map[string]struct{}
}

type serverRecord struct {
	ip         string
	queryCount int64
}

// ---------------------------------------------------------------------------
// DNSStats — the main in-memory DNS statistics tracker
// ---------------------------------------------------------------------------

// DNSStats collects DNS query/response statistics from captured packets.
type DNSStats struct {
	mu             sync.RWMutex
	queryDomains   map[string]*domainRecord // domain -> record
	dnsServers     map[string]*serverRecord // server IP -> record
	responseCodes  map[string]int64         // NOERROR / NXDOMAIN / SERVFAIL …
	queryTypes     map[string]int64         // A / AAAA / MX / CNAME …
	totalQueries   int64
	totalResponses int64
}

// NewDNSStats creates a new, ready-to-use DNSStats tracker.
func NewDNSStats() *DNSStats {
	return &DNSStats{
		queryDomains:  make(map[string]*domainRecord),
		dnsServers:    make(map[string]*serverRecord),
		responseCodes: make(map[string]int64),
		queryTypes:    make(map[string]int64),
	}
}

// ---------------------------------------------------------------------------
// Packet processing
// ---------------------------------------------------------------------------

// ProcessDNSPacket analyses a single DNS layer extracted from a captured packet.
//
//   - srcIP / dstIP are the IP-layer addresses (used to determine DNS server).
//   - dnsLayer is the parsed gopacket DNS layer.
func (d *DNSStats) ProcessDNSPacket(dnsLayer *layers.DNS, srcIP, dstIP string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()

	if dnsLayer.QR { // Response
		d.totalResponses++

		// Response code
		rcName := dnsResponseCodeName(dnsLayer.ResponseCode)
		d.responseCodes[rcName]++

		// The source of a response is the DNS server
		srv := d.dnsServers[srcIP]
		if srv == nil {
			srv = &serverRecord{ip: srcIP}
			d.dnsServers[srcIP] = srv
		}
		srv.queryCount++

		// Extract domain from questions (if present)
		var domain string
		if len(dnsLayer.Questions) > 0 {
			domain = string(dnsLayer.Questions[0].Name)
		}

		if domain != "" {
			rec := d.getOrCreateDomain(domain, now)
			// Collect answered IPs (A / AAAA)
			for _, ans := range dnsLayer.Answers {
				if ans.IP != nil {
					rec.responseIPs[ans.IP.String()] = struct{}{}
				}
			}
		}
	} else { // Query
		d.totalQueries++

		// The destination of a query is the DNS server
		srv := d.dnsServers[dstIP]
		if srv == nil {
			srv = &serverRecord{ip: dstIP}
			d.dnsServers[dstIP] = srv
		}
		srv.queryCount++

		for _, q := range dnsLayer.Questions {
			domain := string(q.Name)
			if domain == "" {
				continue
			}
			rec := d.getOrCreateDomain(domain, now)
			rec.queryCount++

			typeName := dnsTypeName(q.Type)
			rec.queryTypes[typeName] = struct{}{}
			d.queryTypes[typeName]++
		}
	}
}

// getOrCreateDomain returns an existing domain record or creates a new one.
func (d *DNSStats) getOrCreateDomain(domain string, now time.Time) *domainRecord {
	rec, ok := d.queryDomains[domain]
	if !ok {
		rec = &domainRecord{
			domain:      domain,
			firstSeen:   now,
			lastSeen:    now,
			queryTypes:  make(map[string]struct{}),
			responseIPs: make(map[string]struct{}),
		}
		d.queryDomains[domain] = rec
	}
	rec.lastSeen = now
	return rec
}

// ---------------------------------------------------------------------------
// Query helpers
// ---------------------------------------------------------------------------

// GetTopDomains returns the top N domains by query count, descending.
func (d *DNSStats) GetTopDomains(limit int) []DomainStats {
	d.mu.RLock()
	defer d.mu.RUnlock()

	list := make([]DomainStats, 0, len(d.queryDomains))
	for _, rec := range d.queryDomains {
		types := make([]string, 0, len(rec.queryTypes))
		for t := range rec.queryTypes {
			types = append(types, t)
		}
		ips := make([]string, 0, len(rec.responseIPs))
		for ip := range rec.responseIPs {
			ips = append(ips, ip)
		}
		list = append(list, DomainStats{
			Domain:      rec.domain,
			QueryCount:  rec.queryCount,
			FirstSeen:   rec.firstSeen.Format(time.RFC3339),
			LastSeen:    rec.lastSeen.Format(time.RFC3339),
			QueryTypes:  types,
			ResponseIPs: ips,
		})
	}

	sort.Slice(list, func(i, j int) bool { return list[i].QueryCount > list[j].QueryCount })

	if limit > 0 && limit < len(list) {
		list = list[:limit]
	}
	return list
}

// GetTopDNSServers returns the top N DNS servers by query count, descending.
func (d *DNSStats) GetTopDNSServers(limit int) []DNSServerStats {
	d.mu.RLock()
	defer d.mu.RUnlock()

	list := make([]DNSServerStats, 0, len(d.dnsServers))
	for _, rec := range d.dnsServers {
		list = append(list, DNSServerStats{
			IP:         rec.ip,
			QueryCount: rec.queryCount,
		})
	}

	sort.Slice(list, func(i, j int) bool { return list[i].QueryCount > list[j].QueryCount })

	if limit > 0 && limit < len(list) {
		list = list[:limit]
	}
	return list
}

// GetResponseCodeStats returns a copy of the response-code counters.
func (d *DNSStats) GetResponseCodeStats() map[string]int64 {
	d.mu.RLock()
	defer d.mu.RUnlock()

	out := make(map[string]int64, len(d.responseCodes))
	for k, v := range d.responseCodes {
		out[k] = v
	}
	return out
}

// GetQueryTypeStats returns a copy of the query-type counters.
func (d *DNSStats) GetQueryTypeStats() map[string]int64 {
	d.mu.RLock()
	defer d.mu.RUnlock()

	out := make(map[string]int64, len(d.queryTypes))
	for k, v := range d.queryTypes {
		out[k] = v
	}
	return out
}

// GetSummary returns an overall DNS summary.
func (d *DNSStats) GetSummary() *DNSSummary {
	d.mu.RLock()
	defer d.mu.RUnlock()

	nxCount := d.responseCodes["NXDOMAIN"]
	var nxPct float64
	if d.totalResponses > 0 {
		nxPct = float64(nxCount) / float64(d.totalResponses) * 100
	}

	return &DNSSummary{
		TotalQueries:   d.totalQueries,
		TotalResponses: d.totalResponses,
		UniqueDomains:  len(d.queryDomains),
		UniqueServers:  len(d.dnsServers),
		NXDomainCount:  nxCount,
		NXDomainPct:    nxPct,
	}
}

// ---------------------------------------------------------------------------
// DNS enum → string helpers
// ---------------------------------------------------------------------------

func dnsTypeName(t layers.DNSType) string {
	switch t {
	case layers.DNSTypeA:
		return "A"
	case layers.DNSTypeAAAA:
		return "AAAA"
	case layers.DNSTypeCNAME:
		return "CNAME"
	case layers.DNSTypeMX:
		return "MX"
	case layers.DNSTypeTXT:
		return "TXT"
	case layers.DNSTypeSRV:
		return "SRV"
	case layers.DNSTypeNS:
		return "NS"
	case layers.DNSTypeSOA:
		return "SOA"
	case layers.DNSTypePTR:
		return "PTR"
	default:
		return fmt.Sprintf("TYPE%d", int(t))
	}
}

func dnsResponseCodeName(rc layers.DNSResponseCode) string {
	switch rc {
	case layers.DNSResponseCodeNoErr:
		return "NOERROR"
	case layers.DNSResponseCodeFormErr:
		return "FORMERR"
	case layers.DNSResponseCodeServFail:
		return "SERVFAIL"
	case layers.DNSResponseCodeNXDomain:
		return "NXDOMAIN"
	case layers.DNSResponseCodeNotImp:
		return "NOTIMP"
	case layers.DNSResponseCodeRefused:
		return "REFUSED"
	default:
		return fmt.Sprintf("RCODE%d", int(rc))
	}
}

// IsPrivateDNSIP checks if the IP is a well-known private DNS (not exported, kept as utility).
func IsPrivateDNSIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	_, loopback, _ := net.ParseCIDR("127.0.0.0/8")
	return loopback.Contains(ip)
}
