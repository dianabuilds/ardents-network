//go:build referencec2 && (h4_3b_multihost || h4_8_a11)

package service_test

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

type h48A11Topology struct {
	Schema                   string `json:"schema"`
	StateRendezvousEndpoint  string `json:"state_rendezvous_endpoint"`
	ProductRendezvousListen  string `json:"product_rendezvous_listen"`
	CarrierRelayListen       string `json:"carrier_relay_listen"`
	CarrierRelayTarget       string `json:"carrier_relay_target"`
	RendezvousImplementation string `json:"rendezvous_implementation"`
	FixtureRendezvousStarted bool   `json:"fixture_rendezvous_started"`
	AlphaRelayPurpose        string `json:"alpha_relay_purpose"`
}

func h48A11ConfigureProductTransit(fixture, plan map[string]any, advertised, loopback string) (h48A11Topology, error) {
	advertisedHost, advertisedPort, err := net.SplitHostPort(advertised)
	if err != nil {
		return h48A11Topology{}, errors.New("A11 State-advertised Rendezvous endpoint is invalid")
	}
	advertisedIP := net.ParseIP(advertisedHost)
	loopbackHost, loopbackPort, loopbackErr := net.SplitHostPort(loopback)
	loopbackIP := net.ParseIP(loopbackHost)
	port, portErr := strconv.Atoi(advertisedPort)
	if advertisedIP == nil || advertisedIP.IsUnspecified() || advertisedIP.IsLoopback() || loopbackErr != nil || loopbackIP == nil ||
		!loopbackIP.IsLoopback() || advertisedPort != loopbackPort || portErr != nil || port < 1024 || port > 65535 {
		return h48A11Topology{}, errors.New("A11 product Rendezvous operational bind is invalid")
	}
	peer, peerOK := fixture["Rendezvous"].(map[string]string)
	rendezvous, rendezvousOK := plan["rendezvous"].(map[string]any)
	if !peerOK || peer["Endpoint"] != advertised || plan["listen"] != advertised || !rendezvousOK {
		return h48A11Topology{}, errors.New("A11 product Rendezvous diverges from authenticated State")
	}
	rendezvous["listen_loopback_override"] = loopback
	fixture["CarrierRelayListenAddress"] = advertised
	fixture["CarrierRelayTargetAddress"] = loopback
	return h48A11Topology{Schema: "ardents-h4-8-a11-topology-v1", StateRendezvousEndpoint: advertised,
		ProductRendezvousListen: loopback, CarrierRelayListen: advertised, CarrierRelayTarget: loopback,
		RendezvousImplementation: "ardents-node", FixtureRendezvousStarted: false, AlphaRelayPurpose: "private-resolution-only"}, nil
}

