package deployment

import (
	"bytes"
	"encoding/json"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCompilePrivateLANTopologyProducesDeterministicRedactedPlans(t *testing.T) {
	raw := readTopologyFixture(t, "private-lan.json")

	plan, err := Compile(raw)
	require.NoError(t, err)

	actual, err := json.MarshalIndent(plan, "", "  ")
	require.NoError(t, err)
	expected := readTopologyFixture(t, "private-lan-plan.json")
	require.JSONEq(t, string(expected), string(actual))
}

func TestCompilePublicDirectTopologyKeepsEvidenceRequirementsDistinct(t *testing.T) {
	raw := readTopologyFixture(t, "public-direct.json")

	plan, err := Compile(raw)
	require.NoError(t, err)

	actual, err := json.MarshalIndent(plan, "", "  ")
	require.NoError(t, err)
	expected := readTopologyFixture(t, "public-direct-plan.json")
	require.JSONEq(t, string(expected), string(actual))
}

func TestCompileAcceptsCompleteModeTransportAndCertificateIdentityMatrix(t *testing.T) {
	privateLAN := readTopologyFixture(t, "private-lan.json")
	publicDirect := readTopologyFixture(t, "public-direct.json")
	tests := []struct {
		name               string
		raw                []byte
		mutate             func(map[string]any)
		wantTransport      string
		wantCertificateRun bool
	}{
		{
			name: "public direct tcp only",
			raw:  publicDirect,
			mutate: func(root map[string]any) {
				root["transport_profile"] = "tcp_only"
				ingress := topologyNode(root, 1)["ingress"].(map[string]any)
				ingress["address"] = "/dns4/node-a.ardents.net/tcp/60000"
				delete(ingress, "certificate_ref")
				delete(ingress, "certificate_identity")
			},
			wantTransport: "tcp_only",
		},
		{
			name: "private lan wss exact ip id",
			raw:  privateLAN,
			mutate: func(root map[string]any) {
				root["transport_profile"] = "tcp_wss"
				for index := range root["nodes"].([]any) {
					ingress := topologyNode(root, index)["ingress"].(map[string]any)
					address := ingress["address"].(string)
					ip := strings.Split(address, "/")[2]
					ingress["address"] = address + "/wss"
					ingress["certificate_ref"] = "wss-certificate-" + topologyNode(root, index)["slot"].(string)
					ingress["certificate_identity"] = ip
				}
			},
			wantTransport:      "tcp_wss",
			wantCertificateRun: true,
		},
		{
			name: "public direct wss exact ip id",
			raw:  publicDirect,
			mutate: func(root map[string]any) {
				ingress := topologyNode(root, 1)["ingress"].(map[string]any)
				ingress["address"] = "/ip4/8.8.8.8/tcp/443/wss"
				ingress["certificate_identity"] = "8.8.8.8"
			},
			wantTransport:      "tcp_wss",
			wantCertificateRun: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := Compile(mutateTopology(t, tc.raw, tc.mutate))
			require.NoError(t, err)
			require.Equal(t, tc.wantTransport, plan.TransportProfile)
			foundCertificateCheck := false
			for _, host := range plan.Hosts {
				if slices.Contains(host.Checks, "wss_certificate_identity_required") {
					foundCertificateCheck = true
				}
			}
			require.Equal(t, tc.wantCertificateRun, foundCertificateCheck)
		})
	}
}

