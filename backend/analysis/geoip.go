package analysis

import (
	"log"
	"net"
	"sync"

	"github.com/oschwald/maxminddb-golang"
)

// GeoInfo 地理位置信息
type GeoInfo struct {
	Country   string  `json:"country"`
	City      string  `json:"city"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	ASN       uint    `json:"asn"`
	ASOrg     string  `json:"as_org"`
}

// GeoIPManager GeoIP 查询管理器
type GeoIPManager struct {
	mu      sync.RWMutex
	cityDB  *maxminddb.Reader
	asnDB   *maxminddb.Reader
	cache   map[string]*GeoInfo // IP -> GeoInfo 缓存
	enabled bool
}

// MaxMind DB 查询结构
type cityRecord struct {
	Country struct {
		ISOCode string            `maxminddb:"iso_code"`
		Names   map[string]string `maxminddb:"names"`
	} `maxminddb:"country"`
	City struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"city"`
	Location struct {
		Latitude  float64 `maxminddb:"latitude"`
		Longitude float64 `maxminddb:"longitude"`
	} `maxminddb:"location"`
}

type asnRecord struct {
	AutonomousSystemNumber       uint   `maxminddb:"autonomous_system_number"`
	AutonomousSystemOrganization string `maxminddb:"autonomous_system_organization"`
}

