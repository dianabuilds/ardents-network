package config

import (
	"fmt"
	"strings"
	"time"

	networkapi "ardents/internal/network/api"
)

func validateNetwork(doc Document) error {
	if strings.TrimSpace(doc.Network.StorePath) == "" {
		return fmt.Errorf("network.store_path is required")
	}
	nodeProfile := networkapi.NodeProfile(doc.Node.Profile)
	transportProfile := networkapi.TransportProfile(doc.Network.TransportProfile)
	if _, err := networkapi.ResolveNodeProfile(nodeProfile); err != nil {
		return fmt.Errorf("node.profile: %w", err)
	}
	if _, err := networkapi.ResolveProfile(transportProfile); err != nil {
		return fmt.Errorf("network.transport_profile: %w", err)
	}
	if err := networkapi.ValidateNodeProfileTransport(nodeProfile, transportProfile); err != nil {
		return fmt.Errorf("network participation configuration: %w", err)
	}
	cfg := networkapi.Config{
		NodeProfile: nodeProfile, Profile: transportProfile,
		BindAddress: doc.Network.BindAddress, ListenPort: doc.Network.ListenPort,
		WSSPort: doc.Network.WSS.Port, WSSCertPath: doc.Network.WSS.CertificateFile,
		WSSKeyPath: doc.Network.WSS.PrivateKeyFile, WSSCAPath: doc.Network.WSS.CAFile,
		WSSAdvertiseAddress:    doc.Network.WSS.AdvertiseAddress,
		DNSDiscoveryURLs:       append([]string(nil), doc.Network.DNSDiscoveryURLs...),
		DNSDiscoveryNameServer: doc.Network.DNSDiscoveryNameServer,
		ReachabilityMode:       networkapi.ReachabilityMode(doc.Network.ReachabilityMode),
		AdvertiseAddresses:     append([]string(nil), doc.Network.AdvertiseAddresses...),
		Limits:                 networkLimits(doc.Network.Limits),
	}
	if err := networkapi.ValidateTransportConfig(cfg, time.Now()); err != nil {
		return fmt.Errorf("network participation configuration: %w", err)
	}
	if doc.Node.Profile == string(networkapi.NodeProfileLocalDevelopment) &&
		doc.Network.BindAddress != "127.0.0.1" && doc.Network.BindAddress != "localhost" {
		return fmt.Errorf("local_development node profile requires a loopback bind address")
	}
	if doc.Network.DiscoveryRefreshSeconds < 1 || doc.Network.DiscoveryRefreshSeconds > 3600 {
		return fmt.Errorf("network.discovery_refresh_seconds must be between 1 and 3600")
	}
	return nil
}

func networkLimits(in NetworkLimits) networkapi.Limits {
	return networkapi.Limits{
		MaxMessageBytes: in.MaxMessageBytes, MaxPeerConnections: in.MaxPeerConnections,
		MaxConnectionsPerIP: in.MaxConnectionsPerIP, MaxConcurrentOperations: in.MaxConcurrentOperations,
		OperationRate: in.OperationRate, OperationBurst: in.OperationBurst,
		MaxFilterSubscribers: in.MaxFilterSubscribers, MaxStoreResults: in.MaxStoreResults,
	}
}