func TestCompileRejectsNonCanonicalManifestEnvelopeWithStableErrors(t *testing.T) {
	valid := readTopologyFixture(t, "private-lan.json")
	tests := []struct {
		name string
		raw  []byte
		code string
	}{
		{
			name: "unknown field",
			raw:  bytes.Replace(valid, []byte(`"mode": "private_lan"`), []byte(`"unexpected": true, "mode": "private_lan"`), 1),
			code: "topology_unknown_field",
		},
		{
			name: "duplicate field",
			raw:  bytes.Replace(valid, []byte(`"mode": "private_lan"`), []byte(`"mode": "private_lan", "mode": "private_lan"`), 1),
			code: "topology_duplicate_field",
		},
		{
			name: "case folded field alias",
			raw:  bytes.Replace(valid, []byte(`"api_version"`), []byte(`"API_VERSION"`), 1),
			code: "topology_unknown_field",
		},
		{
			name: "case folded duplicate bypass",
			raw: bytes.Replace(
				valid,
				[]byte(`"api_version": "ardents.topology/v1"`),
				[]byte(`"api_version": "ardents.topology/v1", "API_VERSION": "ardents.topology/v1"`),
				1,
			),
			code: "topology_unknown_field",
		},
		{
			name: "unicode case folded field alias",
			raw:  bytes.Replace(valid, []byte(`"host": {`), []byte(`"hoſt": {`), 1),
			code: "topology_unknown_field",
		},
		{
			name: "trailing value",
			raw:  append(append([]byte(nil), valid...), []byte("\n{}")...),
			code: "topology_trailing_value",
		},
		{
			name: "unsupported version",
			raw:  bytes.Replace(valid, []byte(TopologyVersion), []byte("ardents.topology/v2"), 1),
			code: "topology_unsupported_version",
		},
		{
			name: "invalid json",
			raw:  []byte(`{"api_version":`),
			code: "topology_invalid_json",
		},
		{
			name: "oversized",
			raw:  []byte(strings.Repeat(" ", MaxTopologyBytes+1)),
			code: "topology_manifest_too_large",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Compile(tc.raw)
			require.EqualError(t, err, tc.code)
		})
	}
}

func TestCompileRejectsNullScalarsAndNonCanonicalIntegers(t *testing.T) {
	privateLAN := readTopologyFixture(t, "private-lan.json")
	publicDirect := readTopologyFixture(t, "public-direct.json")
	tests := []struct {
		name string
		raw  []byte
	}{
		{
			name: "null boolean",
			raw: mutateTopology(t, privateLAN, func(root map[string]any) {
				topologyNode(root, 0)["bootstrap"] = nil
			}),
		},
		{
			name: "null string",
			raw: mutateTopology(t, publicDirect, func(root map[string]any) {
				topologyNode(root, 1)["ingress"].(map[string]any)["address"] = nil
			}),
		},
		{
			name: "null reference",
			raw: mutateTopology(t, publicDirect, func(root map[string]any) {
				topologyNode(root, 1)["ingress"].(map[string]any)["certificate_ref"] = nil
			}),
		},
		{
			name: "exponent integer",
			raw:  bytes.Replace(privateLAN, []byte(`"max_skew_seconds": 30`), []byte(`"max_skew_seconds": 3e1`), 1),
		},
		{
			name: "decimal integer",
			raw:  bytes.Replace(privateLAN, []byte(`"max_skew_seconds": 30`), []byte(`"max_skew_seconds": 30.0`), 1),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Compile(tc.raw)
			require.EqualError(t, err, "topology_invalid_json")
		})
	}
}

func TestCompileRejectsMissingRequiredFields(t *testing.T) {
	valid := readTopologyFixture(t, "private-lan.json")
	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{
			name: "top level signed dns roots",
			mutate: func(root map[string]any) {
				delete(root, "signed_dns_roots")
			},
		},
		{
			name: "zero valued bootstrap",
			mutate: func(root map[string]any) {
				delete(topologyNode(root, 0), "bootstrap")
			},
		},
		{
			name: "nonprovider store",
			mutate: func(root map[string]any) {
				delete(topologyNode(root, 0), "store")
			},
		},
		{
			name: "nested host ownership",
			mutate: func(root map[string]any) {
				delete(topologyHost(root, 0), "ownership")
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Compile(mutateTopology(t, valid, tc.mutate))
			require.EqualError(t, err, "topology_missing_field")
		})
	}
}

