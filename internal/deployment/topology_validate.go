package deployment

import (
	"net/netip"
	"regexp"
	"strconv"
	"strings"

	"ardents/internal/authority"
	identityprincipal "ardents/internal/identity/principal"
	"ardents/internal/runtimeimage"

	"github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multihash"
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
	dnsNamePattern = regexp.MustCompile(
		`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$`,
	)
	// Frozen for ardents.topology/v1 from the IANA Special-Use Domain Names
	// registry. Denying all of arpa also covers its registered subdomains.
	specialUseDNSSuffixes = []string{
		"alt", "arpa", "example", "example.com", "example.net", "example.org",
		"internal", "invalid", "local", "localhost", "onion", "test",
	}
)

type ingressAddress struct {
	ip       netip.Addr
	identity string
	dns      bool
	wss      bool
}

func validateTopology(manifest topologyManifest) error {
	if manifest.APIVersion != TopologyVersion {
		return ValidationError("topology_unsupported_version")
	}
	if len(manifest.Nodes) != exactTopologyNodeCount {
		return ValidationError("topology_exactly_three_nodes_required")
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
			return ValidationError("topology_invalid_node_slot")
		}
		if !rememberUnique(slots, node.Slot) {
			return ValidationError("topology_duplicate_node_slot")
		}
		if node.Profile != "service_node" {
			return ValidationError("topology_unsupported_node_profile")
		}
		if node.Host.OS != "linux" || node.Host.Arch != "amd64" {
			return ValidationError("topology_unsupported_platform")
		}
		if node.Host.Ownership != "operator" {
			return ValidationError("topology_unsupported_host_ownership")
		}
		if !aliasPattern.MatchString(node.Host.SSHAlias) ||
			!aliasPattern.MatchString(node.Host.HostKeyPinRef) ||
			!aliasPattern.MatchString(node.NodeStateRef) {
			return ValidationError("topology_unsafe_reference")
		}
		if !rememberUnique(sshAliases, node.Host.SSHAlias) {
			return ValidationError("topology_duplicate_ssh_alias")
		}
		if node.Host.FailureDomain.Class != "host" ||
			!aliasPattern.MatchString(node.Host.FailureDomain.ID) {
			return ValidationError("topology_invalid_host_failure_domain")
		}
		if !rememberUnique(hostDomains, node.Host.FailureDomain.ID) {
			return ValidationError("topology_duplicate_host_failure_domain")
		}
		principal, err := identityprincipal.Parse(node.ExpectedNodePrincipal)
		if err != nil || principal.String() != node.ExpectedNodePrincipal {
			return ValidationError("topology_invalid_node_principal")
		}
		if !rememberUnique(principals, node.ExpectedNodePrincipal) {
			return ValidationError("topology_duplicate_node_principal")
		}
		if !validWakuPeerID(node.ExpectedWakuPeerID) {
			return ValidationError("topology_invalid_waku_peer_id")
		}
		if !rememberUnique(peerIDs, node.ExpectedWakuPeerID) {
			return ValidationError("topology_duplicate_waku_peer_id")
		}
	}
	return nil
}

func validWakuPeerID(value string) bool {
	peerID, err := multihash.FromB58String(value)
	if err != nil || peerID.B58String() != value {
		return false
	}
	decoded, err := multihash.Decode(peerID)
	if err != nil {
		return false
	}
	switch decoded.Code {
	case multihash.IDENTITY:
		return len(decoded.Digest) > 0 && len(decoded.Digest) <= 42
	case multihash.SHA2_256:
		return len(decoded.Digest) == 32
	default:
		return false
	}
}

