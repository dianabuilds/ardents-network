package projection

import (
	"time"

	appdata "ardents/internal/data"
	hostingservice "ardents/internal/hosting/service"
	apppolicy "ardents/internal/policy"
	domainworkload "ardents/internal/workload/workload"
)

type DataConfig struct {
	DefaultLocalRetentionTTL int64
	DefaultRelayRetentionTTL int64
	MaxRelayRetentionBytes   int64
	MaxLocalStorageBytes     int64
}

type PolicyConfig struct {
	MaxWorkloads                    int
	AllowedPolicyRefs               []string
	DeniedCapabilities              []string
	DisableServicePublication       bool
	DisableNetworkPublishedServices bool
	DeniedServiceTypes              []string
	DisableUntrustedRouteUse        bool
	DeniedRouteSchemes              []string
	DisableLocalBlobRetention       bool
	DisableRelayBlobRetention       bool
	DisableBlobPinning              bool
	DisablePeerBlobReserving        bool
	AllowPinRelayRetainedBlobs      bool
	AllowReservingRelayBlobs        bool
	MaxLocalRetentionTTL            int64
	MaxRelayRetentionTTL            int64
}

type ServiceConfig struct {
	ID             string
	Type           string
	Owner          string
	Mode           string
	Endpoints      []string
	ProbeEndpoints []string
}

type WorkloadConfig struct {
	ID       string
	Kind     string
	Owner    string
	Config   string
	Desired  string
	Services []ServiceConfig
}

func DataServiceConfig(cfg DataConfig) appdata.Config {
	return appdata.Config{
		DefaultLocalRetentionTTL: duration(cfg.DefaultLocalRetentionTTL),
		DefaultRelayRetentionTTL: duration(cfg.DefaultRelayRetentionTTL),
		MaxRelayRetentionBytes:   cfg.MaxRelayRetentionBytes,
		MaxLocalStorageBytes:     cfg.MaxLocalStorageBytes,
	}
}

func PolicyServiceConfig(cfg PolicyConfig) apppolicy.Config {
	return apppolicy.Config{
		MaxWorkloads:                    cfg.MaxWorkloads,
		AllowedPolicyRefs:               cloneStrings(cfg.AllowedPolicyRefs),
		DeniedCapabilities:              cloneStrings(cfg.DeniedCapabilities),
		DisableServicePublication:       cfg.DisableServicePublication,
		DisableNetworkPublishedServices: cfg.DisableNetworkPublishedServices,
		DeniedServiceTypes:              cloneStrings(cfg.DeniedServiceTypes),
		DisableUntrustedRouteUse:        cfg.DisableUntrustedRouteUse,
		DeniedRouteSchemes:              cloneStrings(cfg.DeniedRouteSchemes),
		DisableLocalBlobRetention:       cfg.DisableLocalBlobRetention,
		DisableRelayBlobRetention:       cfg.DisableRelayBlobRetention,
		DisableBlobPinning:              cfg.DisableBlobPinning,
		DisablePeerBlobReserving:        cfg.DisablePeerBlobReserving,
		AllowPinRelayRetainedBlobs:      cfg.AllowPinRelayRetainedBlobs,
		AllowReservingRelayBlobs:        cfg.AllowReservingRelayBlobs,
		MaxLocalRetentionTTL:            duration(cfg.MaxLocalRetentionTTL),
		MaxRelayRetentionTTL:            duration(cfg.MaxRelayRetentionTTL),
	}
}

func ServiceSpecs(items []ServiceConfig) []hostingservice.Spec {
	if len(items) == 0 {
		return nil
	}
	out := make([]hostingservice.Spec, 0, len(items))
	for _, item := range items {
		if item.ID == "" || item.Type == "" {
			continue
		}
		out = append(out, hostingservice.Spec{
			ID:             item.ID,
			Type:           item.Type,
			Owner:          item.Owner,
			Mode:           item.Mode,
			Endpoints:      cloneStrings(item.Endpoints),
			ProbeEndpoints: cloneStrings(item.ProbeEndpoints),
		})
	}
	return out
}

func WorkloadSpecs(items []WorkloadConfig) []domainworkload.Spec {
	if len(items) == 0 {
		return nil
	}
	out := make([]domainworkload.Spec, 0, len(items))
	for _, item := range items {
		if item.ID == "" || item.Kind == "" {
			continue
		}
		out = append(out, domainworkload.Spec{
			ID:       item.ID,
			Kind:     item.Kind,
			Owner:    item.Owner,
			Config:   item.Config,
			Desired:  item.Desired,
			Services: workloadServiceSpecs(item.Services),
		})
	}
	return out
}

func workloadServiceSpecs(items []ServiceConfig) []domainworkload.ServiceSpec {
	if len(items) == 0 {
		return nil
	}
	out := make([]domainworkload.ServiceSpec, 0, len(items))
	for _, item := range items {
		if item.ID == "" || item.Type == "" {
			continue
		}
		out = append(out, domainworkload.ServiceSpec{
			ID:             item.ID,
			Type:           item.Type,
			Mode:           item.Mode,
			Endpoints:      cloneStrings(item.Endpoints),
			ProbeEndpoints: cloneStrings(item.ProbeEndpoints),
		})
	}
	return out
}

func duration(value int64) time.Duration {
	return time.Duration(value)
}