func TestCompileRejectsUnsupportedHostsAndDuplicateIdentityBindings(t *testing.T) {
	valid := readTopologyFixture(t, "private-lan.json")
	tests := []struct {
		name   string
		mutate func(map[string]any)
		code   string
	}{
		{
			name: "requires exactly three nodes",
			mutate: func(root map[string]any) {
				root["nodes"] = root["nodes"].([]any)[:2]
			},
			code: "topology_exactly_three_nodes_required",
		},
		{
			name: "service node profile only",
			mutate: func(root map[string]any) {
				topologyNode(root, 0)["profile"] = "client_node"
			},
			code: "topology_unsupported_node_profile",
		},
		{
			name: "linux amd64 only",
			mutate: func(root map[string]any) {
				topologyHost(root, 0)["os"] = "windows"
			},
			code: "topology_unsupported_platform",
		},
		{
			name: "operator owned hosts only",
			mutate: func(root map[string]any) {
				topologyHost(root, 0)["ownership"] = "provider"
			},
			code: "topology_unsupported_host_ownership",
		},
		{
			name: "duplicate node slot",
			mutate: func(root map[string]any) {
				topologyNode(root, 1)["slot"] = topologyNode(root, 0)["slot"]
			},
			code: "topology_duplicate_node_slot",
		},
		{
			name: "duplicate ssh alias",
			mutate: func(root map[string]any) {
				topologyHost(root, 1)["ssh_alias"] = topologyHost(root, 0)["ssh_alias"]
			},
			code: "topology_duplicate_ssh_alias",
		},
		{
			name: "duplicate host failure domain",
			mutate: func(root map[string]any) {
				topologyFailureDomain(topologyHost(root, 1), "failure_domain")["id"] =
					topologyFailureDomain(topologyHost(root, 0), "failure_domain")["id"]
			},
			code: "topology_duplicate_host_failure_domain",
		},
		{
			name: "invalid node principal",
			mutate: func(root map[string]any) {
				topologyNode(root, 0)["expected_node_principal"] = "node-a"
			},
			code: "topology_invalid_node_principal",
		},
		{
			name: "duplicate node principal",
			mutate: func(root map[string]any) {
				topologyNode(root, 1)["expected_node_principal"] =
					topologyNode(root, 0)["expected_node_principal"]
			},
			code: "topology_duplicate_node_principal",
		},
		{
			name: "invalid waku peer id",
			mutate: func(root map[string]any) {
				topologyNode(root, 0)["expected_waku_peer_id"] = "peer-a"
			},
			code: "topology_invalid_waku_peer_id",
		},
		{
			name: "duplicate waku peer id",
			mutate: func(root map[string]any) {
				topologyNode(root, 1)["expected_waku_peer_id"] =
					topologyNode(root, 0)["expected_waku_peer_id"]
			},
			code: "topology_duplicate_waku_peer_id",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Compile(mutateTopology(t, valid, tc.mutate))
			require.EqualError(t, err, tc.code)
		})
	}
}

