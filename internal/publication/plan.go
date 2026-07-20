package publication

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	hostingexposure "ardents/internal/hosting/exposure"
	hostingreadiness "ardents/internal/hosting/readiness"
	hostingregistry "ardents/internal/hosting/registry"
	hostingservice "ardents/internal/hosting/service"
	networkreadiness "ardents/internal/network/readiness"
)

func publicationGatePlan(items []hostingregistry.ServiceStatus, network networkreadiness.ReachabilitySnapshot,
	allow hostingexposure.PolicyFunc) ([]hostingservice.Spec, []hostingexposure.Denial) {
	allowed := make([]hostingservice.Spec, 0, len(items))
	denied := make([]hostingexposure.Denial, 0)
	for _, item := range items {
		if item.Spec.Mode != "NetworkPublished" {
			continue
		}
		if err := publicationEligibilityError(item, network, allow); err != nil {
			denied = append(denied, hostingexposure.Denial{ID: item.Spec.ID, Err: err})
			continue
		}
		allowed = append(allowed, cloneServiceSpec(item.Spec))
	}
	return allowed, denied
}

func publicationEligibilityError(item hostingregistry.ServiceStatus, network networkreadiness.ReachabilitySnapshot,
	allow hostingexposure.PolicyFunc) error {
	if !item.Readiness.Ready || !item.Readiness.ExposureEligible {
		reason := item.Readiness.Reason
		if reason == "" {
			reason = hostingreadiness.ReasonRuntimeInactive
		}
		return fmt.Errorf("service readiness denied publication: %s", reason)
	}
	if !networkAllowsServicePublication(network) {
		return fmt.Errorf("network reachability denied service publication: %s", network.State)
	}
	if err := validateEndpointPairs(item.Spec, network.Mode); err != nil {
		return err
	}
	if allow != nil {
		return allow(item.Spec)
	}
	return nil
}

func networkAllowsServicePublication(snapshot networkreadiness.ReachabilitySnapshot) bool {
	if !snapshot.Reachable {
		return false
	}
	return snapshot.Mode == networkreadiness.ReachabilityPrivateLAN || snapshot.Mode == networkreadiness.ReachabilityPublicDirect
}

func validateEndpointPairs(spec hostingservice.Spec, mode networkreadiness.ReachabilityMode) error {
	if len(spec.Endpoints) == 0 || len(spec.ProbeEndpoints) != len(spec.Endpoints) {
		return fmt.Errorf("service endpoint and probe endpoint sets must have equal non-zero size")
	}
	for index := range spec.Endpoints {
		if err := validateEndpointPair(spec.Endpoints[index], spec.ProbeEndpoints[index], mode); err != nil {
			return fmt.Errorf("service endpoint pair %d is invalid: %w", index, err)
		}
	}
	return nil
}

func validateEndpointPair(advertised, probe string, mode networkreadiness.ReachabilityMode) error {
	publicURL, err := url.Parse(advertised)
	if err != nil || publicURL.User != nil || publicURL.Fragment != "" {
		return fmt.Errorf("invalid advertised endpoint")
	}
	probeURL, err := url.Parse(probe)
	if err != nil || probeURL.User != nil || probeURL.Fragment != "" {
		return fmt.Errorf("invalid probe endpoint")
	}
	if publicURL.Scheme != probeURL.Scheme || publicURL.Port() == "" || publicURL.Port() != probeURL.Port() {
		return fmt.Errorf("endpoint protocol or port mismatch")
	}
	if publicURL.Scheme != "http" && publicURL.Scheme != "https" && publicURL.Scheme != "tcp" {
		return fmt.Errorf("unsupported advertised endpoint scheme")
	}
	if publicURL.Scheme != "tcp" && (publicURL.EscapedPath() != probeURL.EscapedPath() || publicURL.RawQuery != probeURL.RawQuery) {
		return fmt.Errorf("endpoint HTTP target mismatch")
	}
	if !localEndpointHost(probeURL.Hostname()) || !advertisedEndpointHost(publicURL.Hostname(), mode) {
		return fmt.Errorf("endpoint address scope is invalid")
	}
	return nil
}

func localEndpointHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func advertisedEndpointHost(host string, mode networkreadiness.ReachabilityMode) bool {
	ip := net.ParseIP(host)
	if ip == nil || ip.IsUnspecified() || ip.IsLoopback() {
		return false
	}
	switch mode {
	case networkreadiness.ReachabilityPrivateLAN:
		return ip.IsPrivate() || ip.IsLinkLocalUnicast()
	case networkreadiness.ReachabilityPublicDirect:
		return !ip.IsPrivate() && !ip.IsLinkLocalUnicast()
	default:
		return false
	}
}

func cloneServiceSpec(spec hostingservice.Spec) hostingservice.Spec {
	spec.Endpoints = append([]string(nil), spec.Endpoints...)
	spec.ProbeEndpoints = append([]string(nil), spec.ProbeEndpoints...)
	return spec
}

func staticHostedServiceStatus(spec hostingservice.Spec) hostingreadiness.Status {
	return hostingreadiness.Status{
		ID: spec.ID, Type: spec.Type, Owner: spec.Owner, Mode: spec.Mode, Published: false,
		State: hostingreadiness.StateInactive, Reason: "static service has no runtime-backed publication",
		ReadinessReason: hostingreadiness.ReasonRuntimeInactive, Source: "local", Endpoints: append([]string(nil), spec.Endpoints...),
	}
}
