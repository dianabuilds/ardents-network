package transport

import (
	"fmt"
	"net"
	"strings"

	gethdns "github.com/ethereum/go-ethereum/p2p/dnsdisc"
)

const maxDNSDiscoveryURLs = 4

func validateDiscoveryConfig(cfg Config) error {
	urls := cfg.DNSDiscoveryURLs
	if len(urls) > maxDNSDiscoveryURLs {
		return fmt.Errorf("DNS discovery accepts at most %d signed ENR trees", maxDNSDiscoveryURLs)
	}
	seen := make(map[string]struct{}, len(urls))
	for _, rawURL := range urls {
		url := strings.TrimSpace(rawURL)
		if _, _, err := gethdns.ParseURL(url); err != nil {
			return fmt.Errorf("DNS discovery URL must be a signed enrtree URL")
		}
		if _, exists := seen[url]; exists {
			return fmt.Errorf("DNS discovery URLs must be unique")
		}
		seen[url] = struct{}{}
	}
	nameserver := strings.TrimSpace(cfg.DNSDiscoveryNameServer)
	if nameserver != "" && len(urls) == 0 {
		return fmt.Errorf("DNS discovery nameserver requires at least one signed ENR tree")
	}
	if nameserver != "" && net.ParseIP(nameserver) == nil {
		return fmt.Errorf("DNS discovery nameserver must be an IP address without a port")
	}
	return nil
}