func TestCompileRejectsInvalidRecoveryAuthorityAndImmutableMaterial(t *testing.T) {
	valid := readTopologyFixture(t, "private-lan.json")
	tests := []struct {
		name   string
		mutate func(map[string]any)
		code   string
	}{
		{
			name: "self recovery peer",
			mutate: func(root map[string]any) {
				topologyNode(root, 0)["static_recovery_peers"] = []any{topologyNode(root, 0)["slot"], topologyNode(root, 1)["slot"]}
			},
			code: "topology_static_peer_self",
		},
		{
			name: "duplicate recovery peer",
			mutate: func(root map[string]any) {
				peer := topologyNode(root, 1)["slot"]
				topologyNode(root, 0)["static_recovery_peers"] = []any{peer, peer}
			},
			code: "topology_duplicate_static_peer",
		},
		{
			name: "unknown recovery peer",
			mutate: func(root map[string]any) {
				topologyNode(root, 0)["static_recovery_peers"] = []any{"node-unknown", topologyNode(root, 1)["slot"]}
			},
			code: "topology_unknown_static_peer",
		},
		{
			name: "insufficient recovery peers",
			mutate: func(root map[string]any) {
				topologyNode(root, 0)["static_recovery_peers"] = []any{topologyNode(root, 1)["slot"]}
			},
			code: "topology_insufficient_static_peers",
		},
		{
			name: "insufficient bootstrap and store providers",
			mutate: func(root map[string]any) {
				topologyNode(root, 1)["bootstrap"] = false
			},
			code: "topology_insufficient_bootstrap_store_providers",
		},
		{
			name: "invalid store retention",
			mutate: func(root map[string]any) {
				topologyNode(root, 1)["store"].(map[string]any)["retention_class"] = "unbounded"
			},
			code: "topology_unsupported_store_retention",
		},
		{
			name: "disabled store has retention",
			mutate: func(root map[string]any) {
				topologyNode(root, 0)["store"].(map[string]any)["retention_class"] = "bounded_7d"
			},
			code: "topology_invalid_store_retention",
		},
		{
			name: "unknown authority slot",
			mutate: func(root map[string]any) {
				root["authority"].(map[string]any)["slot"] = "node-unknown"
			},
			code: "topology_invalid_authority_slot",
		},
		{
			name: "authority host domain mismatch",
			mutate: func(root map[string]any) {
				topologyFailureDomain(root["authority"].(map[string]any), "failure_domain")["id"] = "host-b"
			},
			code: "topology_authority_failure_domain_mismatch",
		},
		{
			name: "authority backup domain class",
			mutate: func(root map[string]any) {
				topologyFailureDomain(root["authority"].(map[string]any), "backup_failure_domain")["class"] = "host"
			},
			code: "topology_invalid_authority_backup_domain",
		},
		{
			name: "checkpoint domain class",
			mutate: func(root map[string]any) {
				topologyFailureDomain(root["checkpoint_repository"].(map[string]any), "failure_domain")["class"] = "backup"
			},
			code: "topology_invalid_checkpoint_domain",
		},
		{
			name: "state reference collision",
			mutate: func(root map[string]any) {
				root["authority"].(map[string]any)["state_ref"] = topologyNode(root, 0)["node_state_ref"]
			},
			code: "topology_state_reference_collision",
		},
		{
			name: "checkpoint mutable history",
			mutate: func(root map[string]any) {
				root["checkpoint_repository"].(map[string]any)["immutable_history"] = false
			},
			code: "topology_checkpoint_immutable_history_required",
		},
		{
			name: "checkpoint capacity",
			mutate: func(root map[string]any) {
				root["checkpoint_repository"].(map[string]any)["max_heads"] = float64(65535)
			},
			code: "topology_checkpoint_capacity_mismatch",
		},
		{
			name: "clock skew",
			mutate: func(root map[string]any) {
				root["clock"].(map[string]any)["max_skew_seconds"] = float64(31)
			},
			code: "topology_clock_contract_mismatch",
		},
		{
			name: "authority margin",
			mutate: func(root map[string]any) {
				root["clock"].(map[string]any)["authority_safety_margin_seconds"] = float64(59)
			},
			code: "topology_clock_contract_mismatch",
		},
		{
			name: "unsafe signer reference",
			mutate: func(root map[string]any) {
				root["operator_signer_alias"] = "../operator.key"
			},
			code: "topology_unsafe_reference",
		},
		{
			name: "mutable image tag",
			mutate: func(root map[string]any) {
				topologyNode(root, 0)["image"] = "registry.example/ardents/node:latest"
			},
			code: "topology_immutable_image_required",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Compile(mutateTopology(t, valid, tc.mutate))
			require.EqualError(t, err, tc.code)
		})
	}
}