// 内置 IP 段-国家映射回退数据
var builtinGeoData = []struct {
	cidr    string
	country string
}{
	// North America
	{"8.0.0.0/8", "US"},
	{"8.8.8.0/24", "US"},     // Google DNS
	{"8.8.4.0/24", "US"},     // Google DNS
	{"1.1.1.0/24", "US"},     // Cloudflare
	{"15.0.0.0/8", "US"},     // HP
	{"16.0.0.0/8", "US"},     // DEC
	{"17.0.0.0/8", "US"},     // Apple
	{"20.0.0.0/8", "US"},     // Microsoft
	{"32.0.0.0/8", "US"},     // AT&T
	{"34.0.0.0/8", "US"},     // Halliburton
	{"35.0.0.0/8", "US"},     // Google
	{"44.0.0.0/8", "US"},     // Amazon
	{"52.0.0.0/8", "US"},     // Amazon AWS
	{"54.0.0.0/8", "US"},     // Amazon AWS
	{"63.0.0.0/8", "US"},     // ARIN
	{"64.0.0.0/8", "US"},     // ARIN
	{"65.0.0.0/8", "US"},     // ARIN
	{"66.0.0.0/8", "US"},     // ARIN
	{"67.0.0.0/8", "US"},     // ARIN
	{"68.0.0.0/8", "US"},     // ARIN
	{"69.0.0.0/8", "US"},     // ARIN
	{"70.0.0.0/8", "US"},     // ARIN
	{"71.0.0.0/8", "US"},     // ARIN
	{"72.0.0.0/8", "US"},     // ARIN
	{"73.0.0.0/8", "US"},     // ARIN
	{"74.0.0.0/8", "US"},     // ARIN
	{"76.0.0.0/8", "US"},     // ARIN
	{"96.0.0.0/8", "US"},     // ARIN
	{"104.0.0.0/8", "US"},    // ARIN
	{"142.0.0.0/8", "CA"},    // Canada
	{"24.0.0.0/8", "US"},     // Cable TV
	{"184.0.0.0/8", "US"},    // ARIN
	{"199.0.0.0/8", "US"},    // ARIN
	{"204.0.0.0/8", "US"},    // ARIN
	{"205.0.0.0/8", "US"},    // ARIN
	{"206.0.0.0/8", "US"},    // ARIN
	{"207.0.0.0/8", "US"},    // ARIN
	{"208.0.0.0/8", "US"},    // ARIN
	{"209.0.0.0/8", "US"},    // ARIN
	{"216.0.0.0/8", "US"},    // ARIN
	{"172.217.0.0/16", "US"}, // Google

	// Europe
	{"2.0.0.0/8", "EU"},   // RIPE
	{"5.0.0.0/8", "EU"},   // RIPE
	{"31.0.0.0/8", "EU"},  // RIPE
	{"37.0.0.0/8", "EU"},  // RIPE
	{"46.0.0.0/8", "EU"},  // RIPE
	{"62.0.0.0/8", "EU"},  // RIPE
	{"77.0.0.0/8", "EU"},  // RIPE
	{"78.0.0.0/8", "EU"},  // RIPE
	{"79.0.0.0/8", "EU"},  // RIPE
	{"80.0.0.0/8", "EU"},  // RIPE
	{"81.0.0.0/8", "EU"},  // RIPE
	{"82.0.0.0/8", "EU"},  // RIPE
	{"83.0.0.0/8", "EU"},  // RIPE
	{"84.0.0.0/8", "EU"},  // RIPE
	{"85.0.0.0/8", "EU"},  // RIPE
	{"86.0.0.0/8", "EU"},  // RIPE
	{"87.0.0.0/8", "EU"},  // RIPE
	{"88.0.0.0/8", "EU"},  // RIPE
	{"89.0.0.0/8", "EU"},  // RIPE
	{"90.0.0.0/8", "EU"},  // RIPE
	{"91.0.0.0/8", "EU"},  // RIPE
	{"92.0.0.0/8", "EU"},  // RIPE
	{"93.0.0.0/8", "EU"},  // RIPE
	{"94.0.0.0/8", "EU"},  // RIPE
	{"95.0.0.0/8", "EU"},  // RIPE
	{"109.0.0.0/8", "EU"}, // RIPE
	{"151.0.0.0/8", "EU"}, // RIPE
	{"176.0.0.0/8", "EU"}, // RIPE
	{"178.0.0.0/8", "EU"}, // RIPE
	{"185.0.0.0/8", "EU"}, // RIPE
	{"188.0.0.0/8", "EU"}, // RIPE
	{"193.0.0.0/8", "EU"}, // RIPE
	{"194.0.0.0/8", "EU"}, // RIPE
	{"195.0.0.0/8", "EU"}, // RIPE
	{"212.0.0.0/8", "EU"}, // RIPE
	{"213.0.0.0/8", "EU"}, // RIPE
	{"217.0.0.0/8", "EU"}, // RIPE

	// Asia-Pacific
	{"1.0.0.0/8", "AU"},   // APNIC
	{"14.0.0.0/8", "JP"},  // APNIC - Japan
	{"27.0.0.0/8", "CN"},  // APNIC - China
	{"36.0.0.0/8", "CN"},  // APNIC - China
	{"39.0.0.0/8", "CN"},  // APNIC - China
	{"42.0.0.0/8", "CN"},  // APNIC - China
	{"49.0.0.0/8", "CN"},  // APNIC - China
	{"58.0.0.0/8", "CN"},  // APNIC - China
	{"59.0.0.0/8", "CN"},  // APNIC - China
	{"60.0.0.0/8", "CN"},  // APNIC - China
	{"61.0.0.0/8", "CN"},  // APNIC - China
	{"101.0.0.0/8", "CN"}, // APNIC - China
	{"106.0.0.0/8", "CN"}, // APNIC - China
	{"110.0.0.0/8", "CN"}, // APNIC - China
	{"111.0.0.0/8", "CN"}, // APNIC - China
	{"112.0.0.0/8", "CN"}, // APNIC - China
	{"113.0.0.0/8", "CN"}, // APNIC - China
	{"114.0.0.0/8", "CN"}, // APNIC - China
	{"115.0.0.0/8", "CN"}, // APNIC - China
	{"116.0.0.0/8", "CN"}, // APNIC - China
	{"117.0.0.0/8", "CN"}, // APNIC - China
	{"118.0.0.0/8", "CN"}, // APNIC - China
	{"119.0.0.0/8", "CN"}, // APNIC - China
	{"120.0.0.0/8", "CN"}, // APNIC - China
	{"121.0.0.0/8", "CN"}, // APNIC - China
	{"122.0.0.0/8", "CN"}, // APNIC - China
	{"123.0.0.0/8", "CN"}, // APNIC - China
	{"124.0.0.0/8", "CN"}, // APNIC - China
	{"125.0.0.0/8", "CN"}, // APNIC - China
	{"126.0.0.0/8", "JP"}, // APNIC - Japan
	{"133.0.0.0/8", "JP"}, // APNIC - Japan
	{"150.0.0.0/8", "JP"}, // APNIC - Japan
	{"153.0.0.0/8", "JP"}, // APNIC - Japan
	{"163.0.0.0/8", "CN"}, // APNIC - China
	{"175.0.0.0/8", "CN"}, // APNIC - China
	{"180.0.0.0/8", "CN"}, // APNIC - China
	{"182.0.0.0/8", "CN"}, // APNIC - China
	{"183.0.0.0/8", "CN"}, // APNIC - China
	{"202.0.0.0/8", "CN"}, // APNIC
	{"203.0.0.0/8", "CN"}, // APNIC
	{"210.0.0.0/8", "CN"}, // APNIC
	{"211.0.0.0/8", "KR"}, // APNIC - Korea
	{"218.0.0.0/8", "CN"}, // APNIC - China
	{"219.0.0.0/8", "CN"}, // APNIC - China
	{"220.0.0.0/8", "CN"}, // APNIC - China
	{"221.0.0.0/8", "CN"}, // APNIC - China
	{"222.0.0.0/8", "CN"}, // APNIC - China
	{"223.0.0.0/8", "CN"}, // APNIC - China

	// Latin America / Africa
	{"177.0.0.0/8", "BR"}, // LACNIC - Brazil
	{"179.0.0.0/8", "BR"}, // LACNIC - Brazil
	{"186.0.0.0/8", "BR"}, // LACNIC - Brazil
	{"187.0.0.0/8", "BR"}, // LACNIC - Brazil
	{"189.0.0.0/8", "BR"}, // LACNIC - Brazil
	{"190.0.0.0/8", "BR"}, // LACNIC
	{"196.0.0.0/8", "ZA"}, // AFRINIC - South Africa
	{"197.0.0.0/8", "ZA"}, // AFRINIC
	{"41.0.0.0/8", "ZA"},  // AFRINIC
	{"102.0.0.0/8", "ZA"}, // AFRINIC
	{"105.0.0.0/8", "ZA"}, // AFRINIC
	{"154.0.0.0/8", "ZA"}, // AFRINIC
	{"200.0.0.0/8", "BR"}, // LACNIC
	{"201.0.0.0/8", "BR"}, // LACNIC
	{"169.0.0.0/8", "IN"}, // India
	{"103.0.0.0/8", "IN"}, // APNIC - India
	{"43.0.0.0/8", "IN"},  // APNIC - India
	{"157.0.0.0/8", "RU"}, // Russia
	{"128.0.0.0/8", "US"}, // Various
	{"130.0.0.0/8", "US"}, // Various
	{"131.0.0.0/8", "US"}, // Various
	{"132.0.0.0/8", "US"}, // Various
	{"134.0.0.0/8", "US"}, // Various
	{"136.0.0.0/8", "US"}, // Various
	{"137.0.0.0/8", "US"}, // Various
	{"138.0.0.0/8", "US"}, // Various
	{"139.0.0.0/8", "US"}, // Various
	{"140.0.0.0/8", "US"}, // Various
	{"141.0.0.0/8", "US"}, // Various
	{"143.0.0.0/8", "US"}, // Various
	{"144.0.0.0/8", "US"}, // Various
	{"146.0.0.0/8", "US"}, // Various
	{"147.0.0.0/8", "US"}, // Various
	{"148.0.0.0/8", "US"}, // Various
	{"149.0.0.0/8", "US"}, // Various
	{"152.0.0.0/8", "US"}, // Various
	{"155.0.0.0/8", "US"}, // Various
	{"156.0.0.0/8", "US"}, // Various
	{"158.0.0.0/8", "US"}, // Various
	{"159.0.0.0/8", "US"}, // Various
	{"160.0.0.0/8", "US"}, // Various
	{"161.0.0.0/8", "US"}, // Various
	{"162.0.0.0/8", "US"}, // Various
	{"164.0.0.0/8", "US"}, // Various
	{"165.0.0.0/8", "US"}, // Various
	{"166.0.0.0/8", "US"}, // Various
	{"167.0.0.0/8", "US"}, // Various
	{"168.0.0.0/8", "US"}, // Various
	{"170.0.0.0/8", "US"}, // Various
	{"198.0.0.0/8", "US"}, // ARIN
}

