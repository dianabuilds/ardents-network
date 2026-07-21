package docker

import (
	"ardents/internal/workload/execution"
	"context"
	"crypto/sha256"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"

	cerrdefs "github.com/containerd/errdefs"
	containerapi "github.com/moby/moby/api/types/container"
	networkapi "github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
)

const maxWorkloadIngressPorts = 16

type containerIngressOptions struct {
	networkMode  containerapi.NetworkMode
	portBindings networkapi.PortMap
	exposedPorts networkapi.PortSet
	networking   *networkapi.NetworkingConfig
}

func (e *Executor) admitIngress(requests []execution.IngressRequest) ([]execution.IngressBinding, error) {
	bindings := make([]execution.IngressBinding, 0)
	seen := map[uint16]struct{}{}
	for _, request := range requests {
		if request.Mode != "NetworkPublished" && request.Mode != "LocalOnly" {
			continue
		}
		if len(request.Endpoints) == 0 || len(request.Endpoints) != len(request.ProbeEndpoints) {
			return nil, fmt.Errorf("hosted service ingress requires paired endpoint sets")
		}
		for index := range request.Endpoints {
			binding, err := e.admitIngressPair(request.Mode, request.Endpoints[index], request.ProbeEndpoints[index])
			if err != nil {
				return nil, err
			}
			if _, duplicate := seen[binding.Port]; duplicate {
				return nil, fmt.Errorf("hosted service ingress port is duplicated")
			}
			seen[binding.Port] = struct{}{}
			bindings = append(bindings, binding)
			if len(bindings) > maxWorkloadIngressPorts {
				return nil, fmt.Errorf("hosted service ingress exceeds port limit")
			}
		}
	}
	if len(bindings) > 0 {
		if e.ingressProxyImage == "" {
			return nil, fmt.Errorf("docker ingress proxy image is not configured")
		}
		if err := e.admitProxyImage(); err != nil {
			return nil, fmt.Errorf("docker ingress proxy image is denied: %w", err)
		}
	}
	return bindings, nil
}

func (e *Executor) admitProxyImage() error {
	if strings.HasPrefix(e.ingressProxyImage, "sha256:") && len(e.ingressProxyImage) == len("sha256:")+64 {
		return nil
	}
	return e.admitImage(e.ingressProxyImage)
}

func (e *Executor) admitIngressPair(mode, advertised, probe string) (execution.IngressBinding, error) {
	publicURL, err := parseIngressURL(advertised)
	if err != nil {
		return execution.IngressBinding{}, err
	}
	probeURL, err := parseIngressURL(probe)
	if err != nil || !isLoopbackHost(probeURL.Hostname()) {
		return execution.IngressBinding{}, fmt.Errorf("hosted service probe endpoint must use loopback")
	}
	if publicURL.Scheme != probeURL.Scheme || publicURL.Port() != probeURL.Port() ||
		(publicURL.Scheme != "tcp" && (publicURL.EscapedPath() != probeURL.EscapedPath() || publicURL.RawQuery != probeURL.RawQuery)) {
		return execution.IngressBinding{}, fmt.Errorf("hosted service endpoint pair does not match")
	}
	port64, err := strconv.ParseUint(probeURL.Port(), 10, 16)
	if err != nil || port64 < 1024 {
		return execution.IngressBinding{}, fmt.Errorf("hosted service ingress port must be between 1024 and 65535")
	}
	bindAddress := probeURL.Hostname()
	if mode == "NetworkPublished" {
		bindAddress, err = e.admitNetworkBind(publicURL.Hostname())
		if err != nil {
			return execution.IngressBinding{}, err
		}
	}
	return execution.IngressBinding{Port: uint16(port64), ProbeHost: probeURL.Hostname(), BindAddress: bindAddress}, nil
}

func parseIngressURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User != nil || parsed.Fragment != "" || parsed.Port() == "" {
		return nil, fmt.Errorf("invalid hosted service ingress endpoint")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" && parsed.Scheme != "tcp" {
		return nil, fmt.Errorf("unsupported hosted service ingress scheme")
	}
	return parsed, nil
}