func TestCompileRejectsMixedUnsafeOrUnprovableIngress(t *testing.T) {
	privateLAN := readTopologyFixture(t, "private-lan.json")
	publicDirect := readTopologyFixture(t, "public-direct.json")
	tests := []struct {
		name   string
		raw    []byte
		mutate func(map[string]any)
		code   string
	}{
		{
			name: "unsupported mode",
			raw:  privateLAN,
			mutate: func(root map[string]any) {
				root["mode"] = "automatic_nat"
			},
			code: "topology_unsupported_mode",
		},
		{
			name: "unsupported transport",
			raw:  privateLAN,
			mutate: func(root map[string]any) {
				root["transport_profile"] = "quic"
			},
			code: "topology_unsupported_transport_profile",
		},
		{
			name: "mixed ingress mode",
			raw:  publicDirect,
			mutate: func(root map[string]any) {
				topologyNode(root, 0)["ingress"].(map[string]any)["kind"] = "private_lan"
			},
			code: "topology_ingress_mode_mismatch",
		},
		{
			name: "private lan requires private address",
			raw:  privateLAN,
			mutate: func(root map[string]any) {
				topologyNode(root, 0)["ingress"].(map[string]any)["address"] = "/ip4/203.0.113.13/tcp/60000"
			},
			code: "topology_private_address_required",
		},
		{
			name: "public ingress requires public address",
			raw:  publicDirect,
			mutate: func(root map[string]any) {
				topologyNode(root, 1)["ingress"].(map[string]any)["address"] = "/ip4/10.23.0.11/tcp/443/wss"
				topologyNode(root, 1)["ingress"].(map[string]any)["certificate_identity"] = "10.23.0.11"
			},
			code: "topology_public_address_required",
		},
		{
			name: "cgnat is not public ingress",
			raw:  publicDirect,
			mutate: func(root map[string]any) {
				topologyNode(root, 2)["ingress"].(map[string]any)["address"] = "/ip4/100.64.0.12/tcp/60000"
			},
			code: "topology_public_address_required",
		},
		{
			name: "documentation range is not public ingress",
			raw:  publicDirect,
			mutate: func(root map[string]any) {
				topologyNode(root, 2)["ingress"].(map[string]any)["address"] = "/ip4/198.51.100.12/tcp/60000"
			},
			code: "topology_public_address_required",
		},
		{
			name: "reserved dns name is not public ingress",
			raw:  publicDirect,
			mutate: func(root map[string]any) {
				ingress := topologyNode(root, 1)["ingress"].(map[string]any)
				ingress["address"] = "/dns4/node-a.example/tcp/443/wss"
				ingress["certificate_identity"] = "node-a.example"
			},
			code: "topology_public_address_required",
		},
		{
			name: "special use onion name is not public ingress",
			raw:  publicDirect,
			mutate: func(root map[string]any) {
				ingress := topologyNode(root, 1)["ingress"].(map[string]any)
				ingress["address"] = "/dns4/node-a.onion/tcp/443/wss"
				ingress["certificate_identity"] = "node-a.onion"
			},
			code: "topology_public_address_required",
		},
		{
			name: "special use alt name is not public ingress",
			raw:  publicDirect,
			mutate: func(root map[string]any) {
				ingress := topologyNode(root, 1)["ingress"].(map[string]any)
				ingress["address"] = "/dns4/node-a.alt/tcp/443/wss"
				ingress["certificate_identity"] = "node-a.alt"
			},
			code: "topology_public_address_required",
		},
		{
			name: "special use arpa subtree is not public ingress",
			raw:  publicDirect,
			mutate: func(root map[string]any) {
				ingress := topologyNode(root, 1)["ingress"].(map[string]any)
				ingress["address"] = "/dns4/node-a.home.arpa/tcp/443/wss"
				ingress["certificate_identity"] = "node-a.home.arpa"
			},
			code: "topology_public_address_required",
		},
		{
			name: "zero tcp port",
			raw:  publicDirect,
			mutate: func(root map[string]any) {
				topologyNode(root, 2)["ingress"].(map[string]any)["address"] = "/ip4/1.1.1.1/tcp/0"
			},
			code: "topology_unsupported_ingress_address",
		},
		{
			name: "ipv4 mapped ipv6 address",
			raw:  publicDirect,
			mutate: func(root map[string]any) {
				topologyNode(root, 2)["ingress"].(map[string]any)["address"] =
					"/ip6/::ffff:101:101/tcp/60000"
			},
			code: "topology_unsupported_ingress_address",
		},
		{
			name: "at least two public ingress nodes",
			raw:  publicDirect,
			mutate: func(root map[string]any) {
				topologyNode(root, 2)["ingress"] = map[string]any{"kind": "outbound_only"}
			},
			code: "topology_insufficient_public_ingress",
		},
		{
			name: "outbound only has no address",
			raw:  publicDirect,
			mutate: func(root map[string]any) {
				topologyNode(root, 0)["ingress"].(map[string]any)["address"] = "/ip4/203.0.113.13/tcp/60000"
			},
			code: "topology_invalid_outbound_only_ingress",
		},
		{
			name: "tcp only rejects wss",
			raw:  publicDirect,
			mutate: func(root map[string]any) {
				root["transport_profile"] = "tcp_only"
			},
			code: "topology_transport_profile_mismatch",
		},
		{
			name: "wss certificate required",
			raw:  publicDirect,
			mutate: func(root map[string]any) {
				ingress := topologyNode(root, 1)["ingress"].(map[string]any)
				delete(ingress, "certificate_ref")
				delete(ingress, "certificate_identity")
			},
			code: "topology_wss_certificate_required",
		},
		{
			name: "wss identity must match advertised dns name",
			raw:  publicDirect,
			mutate: func(root map[string]any) {
				topologyNode(root, 1)["ingress"].(map[string]any)["certificate_identity"] = "node-a.example"
			},
			code: "topology_wss_certificate_identity_mismatch",
		},
		{
			name: "tcp has no certificate reference",
			raw:  publicDirect,
			mutate: func(root map[string]any) {
				topologyNode(root, 2)["ingress"].(map[string]any)["certificate_ref"] = "unused-certificate"
			},
			code: "topology_unexpected_certificate_reference",
		},
		{
			name: "duplicate ingress address",
			raw:  privateLAN,
			mutate: func(root map[string]any) {
				topologyNode(root, 1)["ingress"].(map[string]any)["address"] =
					topologyNode(root, 0)["ingress"].(map[string]any)["address"]
			},
			code: "topology_duplicate_ingress_address",
		},
		{
			name: "unsupported address protocol",
			raw:  privateLAN,
			mutate: func(root map[string]any) {
				topologyNode(root, 0)["ingress"].(map[string]any)["address"] = "/ip4/10.23.0.13/udp/60000"
			},
			code: "topology_unsupported_ingress_address",
		},
		{
			name: "too many dns roots",
			raw:  publicDirect,
			mutate: func(root map[string]any) {
				root["signed_dns_roots"] = []any{
					"enrtree://AKPYQIUQIL7PSIACI32J7FGZW56E5FKHEFCCOFHILBIMW3M6LWXS2@a.example",
					"enrtree://AKPYQIUQIL7PSIACI32J7FGZW56E5FKHEFCCOFHILBIMW3M6LWXS2@b.example",
					"enrtree://AKPYQIUQIL7PSIACI32J7FGZW56E5FKHEFCCOFHILBIMW3M6LWXS2@c.example",
					"enrtree://AKPYQIUQIL7PSIACI32J7FGZW56E5FKHEFCCOFHILBIMW3M6LWXS2@d.example",
					"enrtree://AKPYQIUQIL7PSIACI32J7FGZW56E5FKHEFCCOFHILBIMW3M6LWXS2@e.example",
				}
			},
			code: "topology_too_many_signed_dns_roots",
		},
		{
			name: "invalid dns root",
			raw:  publicDirect,
			mutate: func(root map[string]any) {
				root["signed_dns_roots"] = []any{"https://nodes.example.org"}
			},
			code: "topology_invalid_signed_dns_root",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Compile(mutateTopology(t, tc.raw, tc.mutate))
			require.EqualError(t, err, tc.code)
		})
	}
}