// 解析后的内置网络列表
var builtinNets []*net.IPNet
var builtinCountries []string

func init() {
	builtinNets = make([]*net.IPNet, 0, len(builtinGeoData))
	builtinCountries = make([]string, 0, len(builtinGeoData))
	for _, entry := range builtinGeoData {
		_, ipnet, err := net.ParseCIDR(entry.cidr)
		if err != nil {
			continue
		}
		builtinNets = append(builtinNets, ipnet)
		builtinCountries = append(builtinCountries, entry.country)
	}
}

// NewGeoIPManager 创建 GeoIP 查询管理器
// 打开 MaxMind DB 文件，如果文件不存在则使用内置回退数据
func NewGeoIPManager(cityDBPath, asnDBPath string) (*GeoIPManager, error) {
	mgr := &GeoIPManager{
		cache: make(map[string]*GeoInfo),
	}

	var hasDB bool

	if cityDBPath != "" {
		db, err := maxminddb.Open(cityDBPath)
		if err != nil {
			log.Printf("GeoIP: failed to open city DB %q: %v (using builtin fallback)", cityDBPath, err)
		} else {
			mgr.cityDB = db
			hasDB = true
		}
	}

	if asnDBPath != "" {
		db, err := maxminddb.Open(asnDBPath)
		if err != nil {
			log.Printf("GeoIP: failed to open ASN DB %q: %v (using builtin fallback)", asnDBPath, err)
		} else {
			mgr.asnDB = db
			hasDB = true
		}
	}

	mgr.enabled = hasDB || len(builtinNets) > 0

	if hasDB {
		log.Println("GeoIP: initialized with MaxMind database(s)")
	} else {
		log.Println("GeoIP: using builtin fallback data (limited accuracy)")
	}

	return mgr, nil
}

