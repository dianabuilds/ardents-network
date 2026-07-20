package process

import (
	"fmt"
	"net"
	"strings"
	"time"

	networkapi "ardents/internal/network/api"
)

func ValidateConfig(cfg Config) error {
	nodeProfile := networkapi.NormalizeNodeProfile(cfg.NodeProfile)
	transportProfile := networkapi.NormalizeProfile(cfg.Transport.Profile)
	if err := networkapi.ValidateNodeProfileTransport(nodeProfile, transportProfile); err != nil {
		return fmt.Errorf("network participation configuration: %w", err)
	}
	if nodeProfile == networkapi.NodeProfileLocalDevelopment && !isLoopbackBind(networkapi.ResolveBindAddress(cfg.Transport.BindAddress)) {
		return fmt.Errorf("network participation configuration: node profile %q requires a loopback bind address", nodeProfile)
	}
	if nodeProfile == networkapi.NodeProfileLocalDevelopment && len(cfg.Transport.DNSDiscoveryURLs) > 0 {
		return fmt.Errorf("network participation configuration: node profile %q does not allow network DNS discovery", nodeProfile)
	}
	mode, err := validateParticipationReachability(cfg, nodeProfile)
	if err != nil {
		return err
	}
	transportConfig := networkapi.Config{
		NodeProfile:            nodeProfile,
		Profile:                cfg.Transport.Profile,
		WSSPort:                cfg.Transport.WSSPort,
		WSSCertPath:            cfg.Transport.WSSCertPath,
		WSSKeyPath:             cfg.Transport.WSSKeyPath,
		WSSCAPath:              cfg.Transport.WSSCAPath,
		WSSAdvertiseAddress:    cfg.Transport.WSSAdvertiseAddress,
		DNSDiscoveryURLs:       append([]string(nil), cfg.Transport.DNSDiscoveryURLs...),
		DNSDiscoveryNameServer: cfg.Transport.DNSDiscoveryNameServer,
		ReachabilityMode:       mode,
		AdvertiseAddresses:     append([]string(nil), cfg.Transport.AdvertiseAddresses...),
		Limits:                 cfg.Transport.Limits,
	}
	if err := networkapi.ValidateTransportConfig(transportConfig, time.Now()); err != nil {
		return fmt.Errorf("network participation configuration: %w", err)
	}
	return nil
}

func validateParticipationReachability(cfg Config, nodeProfile networkapi.NodeProfile) (networkapi.ReachabilityMode, error) {
	mode := cfg.Transport.ReachabilityMode
	if mode == "" {
		if nodeProfile == networkapi.NodeProfileLocalDevelopment {
			mode = networkapi.ReachabilityLocalOnly
		} else {
			mode = networkapi.ReachabilityPrivateLAN
		}
	}
	if nodeProfile == networkapi.NodeProfileLocalDevelopment && mode != networkapi.ReachabilityLocalOnly {
		return mode, fmt.Errorf("network participation configuration: node profile %q requires reachability mode %q", nodeProfile, networkapi.ReachabilityLocalOnly)
	}
	if nodeProfile == networkapi.NodeProfileConstrainedClient && mode != networkapi.ReachabilityOutboundOnly {
		return mode, fmt.Errorf("network participation configuration: node profile %q requires reachability mode %q", nodeProfile, networkapi.ReachabilityOutboundOnly)
	}
	if nodeProfile == networkapi.NodeProfileServiceNode && mode == networkapi.ReachabilityLocalOnly {
		return mode, fmt.Errorf("network participation configuration: node profile %q does not allow reachability mode %q", nodeProfile, mode)
	}
	if mode == networkapi.ReachabilityPublicDirect && isLoopbackBind(networkapi.ResolveBindAddress(cfg.Transport.BindAddress)) {
		return mode, fmt.Errorf("network participation configuration: public direct reachability requires a non-loopback bind address")
	}
	return mode, nil
}

func isLoopbackBind(address string) bool {
	if strings.EqualFold(strings.TrimSpace(address), "localhost") {
		return true
	}
	ip := net.ParseIP(strings.TrimSpace(address))
	return ip != nil && ip.IsLoopback()
}
