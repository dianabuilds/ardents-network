package deployment

import (
	"net/netip"
	"regexp"

	identityprincipal "ardents/internal/identity/principal"

	"github.com/distribution/reference"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

const (
	exactTopologyNodeCount   = 3
	checkpointHeadCapacity   = 65_536
	maxClockSkewSeconds      = 30
	authorityMarginSeconds   = 60
	minimumRecoveryPeerCount = 2
	maxSignedDNSRoots        = 4
)

var (
	slotPattern          = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)
	aliasPattern         = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,127}$`)
	signedDNSRootPattern = regexp.MustCompile(
		`^enrtree://[A-Z2-7]{53}@[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$`,
	)
)

func validateTopology(manifest topologyManifest) error {
	if manifest.APIVersion != TopologyVersion {
		return ValidationError("topology_unsupported_version")
	}
	if len(manifest.Nodes) != exactTopologyNodeCount {
		return validationError("topology_exactly_three_nodes_required")
	}
	if err := validateTopologyNodes(manifest.Nodes); err != nil {
		return err
	}
	if err := validateRecoveryAndProviders(manifest.Nodes); err != nil {
		return err
	}
	if err := validateModeAndIngress(manifest); err != nil {
		return err
	}
	return validateAuthorityAndMaterial(manifest)
}

func validateTopologyNodes(nodes []nodeSpec) error {
	slots := make(map[string]struct{}, len(nodes))
	sshAliases := make(map[string]struct{}, len(nodes))
	hostDomains := make(map[string]struct{}, len(nodes))
	principals := make(map[string]struct{}, len(nodes))
	peerIDs := make(map[string]struct{}, len(nodes))
	for _, node := range nodes {
		if !slotPattern.MatchString(node.Slot) {
			return validationError("topology_invalid_node_slot")
		}
		if !rememberUnique(slots, node.Slot) {
			return validationError("topology_duplicate_node_slot")
		}
		if node.Profile != "service_node" {
			return validationError("topology_unsupported_node_profile")
		}
		if node.Host.OS != "linux" || node.Host.Arch != "amd64" {
			return validationError("topology_unsupported_platform")
		}
		if node.Host.Ownership != "operator" {
			return validationError("topology_unsupported_host_ownership")
		}
		if !aliasPattern.MatchString(node.Host.SSHAlias) ||
			!aliasPattern.MatchString(node.Host.HostKeyPinRef) ||
			!aliasPattern.MatchString(node.NodeStateRef) {
			return validationError("topology_unsafe_reference")
		}
		if !rememberUnique(sshAliases, node.Host.SSHAlias) {
			return validationError("topology_duplicate_ssh_alias")
		}
		if node.Host.FailureDomain.Class != "host" ||
			!aliasPattern.MatchString(node.Host.FailureDomain.ID) {
			return validationError("topology_invalid_host_failure_domain")
		}
		if !rememberUnique(hostDomains, node.Host.FailureDomain.ID) {
			return validationError("topology_duplicate_host_failure_domain")
		}
		principal, err := identityprincipal.Parse(node.ExpectedNodePrincipal)
		if err != nil || principal.String() != node.ExpectedNodePrincipal {
			return validationError("topology_invalid_node_principal")
		}
		if !rememberUnique(principals, node.ExpectedNodePrincipal) {
			return validationError("topology_duplicate_node_principal")
		}
		peerID, err := peer.Decode(node.ExpectedWakuPeerID)
		if err != nil || peerID.String() != node.ExpectedWakuPeerID {
			return validationError("topology_invalid_waku_peer_id")
		}
		if !rememberUnique(peerIDs, node.ExpectedWakuPeerID) {
			return validationError("topology_duplicate_waku_peer_id")
		}
	}
	return nil
}

func validateRecoveryAndProviders(nodes []nodeSpec) error {
	slots := make(map[string]struct{}, len(nodes))
	for _, node := range nodes {
		slots[node.Slot] = struct{}{}
	}
	providers := 0
	for _, node := range nodes {
		if len(node.StaticRecoveryPeers) < minimumRecoveryPeerCount {
			return validationError("topology_insufficient_static_peers")
		}
		peers := make(map[string]struct{}, len(node.StaticRecoveryPeers))
		for _, candidate := range node.StaticRecoveryPeers {
			if candidate == node.Slot {
				return validationError("topology_static_peer_self")
			}
			if _, found := slots[candidate]; !found {
				return validationError("topology_unknown_static_peer")
			}
			if !rememberUnique(peers, candidate) {
				return validationError("topology_duplicate_static_peer")
			}
		}
		switch {
		case node.Store.Persistent && !validStoreRetention(node.Store.RetentionClass):
			return validationError("topology_unsupported_store_retention")
		case !node.Store.Persistent && node.Store.RetentionClass != "":
			return validationError("topology_invalid_store_retention")
		}
		if node.Bootstrap && node.Store.Persistent {
			providers++
		}
	}
	if providers < 2 {
		return validationError("topology_insufficient_bootstrap_store_providers")
	}
	return nil
}