func (e *Executor) admitNetworkBind(advertisedHost string) (string, error) {
	advertised := net.ParseIP(advertisedHost)
	if advertised == nil || advertised.IsUnspecified() || advertised.IsLoopback() {
		return "", fmt.Errorf("network-published Docker service requires a literal non-loopback host")
	}
	if _, allowed := e.allowedIngressHosts[strings.ToLower(advertisedHost)]; !allowed {
		return "", fmt.Errorf("hosted service advertised host is not allowed")
	}
	bind := net.ParseIP(e.ingressBindAddress)
	if bind == nil || bind.IsLoopback() {
		return "", fmt.Errorf("docker ingress bind address is not configured")
	}
	if !bind.IsUnspecified() && !bind.Equal(advertised) {
		return "", fmt.Errorf("hosted service advertised host does not match Docker ingress bind address")
	}
	return e.ingressBindAddress, nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (e *Executor) workloadIngressOptions(prepared execution.PreparedWorkload) containerIngressOptions {
	if len(prepared.Ingress) == 0 {
		return containerIngressOptions{networkMode: "none"}
	}
	name := e.workloadNetworkName(prepared)
	return containerIngressOptions{networkMode: containerapi.NetworkMode(name),
		networking: &networkapi.NetworkingConfig{EndpointsConfig: map[string]*networkapi.EndpointSettings{
			name: {},
		}}}
}

func (e *Executor) proxyIngressOptions(prepared execution.PreparedWorkload) containerIngressOptions {
	ports := make(networkapi.PortMap, len(prepared.Ingress))
	exposed := make(networkapi.PortSet, len(prepared.Ingress))
	for _, binding := range prepared.Ingress {
		port := networkapi.MustParsePort(fmt.Sprintf("%d/tcp", binding.Port))
		exposed[port] = struct{}{}
		ports[port] = dockerPortBindings(binding)
	}
	workloadNetwork := e.workloadNetworkName(prepared)
	return containerIngressOptions{networkMode: containerapi.NetworkMode(e.ingressNetworkName),
		portBindings: ports, exposedPorts: exposed,
		networking: &networkapi.NetworkingConfig{EndpointsConfig: map[string]*networkapi.EndpointSettings{
			e.ingressNetworkName: {}, workloadNetwork: {},
		}},
	}
}

func dockerPortBindings(binding execution.IngressBinding) []networkapi.PortBinding {
	bind := netip.MustParseAddr(binding.BindAddress)
	probe := netip.MustParseAddr(normalizeLoopback(binding.ProbeHost))
	items := []networkapi.PortBinding{{HostIP: bind, HostPort: strconv.Itoa(int(binding.Port))}}
	if !bind.IsUnspecified() && bind != probe {
		items = append(items, networkapi.PortBinding{HostIP: probe, HostPort: strconv.Itoa(int(binding.Port))})
	}
	return items
}

func normalizeLoopback(host string) string {
	if strings.EqualFold(host, "localhost") {
		return "127.0.0.1"
	}
	return host
}

func dockerIngressNetworkName(nodeID string) string {
	digest := sha256.Sum256([]byte(nodeID))
	return fmt.Sprintf("ardents-ingress-%x", digest[:6])
}

func (e *Executor) workloadNetworkName(prepared execution.PreparedWorkload) string {
	digest := sha256.Sum256(fmt.Appendf(nil, "%s\x00%s\x00%d", e.nodeID, prepared.WorkloadID, prepared.Generation))
	return fmt.Sprintf("ardents-workload-%x", digest[:6])
}

func (e *Executor) ensureIngressNetworks(ctx context.Context, prepared execution.PreparedWorkload) error {
	labels := map[string]string{labelNode: e.nodeID, labelWorkload: prepared.WorkloadID,
		labelGeneration: strconv.FormatInt(prepared.Generation, 10)}
	if err := e.ensureNetwork(ctx, e.workloadNetworkName(prepared), true, labels); err != nil {
		return err
	}
	return e.ensureNetwork(ctx, e.ingressNetworkName, false, map[string]string{labelNode: e.nodeID})
}

func (e *Executor) ensureNetwork(ctx context.Context, name string, internal bool, labels map[string]string) error {
	labels[labelManaged] = "network"
	if inspected, err := e.client.NetworkInspect(ctx, name, client.NetworkInspectOptions{}); err == nil {
		return validateIngressNetwork(inspected.Network.Internal, inspected.Network.Labels, e.nodeID, internal)
	} else if !cerrdefs.IsNotFound(err) {
		return dockerSafeError("inspect workload ingress network", err)
	}
	_, err := e.client.NetworkCreate(ctx, name, client.NetworkCreateOptions{
		Driver: "bridge", Internal: internal, Labels: labels,
	})
	if err != nil {
		return dockerSafeError("create workload ingress network", err)
	}
	return nil
}

func validateIngressNetwork(internal bool, labels map[string]string, nodeID string, expectedInternal bool) error {
	if internal != expectedInternal || labels[labelManaged] != "network" || labels[labelNode] != nodeID {
		return fmt.Errorf("docker ingress network identity conflicts with node policy")
	}
	return nil
}
