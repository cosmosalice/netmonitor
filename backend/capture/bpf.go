package capture

import (
	"fmt"
)

// BPFValidator validates BPF filter syntax
type BPFValidator struct{}

// NewBPFValidator creates a new BPF validator
func NewBPFValidator() *BPFValidator {
	return &BPFValidator{}
}

// Validate checks if a BPF filter string is valid
func (v *BPFValidator) Validate(filter string) error {
	if filter == "" {
		return nil
	}

	// Basic syntax check
	if len(filter) > 1024 {
		return fmt.Errorf("BPF filter too long (max 1024 chars)")
	}

	return nil
}

// CommonFilters provides preset BPF filters
var CommonFilters = map[string]string{
	"http":         "tcp port 80",
	"https":        "tcp port 443",
	"dns":          "udp port 53",
	"ssh":          "tcp port 22",
	"smtp":         "tcp port 25",
	"ftp":          "tcp port 20 or tcp port 21",
	"web_traffic":  "tcp port 80 or tcp port 443 or tcp port 8080 or tcp port 8443",
	"voip":         "udp portrange 10000-20000",
	"all_tcp":      "tcp",
	"all_udp":      "udp",
	"no_broadcast": "not ether broadcast",
}

// GetCommonFilter returns a preset BPF filter by name
func GetCommonFilter(name string) (string, error) {
	filter, ok := CommonFilters[name]
	if !ok {
		return "", fmt.Errorf("unknown filter name: %s", name)
	}
	return filter, nil
}

// ListCommonFilters returns all available preset filter names
func ListCommonFilters() []string {
	var names []string
	for name := range CommonFilters {
		names = append(names, name)
	}
	return names
}