func validStoreRetention(value string) bool {
	switch value {
	case "bounded_24h", "bounded_7d", "bounded_30d":
		return true
	default:
		return false
	}
}

func validateModeAndIngress(manifest topologyManifest) error {
	switch manifest.Mode {
	case "private_lan", "public_direct":
	default:
		return validationError("topology_unsupported_mode")
	}
	if manifest.TransportProfile != "tcp_only" && manifest.TransportProfile != "tcp_wss" {
		return validationError("topology_unsupported_transport_profile")
	}
	if len(manifest.SignedDNSRoots) > maxSignedDNSRoots {
		return validationError("topology_too_many_signed_dns_roots")
	}
	dnsRoots := make(map[string]struct{}, len(manifest.SignedDNSRoots))
	for _, root := range manifest.SignedDNSRoots {
		if len(root) > 256 || !signedDNSRootPattern.MatchString(root) {
			return validationError("topology_invalid_signed_dns_root")
		}
		if !rememberUnique(dnsRoots, root) {
			return validationError("topology_duplicate_signed_dns_root")
		}
	}
	addresses := make(map[string]struct{}, len(manifest.Nodes))
	publicIngress := 0
	for _, node := range manifest.Nodes {
		switch manifest.Mode {
		case "private_lan":
			if node.Ingress.Kind != "private_lan" {
				return validationError("topology_ingress_mode_mismatch")
			}
		case "public_direct":
			if node.Ingress.Kind != "public" && node.Ingress.Kind != "outbound_only" {
				return validationError("topology_ingress_mode_mismatch")
			}
		}
		if node.Ingress.Kind == "outbound_only" {
			if node.Ingress.Address != "" || node.Ingress.CertificateRef != "" ||
				node.Ingress.CertificateIdentity != "" {
				return validationError("topology_invalid_outbound_only_ingress")
			}
			continue
		}
		ip, wss, ok := parseIngressAddress(node.Ingress.Address)
		if !ok {
			return validationError("topology_unsupported_ingress_address")
		}
		if !rememberUnique(addresses, node.Ingress.Address) {
			return validationError("topology_duplicate_ingress_address")
		}
		if node.Ingress.Kind == "private_lan" && !admissiblePrivateAddress(ip) {
			return validationError("topology_private_address_required")
		}
		if node.Ingress.Kind == "public" {
			if !admissiblePublicAddress(ip) {
				return validationError("topology_public_address_required")
			}
			publicIngress++
		}
		if wss && manifest.TransportProfile == "tcp_only" {
			return validationError("topology_transport_profile_mismatch")
		}
		if err := validateCertificateBinding(node.Ingress, ip, wss); err != nil {
			return err
		}
	}
	if manifest.Mode == "public_direct" && publicIngress < 2 {
		return validationError("topology_insufficient_public_ingress")
	}
	return nil
}

func parseIngressAddress(value string) (netip.Addr, bool, bool) {
	if len(value) == 0 || len(value) > 256 {
		return netip.Addr{}, false, false
	}
	address, err := multiaddr.NewMultiaddr(value)
	if err != nil || address.String() != value {
		return netip.Addr{}, false, false
	}
	protocols := address.Protocols()
	if len(protocols) != 2 && len(protocols) != 3 {
		return netip.Addr{}, false, false
	}
	if (protocols[0].Code != multiaddr.P_IP4 && protocols[0].Code != multiaddr.P_IP6) ||
		protocols[1].Code != multiaddr.P_TCP {
		return netip.Addr{}, false, false
	}
	wss := len(protocols) == 3
	if wss && protocols[2].Code != multiaddr.P_WSS {
		return netip.Addr{}, false, false
	}
	rawIP, err := address.ValueForProtocol(protocols[0].Code)
	if err != nil {
		return netip.Addr{}, false, false
	}
	ip, err := netip.ParseAddr(rawIP)
	if err != nil || !ip.IsValid() {
		return netip.Addr{}, false, false
	}
	return ip.Unmap(), wss, true
}

