package deployment

import (
	"sort"
)

// MaxTopologyBytes is the fail-closed input bound for ardents.topology/v1.
const MaxTopologyBytes = 256 << 10

// Compile strictly admits one topology manifest and returns only its redacted,
// deterministic host-local plan.
func Compile(raw []byte) (Plan, error) {
	manifest, err := decodeTopology(raw)
	if err != nil {
		return Plan{}, err
	}
	if err := validateTopology(manifest); err != nil {
		return Plan{}, err
	}
	return compilePlan(manifest), nil
}

func compilePlan(manifest topologyManifest) Plan {
	nodes := append([]nodeSpec(nil), manifest.Nodes...)
	sort.Slice(nodes, func(left, right int) bool {
		return nodes[left].Slot < nodes[right].Slot
	})
	hosts := make([]HostPlan, 0, len(nodes))
	for _, node := range nodes {
		role := "member"
		if node.Slot == manifest.Authority.Slot {
			role = "authority"
		}
		peers := append([]string(nil), node.StaticRecoveryPeers...)
		sort.Strings(peers)
		hosts = append(hosts, HostPlan{
			Slot: node.Slot, Role: role, Profile: node.Profile,
			TransportProfile:    manifest.TransportProfile,
			Ingress:             compileIngress(node.Ingress.Kind),
			Bootstrap:           node.Bootstrap,
			PersistentStore:     node.Store.Persistent,
			StoreRetentionClass: node.Store.RetentionClass,
			StaticRecoveryPeers: peers,
			Checks:              compileChecks(node.Ingress),
		})
	}
	return Plan{
		APIVersion: PlanVersion, Mode: manifest.Mode,
		TransportProfile:   manifest.TransportProfile,
		SignedDNSRootCount: len(manifest.SignedDNSRoots),
		Authority: AuthorityPlan{
			Slot: manifest.Authority.Slot, SeparateConsistencyGroup: true,
			IndependentBackup: true, IndependentCheckpointRepository: true,
			CheckpointMaxHeads:           manifest.CheckpointRepository.MaxHeads,
			MaxClockSkewSeconds:          manifest.Clock.MaxSkewSeconds,
			AuthoritySafetyMarginSeconds: manifest.Clock.AuthoritySafetyMarginSeconds,
		},
		Hosts: hosts,
	}
}

func compileIngress(kind string) string {
	switch kind {
	case "private_lan":
		return "private_probe_required"
	case "public":
		return "public_autonat_required"
	default:
		return "outbound_only"
	}
}

func compileChecks(ingress ingressSpec) []string {
	checks := []string{
		"clock_sync_required",
		"host_key_pin_required",
		"identity_binding_required",
		"image_digest_required",
	}
	switch ingress.Kind {
	case "private_lan":
		checks = append(checks, "private_cross_host_probe_required")
	case "public":
		checks = append(checks, "public_autonat_required")
	}
	if ingress.CertificateRef != nil {
		checks = append(checks, "wss_certificate_identity_required")
	}
	return checks
}