func validateRecoveryAndProviders(nodes []nodeSpec) error {
	slots := make(map[string]struct{}, len(nodes))
	for _, node := range nodes {
		slots[node.Slot] = struct{}{}
	}
	providers := 0
	for _, node := range nodes {
		if len(node.StaticRecoveryPeers) < minimumRecoveryPeerCount {
			return ValidationError("topology_insufficient_static_peers")
		}
		peers := make(map[string]struct{}, len(node.StaticRecoveryPeers))
		for _, candidate := range node.StaticRecoveryPeers {
			if candidate == node.Slot {
				return ValidationError("topology_static_peer_self")
			}
			if _, found := slots[candidate]; !found {
				return ValidationError("topology_unknown_static_peer")
			}
			if !rememberUnique(peers, candidate) {
				return ValidationError("topology_duplicate_static_peer")
			}
		}
		switch {
		case node.Store.Persistent && !validStoreRetention(node.Store.RetentionClass):
			return ValidationError("topology_unsupported_store_retention")
		case !node.Store.Persistent && node.Store.RetentionClass != "":
			return ValidationError("topology_invalid_store_retention")
		}
		if node.Bootstrap && node.Store.Persistent {
			providers++
		}
	}
	if providers < 2 {
		return ValidationError("topology_insufficient_bootstrap_store_providers")
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
		return ValidationError("topology_unsupported_mode")
	}
	if manifest.TransportProfile != "tcp_only" && manifest.TransportProfile != "tcp_wss" {
		return ValidationError("topology_unsupported_transport_profile")
	}
	if len(manifest.SignedDNSRoots) > maxSignedDNSRoots {
		return ValidationError("topology_too_many_signed_dns_roots")
	}
	dnsRoots := make(map[string]struct{}, len(manifest.SignedDNSRoots))
	for _, root := range manifest.SignedDNSRoots {
		if len(root) > 256 || !signedDNSRootPattern.MatchString(root) {
			return ValidationError("topology_invalid_signed_dns_root")
		}
		if !rememberUnique(dnsRoots, root) {
			return ValidationError("topology_duplicate_signed_dns_root")
		}
	}
	addresses := make(map[string]struct{}, len(manifest.Nodes))
	publicIngress := 0
	for _, node := range manifest.Nodes {
		switch manifest.Mode {
		case "private_lan":
			if node.Ingress.Kind != "private_lan" {
				return ValidationError("topology_ingress_mode_mismatch")
			}
		case "public_direct":
			if node.Ingress.Kind != "public" && node.Ingress.Kind != "outbound_only" {
				return ValidationError("topology_ingress_mode_mismatch")
			}
		}
		if node.Ingress.Kind == "outbound_only" {
			if node.Ingress.Address != nil || node.Ingress.CertificateRef != nil ||
				node.Ingress.CertificateIdentity != nil {
				return ValidationError("topology_invalid_outbound_only_ingress")
			}
			continue
		}
		if node.Ingress.Address == nil {
			return ValidationError("topology_ingress_address_required")
		}
		address, ok := parseIngressAddress(*node.Ingress.Address)
		if !ok {
			return ValidationError("topology_unsupported_ingress_address")
		}
		if !rememberUnique(addresses, *node.Ingress.Address) {
			return ValidationError("topology_duplicate_ingress_address")
		}
		if node.Ingress.Kind == "private_lan" &&
			(address.dns || !admissiblePrivateAddress(address.ip)) {
			return ValidationError("topology_private_address_required")
		}
		if node.Ingress.Kind == "public" {
			if (!address.dns && !admissiblePublicAddress(address.ip)) ||
				(address.dns && !admissiblePublicDNSName(address.identity)) {
				return ValidationError("topology_public_address_required")
			}
			publicIngress++
		}
		if address.wss && manifest.TransportProfile == "tcp_only" {
			return ValidationError("topology_transport_profile_mismatch")
		}
		if err := validateCertificateBinding(node.Ingress, address); err != nil {
			return err
		}
	}
	if manifest.Mode == "public_direct" && publicIngress < 2 {
		return ValidationError("topology_insufficient_public_ingress")
	}
	return nil
}

func parseIngressAddress(value string) (ingressAddress, bool) {
	if len(value) == 0 || len(value) > 256 {
		return ingressAddress{}, false
	}
	address, err := multiaddr.NewMultiaddr(value)
	if err != nil || address.String() != value {
		return ingressAddress{}, false
	}
	protocols := address.Protocols()
	if len(protocols) != 2 && len(protocols) != 3 {
		return ingressAddress{}, false
	}
	switch protocols[0].Code {
	case multiaddr.P_IP4, multiaddr.P_IP6, multiaddr.P_DNS, multiaddr.P_DNS4, multiaddr.P_DNS6:
	default:
		return ingressAddress{}, false
	}
	if protocols[1].Code != multiaddr.P_TCP {
		return ingressAddress{}, false
	}
	wss := len(protocols) == 3
	if wss && protocols[2].Code != multiaddr.P_WSS {
		return ingressAddress{}, false
	}
	rawPort, err := address.ValueForProtocol(multiaddr.P_TCP)
	if err != nil {
		return ingressAddress{}, false
	}
	port, err := strconv.ParseUint(rawPort, 10, 16)
	if err != nil || port == 0 {
		return ingressAddress{}, false
	}
	rawIdentity, err := address.ValueForProtocol(protocols[0].Code)
	if err != nil {
		return ingressAddress{}, false
	}
	result := ingressAddress{identity: rawIdentity, wss: wss}
	if protocols[0].Code == multiaddr.P_IP4 || protocols[0].Code == multiaddr.P_IP6 {
		result.ip, err = netip.ParseAddr(rawIdentity)
		if err != nil || !result.ip.IsValid() {
			return ingressAddress{}, false
		}
		if protocols[0].Code == multiaddr.P_IP6 && result.ip.Is4In6() {
			return ingressAddress{}, false
		}
		result.ip = result.ip.Unmap()
		result.identity = result.ip.String()
		return result, true
	}
	result.dns = true
	return result, true
}

