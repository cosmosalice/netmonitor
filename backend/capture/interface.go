package capture

import (
	"fmt"
	"net"

	"github.com/google/gopacket/pcap"
)

// NetworkInterface represents a network interface
type NetworkInterface struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IPAddress   string `json:"ip_address"`
	MACAddress  string `json:"mac_address"`
	IsUp        bool   `json:"is_up"`
}

// ListInterfaces returns all available network interfaces
func ListInterfaces() ([]NetworkInterface, error) {
	// Get pcap interfaces
	pcapIfaces, err := pcap.FindAllDevs()
	if err != nil {
		return nil, fmt.Errorf("failed to list pcap interfaces: %w", err)
	}

	// Get system interfaces
	sysIfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to list system interfaces: %w", err)
	}

	// Create map for quick lookup
	sysIfaceMap := make(map[string]net.Interface)
	for _, iface := range sysIfaces {
		sysIfaceMap[iface.Name] = iface
	}

	// Merge information
	var interfaces []NetworkInterface
	for _, pcapIface := range pcapIfaces {
		iface := NetworkInterface{
			Name:        pcapIface.Name,
			Description: pcapIface.Description,
		}

		// Get IP address
		if len(pcapIface.Addresses) > 0 {
			iface.IPAddress = pcapIface.Addresses[0].IP.String()
		}

		// Get MAC address and status from system interface
		if sysIface, ok := sysIfaceMap[pcapIface.Name]; ok {
			iface.MACAddress = sysIface.HardwareAddr.String()
			// net.FlagUp is unreliable on Windows; also treat interfaces with
			// non-link-local IP addresses as active.
			iface.IsUp = sysIface.Flags&net.FlagUp != 0 || len(pcapIface.Addresses) > 0
		} else {
			// No matching system interface found – fall back to address heuristic
			iface.IsUp = len(pcapIface.Addresses) > 0
		}

		interfaces = append(interfaces, iface)
	}

	return interfaces, nil
}