// Lookup 查询 IP 的地理和 ASN 信息（先查缓存）
func (mgr *GeoIPManager) Lookup(ipStr string) *GeoInfo {
	if !mgr.enabled {
		return &GeoInfo{}
	}

	if IsPrivateIP(ipStr) {
		return &GeoInfo{}
	}

	// 查缓存
	mgr.mu.RLock()
	if info, ok := mgr.cache[ipStr]; ok {
		mgr.mu.RUnlock()
		return info
	}
	mgr.mu.RUnlock()

	// 查询
	info := mgr.lookupFromDB(ipStr)

	// 写缓存
	mgr.mu.Lock()
	mgr.cache[ipStr] = info
	mgr.mu.Unlock()

	return info
}

// lookupFromDB 从 MaxMind DB 或内置数据查询
func (mgr *GeoIPManager) lookupFromDB(ipStr string) *GeoInfo {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return &GeoInfo{}
	}

	info := &GeoInfo{}

	// 尝试 MaxMind City DB
	if mgr.cityDB != nil {
		var record cityRecord
		err := mgr.cityDB.Lookup(ip, &record)
		if err == nil {
			info.Country = record.Country.ISOCode
			if name, ok := record.City.Names["en"]; ok {
				info.City = name
			} else if name, ok := record.City.Names["zh-CN"]; ok {
				info.City = name
			}
			info.Latitude = record.Location.Latitude
			info.Longitude = record.Location.Longitude
		}
	}

	// 尝试 MaxMind ASN DB
	if mgr.asnDB != nil {
		var record asnRecord
		err := mgr.asnDB.Lookup(ip, &record)
		if err == nil {
			info.ASN = record.AutonomousSystemNumber
			info.ASOrg = record.AutonomousSystemOrganization
		}
	}

	// 如果没有从 MaxMind 获取到国家信息，使用内置回退
	if info.Country == "" {
		info.Country = lookupBuiltin(ip)
	}

	return info
}

// lookupBuiltin 从内置数据查询国家
func lookupBuiltin(ip net.IP) string {
	for i, ipnet := range builtinNets {
		if ipnet.Contains(ip) {
			return builtinCountries[i]
		}
	}
	return ""
}

// Close 关闭数据库
func (mgr *GeoIPManager) Close() {
	if mgr.cityDB != nil {
		mgr.cityDB.Close()
	}
	if mgr.asnDB != nil {
		mgr.asnDB.Close()
	}
}

// IsEnabled 是否有可用的 GeoIP 数据库
func (mgr *GeoIPManager) IsEnabled() bool {
	return mgr.enabled
}

// IsPrivateIP 判断是否为私有 IP（私有 IP 不做 GeoIP 查询）
func IsPrivateIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	// IPv4 private ranges
	privateRanges := []struct {
		start net.IP
		end   net.IP
	}{
		{net.ParseIP("10.0.0.0"), net.ParseIP("10.255.255.255")},
		{net.ParseIP("172.16.0.0"), net.ParseIP("172.31.255.255")},
		{net.ParseIP("192.168.0.0"), net.ParseIP("192.168.255.255")},
		{net.ParseIP("127.0.0.0"), net.ParseIP("127.255.255.255")},
		{net.ParseIP("169.254.0.0"), net.ParseIP("169.254.255.255")},
	}

	ip4 := ip.To4()
	if ip4 == nil {
		// IPv6 link-local
		return ip.IsLinkLocalUnicast() || ip.IsLoopback()
	}

	for _, r := range privateRanges {
		s := r.start.To4()
		e := r.end.To4()
		if s == nil || e == nil {
			continue
		}
		if bytesCompare(ip4, s) >= 0 && bytesCompare(ip4, e) <= 0 {
			return true
		}
	}

	return false
}

func bytesCompare(a, b net.IP) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}