func stageH48A11ProductTransit(t *testing.T, root string, fixture map[string]any, state referenceC2StateFixture,
	material referenceC2CertificateMaterial, client referenceC2SourceCredential, fixtureSources []map[string]string, scenario referenceC2Scenario,
) {
	t.Helper()
	if !scenario.productRendezvousRelay {
		if scenario.transitFault != "" {
			t.Fatal("A11 transit fault requires product Rendezvous plus Carrier relay")
		}
		return
	}
	if scenario.transitFault != "" && scenario.transitFault != referenceC2TransitFaultCarrierLoss && scenario.transitFault != referenceC2TransitFaultProductNodeLoss {
		t.Fatal("A11 transit fault is unsupported")
	}
	if scenario.transitFault != "" && scenario.publisherTerminal != "" {
		t.Fatal("A11 transit and Publisher faults are mutually exclusive")
	}
	record := state.roles["rendezvous"]
	advertised := record.endpoint
	_, port, err := net.SplitHostPort(advertised)
	if err != nil {
		t.Fatal(err)
	}
	loopback := net.JoinHostPort("127.0.0.1", port)
	certificatePath, keyPath := filepath.Join(root, "rendezvous-node-cert.pem"), filepath.Join(root, "rendezvous-node-key.pem")
	h43WriteFile(t, certificatePath, []byte(material.certificate), 0o600)
	h43WriteFile(t, keyPath, []byte(material.privateKey), 0o600)
	h43CopyFile(t, filepath.Join(root, "source-client-cert.pem"), client.certificate)
	h43CopyFile(t, filepath.Join(root, "source-client-key.pem"), client.privateKey)
	h43WriteFile(t, filepath.Join(root, "rendezvous-node-clock.observation"), []byte("A11 product Rendezvous clock\n"), 0o600)
	sources := make([]map[string]string, len(fixtureSources))
	for index, source := range fixtureSources {
		address, suffix := source["Address"], string(rune('a'+index))
		identity := sha256.Sum256([]byte("reference-c2-state-source-identity-" + address))
		sources[index] = map[string]string{"address": address, "server_name": source["ServerName"],
			"identity": hex.EncodeToString(identity[:]), "family": "reference-c2-state-source-" + suffix,
			"endpoint_handle": "reference-c2-state-source-" + suffix, "root_ca": "/work/source-" + suffix + "-root.pem",
			"leaf_key_digest": source["LeafKeyDigest"]}
	}
	authority := state.authority.Public().(ed25519.PublicKey)
	pairByteLimit := scenario.dynamicWorkload.transitRelayByteLimit()
	plan := map[string]any{"schema": "ardents-node-plan-v1", "state_root": "/work/rendezvous-state",
		"local_role_state_root": "/work/rendezvous-product-node-role", "network_id": referenceC2Hex(state.network),
		"authority_public": []string{hex.EncodeToString(authority)}, "threshold": 1, "at": state.now.Format(time.RFC3339),
		"listen": advertised, "server_certificate": "/work/rendezvous-node-cert.pem", "server_key": "/work/rendezvous-node-key.pem",
		"client_root": "/work/source-a-root.pem", "client_key_digests": []string{hex.EncodeToString(client.leafDigest[:])},
		"materialization_index": record.materializationIndex, "order_seed": referenceC2Hex(state.digest),
		"source_client_certificate": "/work/source-client-cert.pem", "source_client_key": "/work/source-client-key.pem", "sources": sources,
		"node_id": referenceC2Hex(record.nodeID), "identity_key": "/work/rendezvous-node-key.pem", "clock_observation_file": "/work/rendezvous-node-clock.observation",
		"maximum_duty_ms": 5_000, "drain_timeout_ms": 5_000,
		"rendezvous": map[string]any{"handshake_limit": 2, "waiting_limit": 2, "pair_limit": 1, "pair_byte_limit": pairByteLimit,
			"admission_timeout_ms": 5_000, "drain_timeout_ms": 5_000}}
	topology, err := h48A11ConfigureProductTransit(fixture, plan, advertised, loopback)
	if err != nil {
		t.Fatal(err)
	}
	fixture["CarrierRelayReadyPath"] = "/work/carrier-relay-ready.json"
	fixture["CarrierRelayResetPath"] = "/work/carrier-relay-reset"
	fixture["CarrierRelayResetResultPath"] = "/work/carrier-relay-reset.json"
	if scenario.publisherTerminal == referenceC2PublisherApplicationReset {
		fixture["PublisherApplicationFaultReadyPath"] = "/work/publisher-application-fault-ready"
		fixture["PublisherApplicationFaultReleasePath"] = "/work/publisher-application-fault-release"
	}
	if scenario.transitFault != "" {
		fixture["TransitFault"] = scenario.transitFault
		fixture["TransitFaultReadyPath"] = "/work/transit-fault-ready"
	}
	h43WriteJSON(t, filepath.Join(root, "rendezvous-node-plan.json"), plan)
	h43WriteJSON(t, filepath.Join(root, "topology.json"), topology)
	h43WriteFile(t, filepath.Join(root, "expected-fault"), []byte(string(scenario.transitFault)+"\n"), 0o600)
	h43WriteFile(t, filepath.Join(root, "expected-warmup-cycles"), []byte(fmt.Sprintf("%d\n", scenario.dynamicWorkload.Cycles)), 0o600)
}

func TestH48A11ProductTransitKeepsStatePublicAndBindsOnlyProductRendezvousLoopback(t *testing.T) {
	public, loopback := "203.0.113.10:49100", "127.0.0.1:49100"
	fixture := map[string]any{"Rendezvous": map[string]string{"Endpoint": public}}
	plan := map[string]any{"listen": public, "rendezvous": map[string]any{"pair_limit": 1}}
	topology, err := h48A11ConfigureProductTransit(fixture, plan, public, loopback)
	if err != nil {
		t.Fatal(err)
	}
	if fixture["Rendezvous"].(map[string]string)["Endpoint"] != public || plan["listen"] != public {
		t.Fatal("product transit changed the State-advertised Rendezvous endpoint")
	}
	rendezvous := plan["rendezvous"].(map[string]any)
	if rendezvous["listen_loopback_override"] != loopback {
		t.Fatalf("product Rendezvous loopback override = %v", rendezvous["listen_loopback_override"])
	}
	if fixture["CarrierRelayListenAddress"] != public || fixture["CarrierRelayTargetAddress"] != loopback {
		t.Fatalf("Carrier relay endpoints = %v -> %v", fixture["CarrierRelayListenAddress"], fixture["CarrierRelayTargetAddress"])
	}
	if topology.Schema != "ardents-h4-8-a11-topology-v1" || topology.StateRendezvousEndpoint != public ||
		topology.ProductRendezvousListen != loopback || topology.CarrierRelayListen != public || topology.CarrierRelayTarget != loopback ||
		topology.RendezvousImplementation != "ardents-node" || topology.FixtureRendezvousStarted || topology.AlphaRelayPurpose != "private-resolution-only" {
		t.Fatalf("A11 product topology = %+v", topology)
	}
}