func TestCompileRejectsSecretFieldsAndOversizedReferencesWithoutEchoingValues(t *testing.T) {
	valid := readTopologyFixture(t, "private-lan.json")
	secret := "do-not-echo-this-private-value"
	tests := []struct {
		name   string
		mutate func(map[string]any)
		code   string
	}{
		{
			name: "secret field",
			mutate: func(root map[string]any) {
				root["password"] = secret
			},
			code: "topology_forbidden_secret_field",
		},
		{
			name: "private key in signer alias",
			mutate: func(root map[string]any) {
				root["operator_signer_alias"] = "-----BEGIN PRIVATE KEY-----"
			},
			code: "topology_unsafe_reference",
		},
		{
			name: "oversized host key reference",
			mutate: func(root map[string]any) {
				topologyHost(root, 0)["host_key_pin_ref"] = strings.Repeat("a", 129)
			},
			code: "topology_unsafe_reference",
		},
		{
			name: "oversized slot",
			mutate: func(root map[string]any) {
				topologyNode(root, 0)["slot"] = "node-" + strings.Repeat("a", 32)
			},
			code: "topology_invalid_node_slot",
		},
		{
			name: "oversized image",
			mutate: func(root map[string]any) {
				topologyNode(root, 0)["image"] = strings.Repeat("a", 513)
			},
			code: "topology_immutable_image_required",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Compile(mutateTopology(t, valid, tc.mutate))
			require.EqualError(t, err, tc.code)
			require.NotContains(t, err.Error(), secret)
		})
	}
}

