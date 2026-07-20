package main

import (
	"time"

	networkapi "ardents/internal/network/api"
	runtimeconfig "ardents/internal/runtime/config"
	runtimeprocess "ardents/internal/runtime/process"
)

func operatorTransportConfig(in runtimeconfig.NetworkConfig) runtimeprocess.TransportConfig {
	return runtimeprocess.TransportConfig{
		StorePath: in.StorePath, PrivateKeyPath: in.PrivateKeyPath, BindAddress: in.BindAddress,
		ListenPort: in.ListenPort, Profile: networkapi.TransportProfile(in.TransportProfile),
		WSSPort: in.WSS.Port, WSSCertPath: in.WSS.CertificateFile,
		WSSKeyPath: in.WSS.PrivateKeyFile, WSSCAPath: in.WSS.CAFile,
		WSSAdvertiseAddress:    in.WSS.AdvertiseAddress,
		DNSDiscoveryURLs:       cloneStrings(in.DNSDiscoveryURLs),
		DNSDiscoveryNameServer: in.DNSDiscoveryNameServer,
		ReachabilityMode:       networkapi.ReachabilityMode(in.ReachabilityMode),
		AdvertiseAddresses:     cloneStrings(in.AdvertiseAddresses),
		Limits: networkapi.Limits{
			MaxMessageBytes: in.Limits.MaxMessageBytes, MaxPeerConnections: in.Limits.MaxPeerConnections,
			MaxConnectionsPerIP:     in.Limits.MaxConnectionsPerIP,
			MaxConcurrentOperations: in.Limits.MaxConcurrentOperations,
			OperationRate:           in.Limits.OperationRate, OperationBurst: in.Limits.OperationBurst,
			MaxFilterSubscribers: in.Limits.MaxFilterSubscribers,
			MaxStoreResults:      in.Limits.MaxStoreResults,
		},
	}
}

func operatorPolicyConfig(doc runtimeconfig.Document) runtimeprocess.PolicyConfig {
	in := doc.Policy
	allowed := in.AllowedPolicyRefs
	if len(allowed) == 0 {
		allowed = doc.Workloads.AllowedPolicyRefs
	}
	return runtimeprocess.PolicyConfig{
		MaxWorkloads: in.MaxWorkloads, AllowedPolicyRefs: cloneStrings(allowed),
		DeniedCapabilities:              cloneStrings(in.DeniedCapabilities),
		DisableServicePublication:       in.DisableServicePublication,
		DisableNetworkPublishedServices: in.DisableNetworkPublishedServices,
		DeniedServiceTypes:              cloneStrings(in.DeniedServiceTypes),
		DisableUntrustedRouteUse:        in.DisableUntrustedRouteUse,
		DeniedRouteSchemes:              cloneStrings(in.DeniedRouteSchemes),
		DisablePrivateCapabilityUse:     in.DisablePrivateCapabilityUse,
		DeniedCapabilityScopes:          cloneStrings(in.DeniedCapabilityScopes),
		DisableLocalBlobRetention:       in.DisableLocalBlobRetention,
		DisableRelayBlobRetention:       in.DisableRelayBlobRetention,
		DisableBlobPinning:              in.DisableBlobPinning,
		DisablePeerBlobReserving:        in.DisablePeerBlobReserving,
		AllowPinRelayRetainedBlobs:      in.AllowPinRelayRetainedBlobs,
		AllowReservingRelayBlobs:        in.AllowReservingRelayBlobs,
		MaxLocalRetentionTTL:            durationOrZero(in.MaxLocalRetention),
		MaxRelayRetentionTTL:            durationOrZero(in.MaxRelayRetention),
	}
}

func durationOrZero(raw string) time.Duration {
	value, _ := time.ParseDuration(raw)
	return value
}

func operatorServiceConfigs(in []runtimeconfig.ServiceConfig) []runtimeprocess.ServiceConfig {
	out := make([]runtimeprocess.ServiceConfig, 0, len(in))
	for _, item := range in {
		out = append(out, runtimeprocess.ServiceConfig{
			ID: item.ID, Type: item.Type, Owner: item.Owner, Mode: item.Mode,
			Endpoints: cloneStrings(item.Endpoints), ProbeEndpoints: cloneStrings(item.ProbeEndpoints),
		})
	}
	return out
}

func operatorWorkloadConfigs(in []runtimeconfig.WorkloadSpec) []runtimeprocess.WorkloadConfig {
	out := make([]runtimeprocess.WorkloadConfig, 0, len(in))
	for _, item := range in {
		out = append(out, runtimeprocess.WorkloadConfig{
			ID: item.ID, Kind: item.Kind, Owner: item.Owner, Config: item.Config,
			Desired: item.Desired, Capabilities: cloneStrings(item.Capabilities),
			PolicyRef: item.PolicyRef, RestartPolicy: item.RestartPolicy,
			Services: operatorServiceConfigs(item.Services),
		})
	}
	return out
}