func TestH48A11ProductTransitRejectsEveryAdvertisedOperationalDivergence(t *testing.T) {
	for name, endpoints := range map[string][2]string{
		"hostname":       {"carrier.example:49100", "127.0.0.1:49100"},
		"public-target":  {"203.0.113.10:49100", "203.0.113.11:49100"},
		"different-port": {"203.0.113.10:49100", "127.0.0.1:49101"},
		"unspecified":    {"203.0.113.10:49100", "0.0.0.0:49100"},
	} {
		t.Run(name, func(t *testing.T) {
			fixture := map[string]any{"Rendezvous": map[string]string{"Endpoint": endpoints[0]}}
			plan := map[string]any{"listen": endpoints[0], "rendezvous": map[string]any{}}
			if _, err := h48A11ConfigureProductTransit(fixture, plan, endpoints[0], endpoints[1]); err == nil {
				t.Fatal("invalid A11 advertised/operational topology was accepted")
			}
		})
	}
}

func TestH48A11ProductTransitStagesExactProductAndRelayInputs(t *testing.T) {
	root := t.TempDir()
	_, authority, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	material := referenceC2CertificateMaterial{certificate: "certificate", privateKey: "private-key", public: referenceC2ID(4)}
	state := referenceC2StateFixture{network: referenceC2ID(1), digest: referenceC2ID(2), epoch: 7,
		now: time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC), authority: authority,
		roles: map[string]referenceC2StateRecord{"rendezvous": {role: "rendezvous", nodeID: referenceC2ID(4), material: material,
			endpoint: "203.0.113.10:49100", materializationIndex: 9}}}
	clientCertificate, clientKey := filepath.Join(root, "client-cert-input"), filepath.Join(root, "client-key-input")
	if err := os.WriteFile(clientCertificate, []byte("client-cert"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(clientKey, []byte("client-key"), 0o600); err != nil {
		t.Fatal(err)
	}
	client := referenceC2SourceCredential{certificate: clientCertificate, privateKey: clientKey, leafDigest: referenceC2ID(8)}
	sources := []map[string]string{
		{"Address": "127.0.0.1:49106", "ServerName": "source-a", "LeafKeyDigest": referenceC2Hex(referenceC2ID(10))},
		{"Address": "127.0.0.1:49107", "ServerName": "source-b", "LeafKeyDigest": referenceC2Hex(referenceC2ID(11))},
	}
	fixture := map[string]any{"Rendezvous": map[string]string{"Endpoint": state.roles["rendezvous"].endpoint}}
	scenario := referenceC2Scenario{productRendezvousRelay: true, transitFault: referenceC2TransitFaultCarrierLoss,
		dynamicWorkload: referenceC2DynamicWorkload{Cycles: 60, BytesEachDirection: 4 << 20}}
	stageH48A11ProductTransit(t, root, fixture, state, material, client, sources, scenario)
	var plan map[string]any
	raw, err := os.ReadFile(filepath.Join(root, "rendezvous-node-plan.json"))
	if err != nil || json.Unmarshal(raw, &plan) != nil {
		t.Fatalf("decode staged product Rendezvous plan: %v", err)
	}
	rendezvous := plan["rendezvous"].(map[string]any)
	if plan["listen"] != "203.0.113.10:49100" || rendezvous["listen_loopback_override"] != "127.0.0.1:49100" ||
		uint64(rendezvous["pair_byte_limit"].(float64)) != 16<<20 || fixture["TransitFault"] != referenceC2TransitFaultCarrierLoss ||
		fixture["TransitFaultReadyPath"] != "/work/transit-fault-ready" {
		t.Fatalf("staged A11 product plan/fixture = %+v / %+v", plan, fixture)
	}
	for _, name := range []string{"topology.json", "rendezvous-node-cert.pem", "rendezvous-node-key.pem", "source-client-cert.pem",
		"source-client-key.pem", "rendezvous-node-clock.observation", "expected-fault", "expected-warmup-cycles"} {
		if info, err := os.Stat(filepath.Join(root, name)); err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
			t.Fatalf("staged A11 product input %s = %+v / %v", name, info, err)
		}
	}
}