func admissiblePrivateAddress(ip netip.Addr) bool {
	return ip.IsPrivate() && !ip.IsLoopback() && !ip.IsUnspecified() &&
		!ip.IsMulticast() && !ip.IsLinkLocalUnicast()
}

func admissiblePublicAddress(ip netip.Addr) bool {
	return ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() &&
		!ip.IsUnspecified() && !ip.IsMulticast() && !ip.IsLinkLocalUnicast()
}

func validateCertificateBinding(ingress ingressSpec, ip netip.Addr, wss bool) error {
	if !wss {
		if ingress.CertificateRef != "" || ingress.CertificateIdentity != "" {
			return validationError("topology_unexpected_certificate_reference")
		}
		return nil
	}
	if !aliasPattern.MatchString(ingress.CertificateRef) || ingress.CertificateIdentity == "" {
		return validationError("topology_wss_certificate_required")
	}
	if ingress.CertificateIdentity != ip.String() {
		return validationError("topology_wss_certificate_identity_mismatch")
	}
	return nil
}

func validateAuthorityAndMaterial(manifest topologyManifest) error {
	if !aliasPattern.MatchString(manifest.OperatorSignerAlias) {
		return validationError("topology_unsafe_reference")
	}
	var authorityNode *nodeSpec
	for index := range manifest.Nodes {
		if manifest.Nodes[index].Slot == manifest.Authority.Slot {
			authorityNode = &manifest.Nodes[index]
			break
		}
	}
	if authorityNode == nil {
		return validationError("topology_invalid_authority_slot")
	}
	if manifest.Authority.FailureDomain.Class != "host" ||
		!aliasPattern.MatchString(manifest.Authority.FailureDomain.ID) ||
		manifest.Authority.FailureDomain != authorityNode.Host.FailureDomain {
		return validationError("topology_authority_failure_domain_mismatch")
	}
	if manifest.Authority.BackupFailureDomain.Class != "backup" ||
		!aliasPattern.MatchString(manifest.Authority.BackupFailureDomain.ID) {
		return validationError("topology_invalid_authority_backup_domain")
	}
	if manifest.CheckpointRepository.FailureDomain.Class != "external_repository" ||
		!aliasPattern.MatchString(manifest.CheckpointRepository.FailureDomain.ID) {
		return validationError("topology_invalid_checkpoint_domain")
	}
	if !manifest.CheckpointRepository.ImmutableHistory {
		return validationError("topology_checkpoint_immutable_history_required")
	}
	if manifest.CheckpointRepository.MaxHeads != checkpointHeadCapacity {
		return validationError("topology_checkpoint_capacity_mismatch")
	}
	if manifest.Clock.MaxSkewSeconds != maxClockSkewSeconds ||
		manifest.Clock.AuthoritySafetyMarginSeconds != authorityMarginSeconds {
		return validationError("topology_clock_contract_mismatch")
	}
	if err := validateOwnedReferences(manifest); err != nil {
		return err
	}
	for _, node := range manifest.Nodes {
		if !validImmutableImage(node.Image) {
			return validationError("topology_immutable_image_required")
		}
	}
	return nil
}

func validateOwnedReferences(manifest topologyManifest) error {
	references := make(map[string]struct{}, len(manifest.Nodes)*2+3)
	values := []string{
		manifest.Authority.StateRef,
		manifest.Authority.BackupRef,
		manifest.CheckpointRepository.Reference,
	}
	for _, node := range manifest.Nodes {
		values = append(values, node.NodeStateRef, node.Host.HostKeyPinRef)
	}
	for _, value := range values {
		if !aliasPattern.MatchString(value) {
			return validationError("topology_unsafe_reference")
		}
		if !rememberUnique(references, value) {
			return validationError("topology_state_reference_collision")
		}
	}
	return nil
}

func validImmutableImage(value string) bool {
	if len(value) == 0 || len(value) > 512 {
		return false
	}
	named, err := reference.ParseNormalizedNamed(value)
	if err != nil || named.String() != value {
		return false
	}
	digested, ok := named.(reference.Digested)
	return ok && digested.Digest().Algorithm().String() == "sha256"
}

func rememberUnique(seen map[string]struct{}, value string) bool {
	if _, duplicate := seen[value]; duplicate {
		return false
	}
	seen[value] = struct{}{}
	return true
}
