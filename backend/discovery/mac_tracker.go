package discovery

import (
	"database/sql"
	"encoding/json"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

// Device represents a network device discovered by MAC address
type Device struct {
	MAC        string    `json:"mac"`
	Name       string    `json:"name"`
	Vendor     string    `json:"vendor"`
	IPs        []string  `json:"ips"` // Multiple IP addresses
	FirstSeen  time.Time `json:"first_seen"`
	LastSeen   time.Time `json:"last_seen"`
	BytesSent  uint64    `json:"bytes_sent"`
	BytesRecv  uint64    `json:"bytes_recv"`
	FlowCount  uint64    `json:"flow_count"`
	DeviceType string    `json:"device_type"` // "pc", "server", "router", "phone", "iot", "unknown"
	IsOnline   bool      `json:"is_online"`
	Hostname   string    `json:"hostname"`
	OS         string    `json:"os"`
}

// MACTracker tracks devices by their MAC addresses
type MACTracker struct {
	mu      sync.RWMutex
	devices map[string]*Device // MAC -> Device
	ouiDB   map[string]string  // OUI prefix (AA:BB:CC) -> vendor name
	db      *sql.DB            // SQLite database for persistence
}

// builtinOUI contains common OUI prefixes and their vendors
// Carefully curated to avoid duplicates - about 100 major vendors
var builtinOUI = map[string]string{
	// VMware
	"00:50:56": "VMware",
	"00:0C:29": "VMware",
	"00:05:69": "VMware",
	// VirtualBox
	"08:00:27": "VirtualBox",
	// Apple (iPhone, iPad, Mac)
	"AC:DE:48": "Apple",
	"3C:22:FB": "Apple",
	"F0:18:98": "Apple",
	"F0:24:75": "Apple",
	"A4:5E:60": "Apple",
	"6C:94:F8": "Apple",
	"70:56:81": "Apple",
	"7C:6D:62": "Apple",
	"88:66:5A": "Apple",
	"98:01:A7": "Apple",
	"9C:20:7B": "Apple",
	"A8:66:7F": "Apple",
	"B8:17:C2": "Apple",
	"B8:F6:B1": "Apple",
	"BC:4C:C4": "Apple",
	"C0:63:94": "Apple",
	"C8:2A:14": "Apple",
	"C8:BC:C8": "Apple",
	"CC:25:EF": "Apple",
	"D0:23:DB": "Apple",
	"D4:F5:7F": "Apple",
	"D8:30:62": "Apple",
	"D8:9E:3F": "Apple",
	"DC:2B:2A": "Apple",
	"E0:5F:45": "Apple",
	"E4:98:D6": "Apple",
	"E4:E7:49": "Apple",
	"E8:04:62": "Apple",
	"EC:35:86": "Apple",
	"F4:37:B7": "Apple",
	"F4:5C:89": "Apple",
	"F8:27:93": "Apple",
	// Raspberry Pi
	"DC:A6:32": "Raspberry Pi",
	"B8:27:EB": "Raspberry Pi",
	"E4:5F:01": "Raspberry Pi",
	// Cisco
	"00:00:0C": "Cisco",
	"00:01:42": "Cisco",
	"00:01:43": "Cisco",
	"00:01:63": "Cisco",
	"00:01:96": "Cisco",
	"00:01:C7": "Cisco",
	"00:01:C9": "Cisco",
	"00:02:16": "Cisco",
	"00:02:17": "Cisco",
	"00:02:4A": "Cisco",
	"00:02:7D": "Cisco",
	"00:02:90": "Cisco",
	"00:02:FC": "Cisco",
	"00:03:31": "Cisco",
	"00:03:6B": "Cisco",
	"00:03:A0": "Cisco",
	"00:03:E3": "Cisco",
	"00:04:12": "Cisco",
	"00:04:27": "Cisco",
	"00:04:9A": "Cisco",
	"00:04:C0": "Cisco",
	"00:04:DD": "Cisco",
	"00:05:32": "Cisco",
	"00:05:73": "Cisco",
	"00:05:9A": "Cisco",
	"00:06:28": "Cisco",
	"00:06:52": "Cisco",
	"00:06:7C": "Cisco",
	"00:06:C1": "Cisco",
	// Juniper
	"00:05:85": "Juniper",
	"00:09:6B": "Juniper",
	"00:12:1E": "Juniper",
	"00:14:F6": "Juniper",
	"00:17:CB": "Juniper",
	"00:19:E2": "Juniper",
	"00:1B:C0": "Juniper",
	"00:1F:12": "Juniper",
	"00:21:59": "Juniper",
	"00:23:9C": "Juniper",
	"00:26:88": "Juniper",
	"00:3F:DB": "Juniper",
	// Dell
	"00:08:74": "Dell",
	"00:0B:DB": "Dell",
	"00:0F:1F": "Dell",
	"00:12:3F": "Dell",
	"00:13:72": "Dell",
	"00:14:22": "Dell",
	"00:15:C5": "Dell",
	"00:19:B9": "Dell",
	"00:1A:A0": "Dell",
	"00:1C:23": "Dell",
	"00:1D:09": "Dell",
	"00:1E:4F": "Dell",
	"00:21:9B": "Dell",
	"00:22:19": "Dell",
	"00:23:AE": "Dell",
	"00:24:E8": "Dell",
	"00:26:B9": "Dell",
	// HP
	"00:01:E6": "HP",
	"00:01:E7": "HP",
	"00:08:02": "HP",
	"00:0B:CD": "HP",
	"00:0E:7F": "HP",
	"00:0F:20": "HP",
	"00:11:0A": "HP",
	"00:12:79": "HP",
	"00:13:21": "HP",
	"00:14:38": "HP",
	"00:15:60": "HP",
	"00:16:35": "HP",
	"00:17:08": "HP",
	"00:18:71": "HP",
	"00:19:BB": "HP",
	"00:1A:4B": "HP",
	"00:1B:78": "HP",
	"00:1C:C4": "HP",
	"00:1D:9C": "HP",
	"00:1E:0B": "HP",
	"00:21:5A": "HP",
	"00:22:64": "HP",
	"00:23:7D": "HP",
	"00:24:81": "HP",
	"00:25:B3": "HP",
	"00:26:55": "HP",
	// Lenovo
	"00:0A:E4": "Lenovo",
	"00:0C:E7": "Lenovo",
	"00:0D:60": "Lenovo",
	"00:0D:FD": "Lenovo",
	"00:0E:35": "Lenovo",
	"00:0E:EC": "Lenovo",
	"00:0F:3D": "Lenovo",
	"00:10:4D": "Lenovo",
	"00:12:FE": "Lenovo",
	"00:13:92": "Lenovo",
	"00:14:85": "Lenovo",
	"00:15:58": "Lenovo",
	"00:16:41": "Lenovo",
	"00:17:A4": "Lenovo",
	"00:19:2E": "Lenovo",
	"00:1A:6B": "Lenovo",
	"00:1B:38": "Lenovo",
	"00:1C:25": "Lenovo",
	"00:1D:72": "Lenovo",
	"00:1E:37": "Lenovo",
	"00:1F:16": "Lenovo",
	"00:21:CC": "Lenovo",
	"00:22:68": "Lenovo",
	"00:23:AC": "Lenovo",
	// Samsung
	"00:00:F0": "Samsung",
	"00:07:AB": "Samsung",
	"00:12:47": "Samsung",
	"00:12:FB": "Samsung",
	"00:13:EF": "Samsung",
	"00:15:99": "Samsung",
	"00:17:C9": "Samsung",
	"00:18:AF": "Samsung",
	"00:1B:98": "Samsung",
	"00:1C:43": "Samsung",
	"00:1D:25": "Samsung",
	"00:1E:7D": "Samsung",
	"00:1F:CC": "Samsung",
	"00:21:19": "Samsung",
	"00:23:39": "Samsung",
	"00:24:54": "Samsung",
	"00:25:38": "Samsung",
	"00:26:5D": "Samsung",
	"00:26:F2": "Samsung",
	"00:37:6D": "Samsung",
	"00:3B:97": "Samsung",
	"00:3C:8A": "Samsung",
	"00:3D:AE": "Samsung",
	"00:3E:5B": "Samsung",
	"04:18:0F": "Samsung",
	"04:FE:31": "Samsung",
	"08:08:EA": "Samsung",
	"08:3D:88": "Samsung",
	"08:5B:0E": "Samsung",
	"08:F4:AB": "Samsung",
	"08:FC:88": "Samsung",
	"0C:14:20": "Samsung",
	"0C:71:5D": "Samsung",
	"0C:89:10": "Samsung",
	"0C:F3:46": "Samsung",
	"10:1D:C0": "Samsung",
	"10:30:47": "Samsung",
	"10:3B:59": "Samsung",
	"10:68:38": "Samsung",
	"10:A5:D0": "Samsung",
	"10:D5:42": "Samsung",
	"14:32:D1": "Samsung",
	"14:56:8E": "Samsung",
	"14:B4:57": "Samsung",
	"14:BB:6E": "Samsung",
	"14:F4:2A": "Samsung",
	"18:16:C9": "Samsung",
	"18:3A:2D": "Samsung",
	"18:3F:47": "Samsung",
	"18:46:44": "Samsung",
	"18:5B:36": "Samsung",
	"18:67:B0": "Samsung",
	"18:E2:C2": "Samsung",
	"1C:5A:6B": "Samsung",
	"1C:66:AA": "Samsung",
	"1C:AF:05": "Samsung",
	"20:13:E0": "Samsung",
	"20:6E:9C": "Samsung",
	"20:D3:90": "Samsung",
	"24:4B:81": "Samsung",
	"24:92:0F": "Samsung",
	"24:9B:0A": "Samsung",
	"24:C6:96": "Samsung",
	"24:DB:ED": "Samsung",
	"28:39:5E": "Samsung",
	"28:57:BE": "Samsung",
	"28:6E:0E": "Samsung",
	"28:A1:83": "Samsung",
	"28:CC:01": "Samsung",
	"2C:44:01": "Samsung",
	"2C:4D:54": "Samsung",
	"2C:5A:05": "Samsung",
	"30:96:FB": "Samsung",
	"30:CB:36": "Samsung",
	"30:D6:C9": "Samsung",
	"34:14:5F": "Samsung",
	"34:AA:8B": "Samsung",
	"34:C3:AC": "Samsung",
	"38:01:95": "Samsung",
	"38:16:D1": "Samsung",
	"38:EC:E4": "Samsung",
	"3C:5A:37": "Samsung",
	"3C:62:00": "Samsung",
	"3C:77:E6": "Samsung",
	"3C:99:F7": "Samsung",
	"40:0E:85": "Samsung",
	// Huawei
	"00:0F:E2": "Huawei",
	"00:18:82": "Huawei",
	"00:1A:1E": "Huawei",
	"00:1C:06": "Huawei",
	"00:1E:10": "Huawei",
	"00:1F:9A": "Huawei",
	"00:22:A1": "Huawei",
	"00:25:68": "Huawei",
	"00:25:9E": "Huawei",
	"00:26:82": "Huawei",
	"00:32:10": "Huawei",
	"00:32:11": "Huawei",
	"00:34:00": "Huawei",
	"00:34:FE": "Huawei",
	"00:3D:E1": "Huawei",
	"00:46:4B": "Huawei",
	"00:4E:FC": "Huawei",
	"00:56:A8": "Huawei",
	"00:66:4B": "Huawei",
	"00:6B:8E": "Huawei",
	"00:88:65": "Huawei",
	"00:90:9E": "Huawei",
	"00:9A:CD": "Huawei",
	"00:E0:20": "Huawei",
	"00:E0:FC": "Huawei",
	"04:02:1F": "Huawei",
	"04:18:D6": "Huawei",
	"04:27:58": "Huawei",
	"04:33:89": "Huawei",
	"04:40:A9": "Huawei",
	"04:61:95": "Huawei",
	"04:79:70": "Huawei",
	"04:82:15": "Huawei",
	"04:9F:CA": "Huawei",
	"04:C3:E6": "Huawei",
	"04:D1:3D": "Huawei",
	"04:F0:21": "Huawei",
	"08:08:11": "Huawei",
	"08:19:A6": "Huawei",
	"08:33:02": "Huawei",
	"08:63:61": "Huawei",
	"08:72:9C": "Huawei",
	"08:7A:4C": "Huawei",
	"08:81:F4": "Huawei",
	"08:8F:2A": "Huawei",
	"08:98:2C": "Huawei",
	"08:9B:4B": "Huawei",
	"08:A1:2B": "Huawei",
	"08:E8:4F": "Huawei",
	"0C:37:DC": "Huawei",
	"0C:96:BF": "Huawei",
	"0C:9D:92": "Huawei",
	"0C:D6:BD": "Huawei",
	"0C:F5:13": "Huawei",
	"10:1B:54": "Huawei",
	"10:44:5A": "Huawei",
	"10:51:07": "Huawei",
	"10:93:97": "Huawei",
	"10:9F:A9": "Huawei",
	"10:A2:4E": "Huawei",
	"10:C1:72": "Huawei",
	"10:F9:6F": "Huawei",
	"14:30:04": "Huawei",
	"14:5F:94": "Huawei",
	"14:7D:DA": "Huawei",
	"14:89:FD": "Huawei",
	"14:B1:C8": "Huawei",
	"14:CF:8D": "Huawei",
	"14:D6:4D": "Huawei",
	"18:0F:76": "Huawei",
	"18:2A:7B": "Huawei",
	"18:3C:B7": "Huawei",
	"18:45:16": "Huawei",
	"18:5A:58": "Huawei",
	"18:7C:0B": "Huawei",
	"18:DE:D7": "Huawei",
	"1C:1D:86": "Huawei",
	"1C:58:C3": "Huawei",
	"1C:8F:D3": "Huawei",
	"1C:99:4C": "Huawei",
	"1C:AD:74": "Huawei",
	"1C:B7:2C": "Huawei",
	"1C:F4:CA": "Huawei",
	"20:08:89": "Huawei",
	"20:0B:C7": "Huawei",
	"20:2B:C1": "Huawei",
	"20:32:6C": "Huawei",
	"20:3D:B2": "Huawei",
	"20:4C:03": "Huawei",
	"20:64:32": "Huawei",
	"20:87:56": "Huawei",
	"20:A6:80": "Huawei",
	"20:D5:BF": "Huawei",
	"24:1F:A0": "Huawei",
	"24:4C:07": "Huawei",
	"24:4C:E3": "Huawei",
	"24:69:A5": "Huawei",
	"24:79:2B": "Huawei",
	"24:7F:20": "Huawei",
	"24:9F:89": "Huawei",
	"24:DB:AC": "Huawei",
	"28:04:76": "Huawei",
	"28:31:16": "Huawei",
	"28:3C:E4": "Huawei",
	"28:5F:DB": "Huawei",
	"28:6E:D4": "Huawei",
	"28:A2:BD": "Huawei",
	"28:BD:89": "Huawei",
	"28:F3:66": "Huawei",
	"28:FF:3E": "Huawei",
	"2C:1A:01": "Huawei",
	"2C:55:D3": "Huawei",
	"2C:9D:1E": "Huawei",
	"2C:AB:00": "Huawei",
	"2C:D0:FA": "Huawei",
	"2C:F0:5D": "Huawei",
	"30:09:F9": "Huawei",
	"30:45:96": "Huawei",
	"30:74:96": "Huawei",
	"30:87:30": "Huawei",
	"30:A2:C2": "Huawei",
	"30:F3:1D": "Huawei",
	"34:00:A3": "Huawei",
	"34:6A:C2": "Huawei",
	"34:6B:D3": "Huawei",
	"34:A5:9C": "Huawei",
	"38:0F:4A": "Huawei",
	"38:F2:3E": "Huawei",
	"3C:47:5A": "Huawei",
	"3C:4D:BE": "Huawei",
	"3C:FA:43": "Huawei",
	"40:4D:8E": "Huawei",
	// Xiaomi
	"00:9E:C8": "Xiaomi",
	"14:F6:5A": "Xiaomi",
	"18:59:36": "Xiaomi",
	"20:82:C0": "Xiaomi",
	"28:E3:1F": "Xiaomi",
	"34:CE:00": "Xiaomi",
	"3C:BD:3E": "Xiaomi",
	"50:EC:50": "Xiaomi",
	"64:69:4E": "Xiaomi",
	"64:B4:73": "Xiaomi",
	"64:CC:2E": "Xiaomi",
	"74:23:44": "Xiaomi",
	"78:02:B1": "Xiaomi",
	"7C:49:EB": "Xiaomi",
	"88:28:B3": "Xiaomi",
	"8C:53:C3": "Xiaomi",
	"90:32:4B": "Xiaomi",
	"98:0C:A5": "Xiaomi",
	"9C:99:A0": "Xiaomi",
	"A4:7B:9C": "Xiaomi",
	"AC:F7:F3": "Xiaomi",
	"B0:E2:35": "Xiaomi",
	"C4:6A:B7": "Xiaomi",
	"CC:B5:11": "Xiaomi",
	"D4:97:0B": "Xiaomi",
	"E4:FA:C4": "Xiaomi",
	"F0:B4:29": "Xiaomi",
	"F4:F5:DB": "Xiaomi",
	"F8:A4:5F": "Xiaomi",
	"FC:64:BA": "Xiaomi",
	// TP-Link
	"00:14:78": "TP-Link",
	"00:1D:0F": "TP-Link",
	"00:25:86": "TP-Link",
	"00:27:19": "TP-Link",
	"00:36:76": "TP-Link",
	"00:5F:67": "TP-Link",
	"00:A0:57": "TP-Link",
	"00:AD:63": "TP-Link",
	"00:C0:CA": "TP-Link",
	"00:E0:4C": "TP-Link",
	"04:C0:6F": "TP-Link",
	"08:10:77": "TP-Link",
	"0C:4C:39": "TP-Link",
	"10:27:BE": "TP-Link",
	"14:CC:20": "TP-Link",
	"18:D6:C7": "TP-Link",
	"1C:3B:F3": "TP-Link",
	"20:DC:E6": "TP-Link",
	"24:69:68": "TP-Link",
	"28:2C:B2": "TP-Link",
	"2C:08:3B": "TP-Link",
	"2C:30:33": "TP-Link",
	"30:B5:C2": "TP-Link",
	"34:E8:94": "TP-Link",
	"38:EA:A7": "TP-Link",
	"3C:84:6A": "TP-Link",
	"40:16:9F": "TP-Link",
	"40:ED:00": "TP-Link",
	"44:6E:20": "TP-Link",
	"48:21:6B": "TP-Link",
	"4C:11:BF": "TP-Link",
	"50:C7:BF": "TP-Link",
	"54:C8:0F": "TP-Link",
	"58:D9:D5": "TP-Link",
	"5C:63:BF": "TP-Link",
	"60:83:73": "TP-Link",
	"60:A4:B7": "TP-Link",
	"64:66:B3": "TP-Link",
	"64:70:02": "TP-Link",
	"68:FF:7B": "TP-Link",
	"6C:5A:B0": "TP-Link",
	"70:4F:57": "TP-Link",
	"70:8A:09": "TP-Link",
	"74:DA:88": "TP-Link",
	"78:44:76": "TP-Link",
	"78:C3:E9": "TP-Link",
	"7C:8B:CA": "TP-Link",
	"80:69:33": "TP-Link",
	"84:16:F9": "TP-Link",
	"84:D8:1B": "TP-Link",
	"88:25:93": "TP-Link",
	"8C:59:C3": "TP-Link",
	"90:9A:4A": "TP-Link",
	"90:F6:52": "TP-Link",
	"94:D9:B3": "TP-Link",
	"98:DA:C4": "TP-Link",
	"9C:A6:15": "TP-Link",
	"A0:63:91": "TP-Link",
	"A4:2B:B0": "TP-Link",
	"AC:15:A2": "TP-Link",
	"AC:84:C6": "TP-Link",
	"B0:95:75": "TP-Link",
	"B0:BE:76": "TP-Link",
	"B4:B0:24": "TP-Link",
	"BC:46:99": "TP-Link",
	"C0:4A:00": "TP-Link",
	"C0:61:18": "TP-Link",
	"C0:6C:F1": "TP-Link",
	"C0:C9:E3": "TP-Link",
	"C4:6E:1F": "TP-Link",
	"C4:A8:1D": "TP-Link",
	"CC:08:FB": "TP-Link",
	"CC:32:E5": "TP-Link",
	"CC:81:DA": "TP-Link",
	"D0:37:45": "TP-Link",
	"D4:6D:6D": "TP-Link",
	"D4:92:34": "TP-Link",
	"D8:0D:17": "TP-Link",
	"D8:5D:4C": "TP-Link",
	"DC:15:C8": "TP-Link",
	"E0:05:C5": "TP-Link",
	"E4:0E:EE": "TP-Link",
	"E8:DE:27": "TP-Link",
	"EC:08:6B": "TP-Link",
	"EC:17:2F": "TP-Link",
	"F0:F3:35": "TP-Link",
	"F4:83:CD": "TP-Link",
	"F4:EC:38": "TP-Link",
	"F4:F2:6D": "TP-Link",
	"F8:5C:4D": "TP-Link",
	"FC:7C:02": "TP-Link",
	// Netgear
	"00:00:5B": "Netgear",
	"00:09:5B": "Netgear",
	"00:0C:41": "Netgear",
	"00:0F:B5": "Netgear",
	"00:14:6C": "Netgear",
	"00:18:4D": "Netgear",
	"00:1B:2F": "Netgear",
	"00:1E:2A": "Netgear",
	"00:1F:33": "Netgear",
	"00:22:3F": "Netgear",
	"00:24:B2": "Netgear",
	"00:8E:F2": "Netgear",
	"04:A1:51": "Netgear",
	"08:02:8E": "Netgear",
	"08:36:C9": "Netgear",
	"08:BD:43": "Netgear",
	"0C:54:A5": "Netgear",
	"10:0D:7F": "Netgear",
	"10:DA:43": "Netgear",
	"14:59:C0": "Netgear",
	"20:4E:7F": "Netgear",
	"20:E5:2A": "Netgear",
	"28:80:88": "Netgear",
	"2C:B0:5D": "Netgear",
	"30:46:9A": "Netgear",
	"34:98:B5": "Netgear",
	"38:94:ED": "Netgear",
	"40:5B:DF": "Netgear",
	"44:A5:6E": "Netgear",
	"44:D9:E7": "Netgear",
	"4C:60:DE": "Netgear",
	"50:6A:03": "Netgear",
	"50:E5:49": "Netgear",
	"54:B5:6C": "Netgear",
	"6C:B0:CE": "Netgear",
	"74:44:01": "Netgear",
	"74:59:09": "Netgear",
	"74:DA:38": "Netgear",
	"80:37:73": "Netgear",
	"84:1B:5E": "Netgear",
	"8C:3B:AD": "Netgear",
	"94:18:82": "Netgear",
	"9C:D3:6D": "Netgear",
	"A0:21:B7": "Netgear",
	"A4:11:62": "Netgear",
	"A4:2B:8C": "Netgear",
	"A4:5D:A1": "Netgear",
	"AC:D9:96": "Netgear",
	"B0:39:56": "Netgear",
	"B0:7F:B9": "Netgear",
	"B0:B9:8A": "Netgear",
	"B4:A5:EF": "Netgear",
	"B8:3B:CC": "Netgear",
	"B8:5E:7B": "Netgear",
	"BC:A5:11": "Netgear",
	"C0:3F:0E": "Netgear",
	"C0:FF:D4": "Netgear",
	"C4:04:15": "Netgear",
	"C4:3D:C7": "Netgear",
	"C4:E9:84": "Netgear",
	"CC:40:D0": "Netgear",
	"D4:6B:A6": "Netgear",
	"D4:9C:28": "Netgear",
	"D8:6C:63": "Netgear",
	"DC:EF:CA": "Netgear",
	"E0:46:9A": "Netgear",
	"E0:91:F5": "Netgear",
	"E4:3C:D4": "Netgear",
	"E4:6F:13": "Netgear",
	"E8:FC:AF": "Netgear",
	"E8:FD:F8": "Netgear",
	"EC:AD:B8": "Netgear",
	"F0:3E:90": "Netgear",
	"F0:4F:7C": "Netgear",
	"F0:A7:31": "Netgear",
	"F4:06:8F": "Netgear",
	"F8:38:80": "Netgear",
	"FC:E5:57": "Netgear",
	// ASUS
	"00:0C:6E": "ASUS",
	"00:0E:A6": "ASUS",
	"00:12:17": "ASUS",
	"00:13:D4": "ASUS",
	"00:15:F2": "ASUS",
	"00:17:31": "ASUS",
	"00:18:F3": "ASUS",
	"00:19:D2": "ASUS",
	"00:1A:92": "ASUS",
	"00:1B:FC": "ASUS",
	"00:1C:C0": "ASUS",
	"00:1D:60": "ASUS",
	"00:1E:8C": "ASUS",
	"00:1F:C6": "ASUS",
	"00:22:15": "ASUS",
	"00:23:54": "ASUS",
	"00:24:8C": "ASUS",
	"00:25:22": "ASUS",
	"00:26:18": "ASUS",
	"04:42:1A": "ASUS",
	"04:D4:C4": "ASUS",
	"08:60:6E": "ASUS",
	"08:BF:B8": "ASUS",
	"10:7B:44": "ASUS",
	"10:7C:61": "ASUS",
	"10:BF:48": "ASUS",
	"14:DA:E9": "ASUS",
	"14:DD:A9": "ASUS",
	"18:31:BF": "ASUS",
	"1C:87:79": "ASUS",
	"20:CF:30": "ASUS",
	"24:4B:FE": "ASUS",
	"28:24:FF": "ASUS",
	"2C:56:DC": "ASUS",
	"30:5A:3A": "ASUS",
	"34:97:F6": "ASUS",
	"38:2C:4A": "ASUS",
	"38:D5:47": "ASUS",
	"3C:54:47": "ASUS",
	"40:16:7E": "ASUS",
	"40:B0:76": "ASUS",
	"48:F8:B3": "ASUS",
	"50:46:5D": "ASUS",
	"54:04:A6": "ASUS",
	"54:A0:50": "ASUS",
	"60:45:CB": "ASUS",
	"60:A4:4C": "ASUS",
	"60:CF:84": "ASUS",
	"6C:AD:EF": "ASUS",
	"70:4D:7B": "ASUS",
	"70:8B:CD": "ASUS",
	"74:D0:2B": "ASUS",
	"78:24:AF": "ASUS",
	"78:E3:B5": "ASUS",
	"7C:10:C9": "ASUS",
	"80:1F:02": "ASUS",
	"80:3F:5D": "ASUS",
	"88:D7:F6": "ASUS",
	"8C:89:A5": "ASUS",
	"90:E6:BA": "ASUS",
	"94:DB:D4": "ASUS",
	"9C:5C:8E": "ASUS",
	"A0:36:BC": "ASUS",
	"A4:62:DF": "ASUS",
	"AC:22:0B": "ASUS",
	"AC:9E:17": "ASUS",
	"B0:6E:BF": "ASUS",
	"BC:EE:7B": "ASUS",
	"C0:A0:BB": "ASUS",
	"C0:C1:C0": "ASUS",
	"C8:60:00": "ASUS",
	"CC:28:AA": "ASUS",
	"CC:46:D6": "ASUS",
	"D0:17:C2": "ASUS",
	"D4:5D:64": "ASUS",
	"D4:5D:DF": "ASUS",
	"D8:50:E6": "ASUS",
	"D8:CE:3A": "ASUS",
	"DC:0E:A1": "ASUS",
	"E0:3F:49": "ASUS",
	"E0:CB:4E": "ASUS",
	"E4:F8:9C": "ASUS",
	"E4:4E:84": "ASUS",
	"EC:88:8F": "ASUS",
	"F0:79:59": "ASUS",
	"F4:6D:04": "ASUS",
	"F4:8E:38": "ASUS",
	"F8:32:E4": "ASUS",
	"FC:34:97": "ASUS",
	// Intel
	"00:02:B3": "Intel",
	"00:03:47": "Intel",
	"00:04:23": "Intel",
	"00:0E:0C": "Intel",
	"00:0E:FC": "Intel",
	"00:12:F0": "Intel",
	"00:13:02": "Intel",
	"00:13:20": "Intel",
	"00:13:E8": "Intel",
	"00:15:00": "Intel",
	"00:15:17": "Intel",
	"00:16:6F": "Intel",
	"00:16:76": "Intel",
	"00:16:EA": "Intel",
	"00:17:7F": "Intel",
	"00:18:DE": "Intel",
	"00:19:D1": "Intel",
	"00:1B:21": "Intel",
	"00:1C:BF": "Intel",
	"00:1D:E0": "Intel",
	"00:1E:64": "Intel",
	"00:1E:67": "Intel",
	"00:1E:E5": "Intel",
	"00:1F:3B": "Intel",
	"00:1F:3C": "Intel",
	"00:21:5C": "Intel",
	"00:21:5D": "Intel",
	"00:21:6A": "Intel",
	"00:21:6B": "Intel",
	"00:22:FA": "Intel",
	"00:22:FB": "Intel",
	"00:24:D6": "Intel",
	"00:24:D7": "Intel",
	// Microsoft
	"00:03:FF": "Microsoft",
	"00:0D:3A": "Microsoft",
	"00:12:5A": "Microsoft",
	"00:13:E2": "Microsoft",
	"00:15:5D": "Microsoft",
	"00:17:FA": "Microsoft",
	"00:1C:C5": "Microsoft",
	"00:1D:D8": "Microsoft",
	"00:22:48": "Microsoft",
	"00:24:36": "Microsoft",
	"00:25:AE": "Microsoft",
	"00:50:F2": "Microsoft",
	"00:80:5F": "Microsoft",
	"00:90:4C": "Microsoft",
	"00:A0:98": "Microsoft",
	"00:D0:59": "Microsoft",
	// Google
	"00:1A:11": "Google",
	"08:9E:08": "Google",
	"18:D6:6F": "Google",
	"28:BC:18": "Google",
	"2C:54:CF": "Google",
	"30:FD:38": "Google",
	"3C:5A:B4": "Google",
	"3C:8D:20": "Google",
	"54:60:09": "Google",
	"54:99:63": "Google",
	"58:CB:52": "Google",
	"5C:E8:83": "Google",
	"64:9E:F3": "Google",
	"74:AC:5F": "Google",
	"74:E5:43": "Google",
	"78:31:C1": "Google",
	"7C:61:93": "Google",
	"7C:9A:54": "Google",
	"84:8E:0C": "Google",
	"88:73:98": "Google",
	"90:18:AE": "Google",
	"94:EB:2C": "Google",
	"9C:8E:99": "Google",
	"A4:77:33": "Google",
	"A4:DA:22": "Google",
	"A4:E4:B8": "Google",
	"AC:5F:3E": "Google",
	"B4:E6:2D": "Google",
	"BC:6E:E2": "Google",
	"C0:97:27": "Google",
	"C4:9D:ED": "Google",
	"CC:B0:DA": "Google",
	"D8:EB:46": "Google",
	"DC:B5:4F": "Google",
	"E0:CB:1D": "Google",
	"E4:58:E7": "Google",
	"E4:F0:42": "Google",
	"F4:F5:E8": "Google",
	"F8:8F:07": "Google",
	"FC:F1:52": "Google",
	// Amazon
	"00:0B:81": "Amazon",
	"00:0F:4B": "Amazon",
	"00:11:43": "Amazon",
	"00:14:51": "Amazon",
	"00:15:C1": "Amazon",
	"00:17:41": "Amazon",
	"00:19:E0": "Amazon",
	"00:1B:9E": "Amazon",
	"00:1D:92": "Amazon",
	"00:1E:C2": "Amazon",
	"00:20:F4": "Amazon",
	"00:22:58": "Amazon",
	"00:BB:3A": "Amazon",
	"00:F0:2C": "Amazon",
	"02:5C:B1": "Amazon",
	"02:63:94": "Amazon",
	"02:6F:F5": "Amazon",
	"02:78:18": "Amazon",
	"02:8F:CE": "Amazon",
	"02:BD:C9": "Amazon",
	"02:F0:EA": "Amazon",
	"02:F1:6C": "Amazon",
	"02:F8:9B": "Amazon",
	"0A:0E:2F": "Amazon",
	"0A:A7:BE": "Amazon",
	"0A:B0:56": "Amazon",
	"0A:CE:91": "Amazon",
	"0A:F6:FD": "Amazon",
	"12:5F:15": "Amazon",
	"12:A5:7C": "Amazon",
	"12:AF:4F": "Amazon",
	"12:B0:AD": "Amazon",
	"12:B7:F2": "Amazon",
	"12:F2:C5": "Amazon",
	"1A:34:5B": "Amazon",
	"1A:A7:BE": "Amazon",
	"1A:B0:56": "Amazon",
	"22:34:5B": "Amazon",
	"2A:34:5B": "Amazon",
	"2A:A7:BE": "Amazon",
	"2A:B0:56": "Amazon",
	"32:34:5B": "Amazon",
	"3A:34:5B": "Amazon",
	"3A:A7:BE": "Amazon",
	"3A:B0:56": "Amazon",
	"42:34:5B": "Amazon",
	"4A:34:5B": "Amazon",
	"4A:A7:BE": "Amazon",
	"4A:B0:56": "Amazon",
	"52:34:5B": "Amazon",
	"5A:34:5B": "Amazon",
	"5A:A7:BE": "Amazon",
	"5A:B0:56": "Amazon",
	"62:34:5B": "Amazon",
	"6A:34:5B": "Amazon",
	"6A:A7:BE": "Amazon",
	"6A:B0:56": "Amazon",
	"72:34:5B": "Amazon",
	"7A:34:5B": "Amazon",
	"7A:A7:BE": "Amazon",
	"7A:B0:56": "Amazon",
	"82:34:5B": "Amazon",
	"8A:34:5B": "Amazon",
	"8A:A7:BE": "Amazon",
	"8A:B0:56": "Amazon",
	"92:34:5B": "Amazon",
	"9A:34:5B": "Amazon",
	"9A:A7:BE": "Amazon",
	"9A:B0:56": "Amazon",
	"A2:34:5B": "Amazon",
	"A4:34:F1": "Amazon",
	"AA:34:5B": "Amazon",
	"AA:A7:BE": "Amazon",
	"AA:B0:56": "Amazon",
	"B2:34:5B": "Amazon",
	"BA:34:5B": "Amazon",
	"BA:A7:BE": "Amazon",
	"BA:B0:56": "Amazon",
	"C2:34:5B": "Amazon",
	"C6:3F:55": "Amazon",
	"CA:34:5B": "Amazon",
	"CA:A7:BE": "Amazon",
	"CA:B0:56": "Amazon",
	"D2:34:5B": "Amazon",
	"DA:34:5B": "Amazon",
	"DA:A7:BE": "Amazon",
	"DA:B0:56": "Amazon",
	"E2:34:5B": "Amazon",
	"EA:34:5B": "Amazon",
	"EA:A7:BE": "Amazon",
	"EA:B0:56": "Amazon",
	"F2:34:5B": "Amazon",
	"FA:34:5B": "Amazon",
	"FA:A7:BE": "Amazon",
	"FA:B0:56": "Amazon",
}

// deviceTypeMapping maps vendors to device types
var deviceTypeMapping = map[string]string{
	// Phones
	"Apple":   "phone",
	"Samsung": "phone",
	"Huawei":  "phone",
	"Xiaomi":  "phone",
	// Routers/Network equipment
	"Cisco":   "router",
	"Juniper": "router",
	"TP-Link": "router",
	"Netgear": "router",
	"ASUS":    "router",
	// PCs/Laptops
	"Dell":      "pc",
	"HP":        "pc",
	"Lenovo":    "pc",
	"Intel":     "pc",
	"Microsoft": "pc",
	// IoT/Embedded
	"Raspberry Pi": "iot",
	// Virtual machines
	"VMware":     "server",
	"VirtualBox": "server",
	// Cloud
	"Amazon": "server",
	"Google": "server",
}

// NewMACTracker creates a new MAC tracker with built-in OUI database
func NewMACTracker() *MACTracker {
	return &MACTracker{
		devices: make(map[string]*Device),
		ouiDB:   builtinOUI,
	}
}

// NewMACTrackerWithDB creates a new MAC tracker with database persistence
func NewMACTrackerWithDB(db *sql.DB) *MACTracker {
	t := &MACTracker{
		devices: make(map[string]*Device),
		ouiDB:   builtinOUI,
		db:      db,
	}
	// Load existing devices from database
	t.loadFromDB()
	return t
}

// loadFromDB loads devices from database
func (t *MACTracker) loadFromDB() {
	if t.db == nil {
		return
	}

	rows, err := t.db.Query(`SELECT mac, name, vendor, ips, first_seen, last_seen, bytes_sent, bytes_recv, flow_count, device_type FROM devices`)
	if err != nil {
		log.Printf("Failed to load devices from DB: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var d Device
		var ipsJSON string
		var firstSeen, lastSeen sql.NullString

		if err := rows.Scan(&d.MAC, &d.Name, &d.Vendor, &ipsJSON, &firstSeen, &lastSeen, &d.BytesSent, &d.BytesRecv, &d.FlowCount, &d.DeviceType); err != nil {
			log.Printf("Failed to scan device row: %v", err)
			continue
		}

		if ipsJSON != "" {
			json.Unmarshal([]byte(ipsJSON), &d.IPs)
		}
		if firstSeen.Valid {
			if t, err := time.Parse("2006-01-02 15:04:05", firstSeen.String); err == nil {
				d.FirstSeen = t
			}
		}
		if lastSeen.Valid {
			if t, err := time.Parse("2006-01-02 15:04:05", lastSeen.String); err == nil {
				d.LastSeen = t
			}
		}

		d.IsOnline = time.Since(d.LastSeen) < 5*time.Minute
		t.devices[d.MAC] = &d
	}
	log.Printf("Loaded %d devices from database", len(t.devices))
}

// saveDevice saves a device to database
func (t *MACTracker) saveDevice(d *Device) error {
	if t.db == nil {
		return nil
	}

	ipsJSON, _ := json.Marshal(d.IPs)

	_, err := t.db.Exec(`INSERT OR REPLACE INTO devices (mac, name, vendor, ips, first_seen, last_seen, bytes_sent, bytes_recv, flow_count, device_type)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.MAC, d.Name, d.Vendor, string(ipsJSON), d.FirstSeen.Format("2006-01-02 15:04:05"),
		d.LastSeen.Format("2006-01-02 15:04:05"), d.BytesSent, d.BytesRecv, d.FlowCount, d.DeviceType)

	return err
}

// formatMAC formats a MAC address to standard AA:BB:CC:DD:EE:FF format
func formatMAC(mac net.HardwareAddr) string {
	return strings.ToUpper(mac.String())
}

// getOUI extracts OUI (first 3 bytes) from MAC address
func getOUI(mac net.HardwareAddr) string {
	if len(mac) < 3 {
		return ""
	}
	return formatMAC(mac[:3])
}

// lookupVendor looks up vendor by MAC address
func (t *MACTracker) lookupVendor(mac net.HardwareAddr) string {
	oui := getOUI(mac)
	if vendor, ok := t.ouiDB[oui]; ok {
		return vendor
	}
	return ""
}

// inferDeviceType infers device type based on vendor
func (t *MACTracker) inferDeviceType(vendor string, bytesSent, bytesRecv uint64) string {
	// Check vendor mapping first
	if deviceType, ok := deviceTypeMapping[vendor]; ok {
		return deviceType
	}

	// Infer from traffic characteristics
	totalBytes := bytesSent + bytesRecv
	if totalBytes > 1024*1024*1024 { // > 1GB
		return "server"
	}

	return "unknown"
}

// ProcessPacket processes a packet and updates device records
func (t *MACTracker) ProcessPacket(srcMAC, dstMAC net.HardwareAddr, srcIP, dstIP string, bytes uint64) {
	now := time.Now()

	// Process source device
	if srcMAC != nil {
		t.updateDevice(srcMAC, srcIP, bytes, 0, now)
	}

	// Process destination device
	if dstMAC != nil {
		t.updateDevice(dstMAC, dstIP, 0, bytes, now)
	}
}

// updateDevice updates or creates a device record
func (t *MACTracker) updateDevice(mac net.HardwareAddr, ip string, bytesSent, bytesRecv uint64, now time.Time) {
	if mac == nil {
		return
	}

	macStr := formatMAC(mac)

	t.mu.Lock()
	defer t.mu.Unlock()

	device, exists := t.devices[macStr]
	if !exists {
		// Create new device
		vendor := t.lookupVendor(mac)
		ips := []string{}
		if ip != "" {
			ips = []string{ip}
		}
		device = &Device{
			MAC:       macStr,
			IPs:       ips,
			Vendor:    vendor,
			FirstSeen: now,
			IsOnline:  true,
		}
		t.devices[macStr] = device
	} else {
		// Add IP to list if not already present
		if ip != "" {
			ipExists := false
			for _, existingIP := range device.IPs {
				if existingIP == ip {
					ipExists = true
					break
				}
			}
			if !ipExists {
				device.IPs = append(device.IPs, ip)
			}
		}
	}

	device.LastSeen = now
	device.IsOnline = true
	device.BytesSent += bytesSent
	device.BytesRecv += bytesRecv

	// Infer device type if not already set or was unknown
	if device.DeviceType == "" || device.DeviceType == "unknown" {
		device.DeviceType = t.inferDeviceType(device.Vendor, device.BytesSent, device.BytesRecv)
	}

	// Save to database
	if err := t.saveDevice(device); err != nil {
		log.Printf("Failed to save device %s: %v", macStr, err)
	}
}

// GetDevices returns all devices
func (t *MACTracker) GetDevices() []Device {
	t.mu.RLock()
	defer t.mu.RUnlock()

	devices := make([]Device, 0, len(t.devices))
	for _, d := range t.devices {
		devices = append(devices, *d)
	}
	return devices
}

// GetDevicesSorted returns devices sorted by specified field
func (t *MACTracker) GetDevicesSorted(sortBy string, limit int) []Device {
	t.mu.RLock()
	defer t.mu.RUnlock()

	devices := make([]Device, 0, len(t.devices))
	for _, d := range t.devices {
		devices = append(devices, *d)
	}

	// Sort devices
	switch sortBy {
	case "traffic":
		for i := 0; i < len(devices); i++ {
			for j := i + 1; j < len(devices); j++ {
				if devices[i].BytesSent+devices[i].BytesRecv < devices[j].BytesSent+devices[j].BytesRecv {
					devices[i], devices[j] = devices[j], devices[i]
				}
			}
		}
	case "last_seen":
		for i := 0; i < len(devices); i++ {
			for j := i + 1; j < len(devices); j++ {
				if devices[i].LastSeen.Before(devices[j].LastSeen) {
					devices[i], devices[j] = devices[j], devices[i]
				}
			}
		}
	case "flow_count":
		for i := 0; i < len(devices); i++ {
			for j := i + 1; j < len(devices); j++ {
				if devices[i].FlowCount < devices[j].FlowCount {
					devices[i], devices[j] = devices[j], devices[i]
				}
			}
		}
	default: // MAC address
		for i := 0; i < len(devices); i++ {
			for j := i + 1; j < len(devices); j++ {
				if devices[i].MAC > devices[j].MAC {
					devices[i], devices[j] = devices[j], devices[i]
				}
			}
		}
	}

	if limit > 0 && len(devices) > limit {
		devices = devices[:limit]
	}
	return devices
}

// GetDevice returns a device by MAC address
func (t *MACTracker) GetDevice(mac string) *Device {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// Normalize MAC address
	mac = strings.ToUpper(mac)

	if device, ok := t.devices[mac]; ok {
		// Return a copy
		d := *device
		return &d
	}
	return nil
}

// UpdateFromFlow updates device info from flow data
// This is called when a flow is processed to track MAC->IP mappings
func (t *MACTracker) UpdateFromFlow(srcMAC, dstMAC net.HardwareAddr, srcIP, dstIP string, bytesSent, bytesRecv uint64) {
	now := time.Now()
	if srcMAC != nil {
		t.updateDevice(srcMAC, srcIP, bytesSent, 0, now)
	}
	if dstMAC != nil {
		t.updateDevice(dstMAC, dstIP, 0, bytesRecv, now)
	}
}

// IncrementFlowCount increments the flow count for a device
func (t *MACTracker) IncrementFlowCount(mac string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	mac = strings.ToUpper(mac)
	if device, ok := t.devices[mac]; ok {
		device.FlowCount++
		t.saveDevice(device)
	}
}

// SearchDevices searches devices by MAC, IP, vendor or name
func (t *MACTracker) SearchDevices(query string) []Device {
	t.mu.RLock()
	defer t.mu.RUnlock()

	query = strings.ToLower(query)
	var results []Device

	for _, d := range t.devices {
		// Search in MAC
		if strings.Contains(strings.ToLower(d.MAC), query) {
			results = append(results, *d)
			continue
		}
		// Search in IPs
		for _, ip := range d.IPs {
			if strings.Contains(strings.ToLower(ip), query) {
				results = append(results, *d)
				break
			}
		}
		// Search in vendor
		if strings.Contains(strings.ToLower(d.Vendor), query) {
			results = append(results, *d)
			continue
		}
		// Search in name
		if strings.Contains(strings.ToLower(d.Name), query) {
			results = append(results, *d)
		}
	}
	return results
}

// GetDeviceByIP returns a device by IP address
func (t *MACTracker) GetDeviceByIP(ip string) *Device {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for _, d := range t.devices {
		for _, deviceIP := range d.IPs {
			if deviceIP == ip {
				dCopy := *d
				return &dCopy
			}
		}
	}
	return nil
}

// CheckOffline marks devices as offline if they haven't been seen for more than 5 minutes
func (t *MACTracker) CheckOffline() {
	t.mu.Lock()
	defer t.mu.Unlock()

	threshold := time.Now().Add(-5 * time.Minute)
	for _, d := range t.devices {
		if d.IsOnline && d.LastSeen.Before(threshold) {
			d.IsOnline = false
		}
	}
}

// GetDeviceStats returns device statistics
func (t *MACTracker) GetDeviceStats() map[string]interface{} {
	t.mu.RLock()
	defer t.mu.RUnlock()

	onlineCount := 0
	offlineCount := 0
	vendorDist := make(map[string]int)
	typeDist := make(map[string]int)

	for _, d := range t.devices {
		if d.IsOnline {
			onlineCount++
		} else {
			offlineCount++
		}

		vendor := d.Vendor
		if vendor == "" {
			vendor = "Unknown"
		}
		vendorDist[vendor]++

		deviceType := d.DeviceType
		if deviceType == "" {
			deviceType = "unknown"
		}
		typeDist[deviceType]++
	}

	return map[string]interface{}{
		"online_count":  onlineCount,
		"offline_count": offlineCount,
		"total_count":   len(t.devices),
		"vendor_dist":   vendorDist,
		"type_dist":     typeDist,
	}
}

// UpdateDeviceOS updates the OS information for a device
func (t *MACTracker) UpdateDeviceOS(mac string, os string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	mac = strings.ToUpper(mac)
	if device, ok := t.devices[mac]; ok {
		device.OS = os
	}
}

// UpdateDeviceHostname updates the hostname for a device
func (t *MACTracker) UpdateDeviceHostname(mac string, hostname string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	mac = strings.ToUpper(mac)
	if device, ok := t.devices[mac]; ok {
		device.Hostname = hostname
	}
}