func admissiblePrivateAddress(ip netip.Addr) bool {
	return ip.IsPrivate() && !ip.IsLoopback() && !ip.IsUnspecified() &&
		!ip.IsMulticast() && !ip.IsLinkLocalUnicast()
}

func admissiblePublicAddress(ip netip.Addr) bool {
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() ||
		ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, reserved := range reservedPublicAddressPrefixes {
		if reserved.Contains(ip) {
			return false
		}
	}
	return true
}

var reservedPublicAddressPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("100::/64"),
	netip.MustParsePrefix("2001::/23"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("3fff::/20"),
}

func admissiblePublicDNSName(value string) bool {
	if len(value) > 253 || value != strings.ToLower(value) ||
		!dnsNamePattern.MatchString(value) {
		return false
	}
	for _, suffix := range specialUseDNSSuffixes {
		if value == suffix || strings.HasSuffix(value, "."+suffix) {
			return false
		}
	}
	return true
}

func validateCertificateBinding(ingress ingressSpec, address ingressAddress) error {
	if !address.wss {
		if ingress.CertificateRef != nil || ingress.CertificateIdentity != nil {
			return ValidationError("topology_unexpected_certificate_reference")
		}
		return nil
	}
	if ingress.CertificateRef == nil || ingress.CertificateIdentity == nil ||
		!aliasPattern.MatchString(*ingress.CertificateRef) ||
		*ingress.CertificateIdentity == "" {
		return ValidationError("topology_wss_certificate_required")
	}
	if *ingress.CertificateIdentity != address.identity {
		return ValidationError("topology_wss_certificate_identity_mismatch")
	}
	return nil
}

func validateAuthorityAndMaterial(manifest topologyManifest) error {
	if !aliasPattern.MatchString(manifest.OperatorSignerAlias) {
		return ValidationError("topology_unsafe_reference")
	}
	if !authority.ValidRealmID(manifest.Authority.RealmID) {
		return ValidationError("topology_invalid_authority_realm")
	}
	var authorityNode *nodeSpec
	for index := range manifest.Nodes {
		if manifest.Nodes[index].Slot == manifest.Authority.Slot {
			authorityNode = &manifest.Nodes[index]
			break
		}
	}
	if authorityNode == nil {
		return ValidationError("topology_invalid_authority_slot")
	}
	if manifest.Authority.FailureDomain.Class != "host" ||
		!aliasPattern.MatchString(manifest.Authority.FailureDomain.ID) ||
		manifest.Authority.FailureDomain != authorityNode.Host.FailureDomain {
		return ValidationError("topology_authority_failure_domain_mismatch")
	}
	if manifest.Authority.BackupFailureDomain.Class != "backup" ||
		!aliasPattern.MatchString(manifest.Authority.BackupFailureDomain.ID) {
		return ValidationError("topology_invalid_authority_backup_domain")
	}
	if manifest.CheckpointRepository.FailureDomain.Class != "external_repository" ||
		!aliasPattern.MatchString(manifest.CheckpointRepository.FailureDomain.ID) {
		return ValidationError("topology_invalid_checkpoint_domain")
	}
	if !manifest.CheckpointRepository.ImmutableHistory {
		return ValidationError("topology_checkpoint_immutable_history_required")
	}
	if manifest.CheckpointRepository.MaxHeads != checkpointHeadCapacity {
		return ValidationError("topology_checkpoint_capacity_mismatch")
	}
	if manifest.Clock.MaxSkewSeconds != maxClockSkewSeconds ||
		manifest.Clock.AuthoritySafetyMarginSeconds != authorityMarginSeconds {
		return ValidationError("topology_clock_contract_mismatch")
	}
	if err := validateOwnedReferences(manifest); err != nil {
		return err
	}
	for _, node := range manifest.Nodes {
		if !runtimeimage.ValidReference(node.Image) {
			return ValidationError("topology_immutable_image_required")
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
			return ValidationError("topology_unsafe_reference")
		}
		if !rememberUnique(references, value) {
			return ValidationError("topology_state_reference_collision")
		}
	}
	return nil
}

func rememberUnique(seen map[string]struct{}, value string) bool {
	if _, duplicate := seen[value]; duplicate {
		return false
	}
	seen[value] = struct{}{}
	return true
}