func TestCompileCanonicalizesInputOrderAndRedactsProtectedTopology(t *testing.T) {
	raw := readTopologyFixture(t, "private-lan.json")
	first, err := Compile(raw)
	require.NoError(t, err)

	reordered := mutateTopology(t, raw, func(root map[string]any) {
		nodes := root["nodes"].([]any)
		root["nodes"] = []any{nodes[1], nodes[2], nodes[0]}
		for _, item := range root["nodes"].([]any) {
			node := item.(map[string]any)
			peers := node["static_recovery_peers"].([]any)
			node["static_recovery_peers"] = []any{peers[1], peers[0]}
		}
	})
	second, err := Compile(reordered)
	require.NoError(t, err)
	require.Equal(t, first, second)

	presentation, err := json.Marshal(first)
	require.NoError(t, err)
	for _, protected := range []string{
		"10.23.0.", "p1_", "12D3Koo", "ssh-node-", "host-pin-",
		"node-state-", "authority-state-", "operator-primary",
		"registry.example", "sha256:", "node-a.ardents.net", "1.1.1.1",
	} {
		require.NotContains(t, string(presentation), protected)
	}
}

func TestCompileDoesNotDereferenceManifestAliasesOrNetworkRoots(t *testing.T) {
	raw := readTopologyFixture(t, "public-direct.json")
	isolated := mutateTopology(t, raw, func(root map[string]any) {
		root["operator_signer_alias"] = "nonexistent-signer"
		root["signed_dns_roots"] = []any{
			"enrtree://AKPYQIUQIL7PSIACI32J7FGZW56E5FKHEFCCOFHILBIMW3M6LWXS2@unreachable.invalid",
		}
		root["checkpoint_repository"].(map[string]any)["reference"] = "nonexistent-repository"
		for index := range root["nodes"].([]any) {
			topologyHost(root, index)["ssh_alias"] = "unreachable-host-" + string(rune('a'+index))
			topologyHost(root, index)["host_key_pin_ref"] = "nonexistent-pin-" + string(rune('a'+index))
		}
	})

	_, err := Compile(isolated)
	require.NoError(t, err)
}

func TestCompilerProductionDependencyClosureHasNoSideEffectAdapters(t *testing.T) {
	allowedImports := map[string]struct{}{
		"bytes": {}, "encoding/json": {}, "errors": {}, "io": {},
		"net/netip": {}, "reflect": {}, "regexp": {}, "sort": {}, "strconv": {}, "strings": {},
		"ardents/internal/identity/principal":  {},
		"github.com/distribution/reference":    {},
		"github.com/multiformats/go-multiaddr": {},
		"github.com/multiformats/go-multihash": {},
	}
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		require.NoError(t, err)
		for _, imported := range parsed.Imports {
			name, err := strconv.Unquote(imported.Path.Value)
			require.NoError(t, err)
			_, allowed := allowedImports[name]
			require.Truef(
				t,
				allowed,
				"%s imports %q; production dependency closure must contain no SSH, dial, DNS, PKI, signer, repository, host-mutation, or process adapter",
				path,
				name,
			)
		}
	}
}

func readTopologyFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return raw
}

func mutateTopology(t *testing.T, raw []byte, mutate func(map[string]any)) []byte {
	t.Helper()
	var root map[string]any
	require.NoError(t, json.Unmarshal(raw, &root))
	mutate(root)
	changed, err := json.Marshal(root)
	require.NoError(t, err)
	return changed
}

func topologyNode(root map[string]any, index int) map[string]any {
	return root["nodes"].([]any)[index].(map[string]any)
}

func topologyHost(root map[string]any, index int) map[string]any {
	return topologyNode(root, index)["host"].(map[string]any)
}

func topologyFailureDomain(parent map[string]any, field string) map[string]any {
	return parent[field].(map[string]any)
}
