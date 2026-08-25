package state_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/tests/epochfixture/assignment"
)

// TestRendezvousNodeProcessPairsOnlyStateAuthorizedLegs proves the maintained
// command composes its current signed State view into the selected native
// Rendezvous duty. The two peer legs are deliberately direct TCP/TLS clients:
// this H4-2A cell owns neither their Endpoint duties nor an H4-3 Route.
func TestRendezvousNodeProcessPairsOnlyStateAuthorizedLegs(t *testing.T) {
	endpoint := freeAddress(t)
	fixture := newRendezvousStateFixture(t, endpoint)
	ardents := buildCommand(t, "ardents")
	nodeBinary := buildCommand(t, "ardents-node")
	stateRoot, sourceRoot := t.TempDir(), t.TempDir()
	if err := os.Chmod(sourceRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	acceptRendezvousEpoch(t, ardents, stateRoot, fixture, fixture.rendezvousIndex)

	var sourceServers [2]processCert
	for index := range sourceServers {
		sourceCA := makeAuthority(t, fmt.Sprintf("rendezvous-source-%d-root", index))
		sourceServers[index] = makeLeaf(t, sourceCA, fmt.Sprintf("rendezvous-source-%d.test", index), true)
	}
	clientCA := makeAuthority(t, "rendezvous-source-client-root")
	sourceClient := makeLeaf(t, clientCA, "rendezvous-source-client.test", false)
	var sourceAddresses [2]string
	var stopSources [2]func()
	for index := range sourceServers {
		sourceAddresses[index] = freeAddress(t)
		sourceState, sourceRoles := t.TempDir(), t.TempDir()
		if err := os.Chmod(sourceRoles, 0o700); err != nil {
			t.Fatal(err)
		}
		acceptRendezvousEpoch(t, ardents, sourceState, fixture, 0)
		plan := writeJSON(t, fmt.Sprintf("rendezvous-source-%d.json", index), nativeRendezvousSourcePlan(fixture, sourceState, sourceRoles, sourceAddresses[index], sourceServers[index], clientCA.root, sourceClient.sourcePin))
		stopSources[index] = startSource(t, nodeBinary, plan)
		defer stopSources[index]()
	}
	observation := filepath.Join(t.TempDir(), "clock.observation")
	stopObserver := startClockObserver(t, observation)
	defer stopObserver()
	plan := writeJSON(t, "rendezvous-node.json", rendezvousNodePlan(t, fixture, stateRoot, sourceRoot, sourceAddresses, sourceServers, sourceClient, observation))
	process := startNode(t, nodeBinary, plan)
	t.Cleanup(func() { stopProcess(process) })
	ready := waitNodeState(t, process, "READY", 5*time.Second)
	if ready.Epoch != fixture.epoch.number || ready.Assignment != "rendezvous" || ready.AssignmentDigest != fixture.rendezvousAssignment {
		t.Fatalf("Rendezvous READY event = %+v", ready)
	}

	attachment := [32]byte{0x91}
	initiator, err := openRendezvousProcessLeg(t.Context(), endpoint, fixture.initiator.certificate, fixture.rendezvous.public,
		fixture.leg(attachment, route.InitiatorRole))
	if err != nil {
		t.Fatal(err)
	}
	defer initiator.Close()
	responder, err := openRendezvousProcessLeg(t.Context(), endpoint, fixture.responder.certificate, fixture.rendezvous.public,
		fixture.leg(attachment, route.ResponderRole))
	if err != nil {
		t.Fatal(err)
	}
	defer responder.Close()
	if _, err := initiator.Write([]byte("State-authorized Rendezvous process")); err != nil {
		t.Fatal(err)
	}
	if received := readProcessExact(t, responder, len("State-authorized Rendezvous process")); string(received) != "State-authorized Rendezvous process" {
		t.Fatalf("Rendezvous process carriage = %q", received)
	}

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := submitRejectedRendezvousProcessLeg(ctx, endpoint, fixture.initiator.certificate, fixture.rendezvous.public,
		fixture.leg([32]byte{0x92}, route.ResponderRole)); err == nil {
		t.Fatal("Rendezvous process accepted a State-unauthorized LegBinding identity")
	}
}

type rendezvousStateFixture struct {
	now                  time.Time
	network              [32]byte
	authorityPublic      ed25519.PublicKey
	authorityPrivate     ed25519.PrivateKey
	rendezvous           rendezvousStateRecord
	initiator, responder rendezvousStateRecord
	epoch                lifecycleEpoch
	rendezvousIndex      uint32
	rendezvousAssignment [32]byte
}

type rendezvousStateRecord struct {
	nodeID  [32]byte
	private ed25519.PrivateKey
	public  [32]byte
	raw     []byte
	family  string

	certificate     tls.Certificate
	certificatePath string
	credentials     processCert
}

func newRendezvousStateFixture(t *testing.T, endpoint string) rendezvousStateFixture {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	network := sha256.Sum256([]byte("ardents-h4-rendezvous-process-network"))
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x71}, ed25519.SeedSize))
	certificateAuthority := makeAuthority(t, "rendezvous-node-root")
	fixture := rendezvousStateFixture{now: now, network: network, authorityPublic: authority.Public().(ed25519.PublicKey), authorityPrivate: authority}
	fixture.rendezvous = makeRendezvousStateRecord(t, network, 0x41, "rendezvous-family", endpoint, makeLeaf(t, certificateAuthority, "rendezvous.test", true), now)
	fixture.initiator = makeRendezvousStateRecord(t, network, 0x42, "initiator-family", freeAddress(t), makeLeaf(t, certificateAuthority, "initiator.test", false), now)
	fixture.responder = makeRendezvousStateRecord(t, network, 0x43, "responder-family", freeAddress(t), makeLeaf(t, certificateAuthority, "responder.test", false), now)
	records := []rendezvousStateRecord{fixture.rendezvous, fixture.initiator, fixture.responder}
	sort.Slice(records, func(first, second int) bool {
		return bytes.Compare(records[first].nodeID[:], records[second].nodeID[:]) < 0
	})
	for index, record := range records {
		if record.nodeID == fixture.rendezvous.nodeID {
			fixture.rendezvousIndex = uint32(index)
			break
		}
	}
	domains := []string{"initiator", "rendezvous", "responder"}
	var seed [32]byte
	for marker := uint64(1); ; marker++ {
		seed = sha256.Sum256([]byte(fmt.Sprintf("rendezvous-process-%d", marker)))
		if rendezvousAssignments(network, 1, seed, fixture.rendezvous.family, fixture.initiator.family, fixture.responder.family) {
			break
		}
	}
	inputs, accepted := make([][]byte, len(records)), make([]Record, len(records))
	for index, record := range records {
		inputs[index] = record.raw
		accepted[index] = Record{Raw: record.raw, NodeID: record.nodeID, Family: record.family, Capacity: 4}
	}
	built, err := BuildEpoch(EpochSpec{NetworkID: network, Number: 1, ValidFrom: now.Add(-time.Minute), ValidUntil: now.Add(10 * time.Minute),
		Inputs: inputs, Accepted: accepted, AssignmentSeed: seed, Profile: route.Profile, Domains: domains, Authorities: []ed25519.PrivateKey{authority}})
	if err != nil {
		t.Fatal(err)
	}
	fixture.epoch = lifecycleEpoch{number: built.Number, seed: built.Seed, raw: built.Raw, digest: built.Digest, inputs: built.Inputs, materials: built.Materials}
	fixture.rendezvousAssignment = assignment.Digest(network, 1, seed, fixture.rendezvous.family, "rendezvous")
	return fixture
}

func makeRendezvousStateRecord(t *testing.T, network [32]byte, marker byte, family, endpoint string, certificate processCert, now time.Time) rendezvousStateRecord {
	t.Helper()
	public := certificate.private.Public().(ed25519.PublicKey)
	var publicFixed [32]byte
	copy(publicFixed[:], public)
	nodeID := sha256.Sum256([]byte{0x92, marker})
	record, err := BuildRecord(RecordSpec{NetworkID: network, NodeID: nodeID, Generation: 1, ValidFrom: now.Add(-time.Minute), ValidUntil: now.Add(10 * time.Minute),
		Family: family, Endpoint: endpoint, Capability: 2, Capacity: 4, PrivateKey: certificate.private})
	if err != nil {
		t.Fatal(err)
	}
	loaded := loadCertificate(t, certificate)
	loaded.Certificate = loaded.Certificate[:1]
	rawCertificate, err := os.ReadFile(certificate.certificate)
	if err != nil {
		t.Fatal(err)
	}
	leaf, _ := pem.Decode(rawCertificate)
	if leaf == nil {
		t.Fatal("Node certificate has no leaf")
	}
	return rendezvousStateRecord{nodeID: nodeID, private: certificate.private, public: publicFixed, raw: record.Raw, family: family,
		certificate: loaded, certificatePath: writePEM(t, "native-"+family+".pem", "CERTIFICATE", leaf.Bytes), credentials: certificate}
}

func rendezvousAssignments(network [32]byte, epoch uint64, seed [32]byte, rendezvous, initiator, responder string) bool {
	selectedRendezvous, _ := assignment.Select(network, epoch, seed, rendezvous, []string{"initiator", "rendezvous", "responder"})
	selectedInitiator, _ := assignment.Select(network, epoch, seed, initiator, []string{"initiator", "rendezvous", "responder"})
	selectedResponder, _ := assignment.Select(network, epoch, seed, responder, []string{"initiator", "rendezvous", "responder"})
	return selectedRendezvous == "rendezvous" && selectedInitiator == "initiator" && selectedResponder == "responder"
}

func acceptRendezvousEpoch(t *testing.T, binary, root string, fixture rendezvousStateFixture, materializationIndex uint32) {
	t.Helper()
	directory, inputs := t.TempDir(), ""
	inputs = filepath.Join(directory, "inputs")
	if err := os.Mkdir(inputs, 0o700); err != nil {
		t.Fatal(err)
	}
	epochPath, material := filepath.Join(directory, "epoch.bin"), filepath.Join(directory, "material.bin")
	if err := os.WriteFile(epochPath, fixture.epoch.raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(material, fixture.epoch.materials[materializationIndex], 0o600); err != nil {
		t.Fatal(err)
	}
	for index, raw := range fixture.epoch.inputs {
		if err := os.WriteFile(filepath.Join(inputs, fmt.Sprintf("%04d.bin", index)), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	arguments := []string{"accept-offline", "--state-root", root, "--network-id", hex.EncodeToString(fixture.network[:]), "--authorities", hex.EncodeToString(fixture.authorityPublic),
		"--threshold", "1", "--at", fixture.now.Format(time.RFC3339), "--epoch", epochPath, "--inputs", inputs, "--materialization", material, "--profile", route.Profile}
	if output, err := exec.Command(binary, arguments...).CombinedOutput(); err != nil {
		t.Fatalf("accept native State Epoch: %v\n%s", err, output)
	}
}

func nativeRendezvousSourcePlan(fixture rendezvousStateFixture, root, roles, address string, server processCert, clientRoot string, clientPin [32]byte) map[string]any {
	return map[string]any{"schema": "ardents-source-server-v1", "state_root": root, "local_role_state_root": roles,
		"network_id": hex.EncodeToString(fixture.network[:]), "authority_public": []string{hex.EncodeToString(fixture.authorityPublic)}, "threshold": 1,
		"at": fixture.now.Format(time.RFC3339), "listen": address, "server_certificate": server.certificate, "server_key": server.key,
		"client_root": clientRoot, "client_key_digests": []string{hex.EncodeToString(clientPin[:])}, "materialization_index": 0,
		"native_rendezvous_profile": true}
}

func rendezvousNodePlan(t *testing.T, fixture rendezvousStateFixture, root, sourceRoot string, sourceAddresses [2]string, sourceServers [2]processCert, sourceClient processCert, observation string) map[string]any {
	t.Helper()
	identity := writePrivateKey(t, "rendezvous-identity.pem", fixture.rendezvous.private)
	sources := make([]map[string]any, 2)
	for index := range sources {
		sourceServer := sourceServers[index]
		identityDigest := sha256.Sum256([]byte(fmt.Sprintf("rendezvous-source-%d", index)))
		sources[index] = map[string]any{"address": sourceAddresses[index], "server_name": fmt.Sprintf("rendezvous-source-%d.test", index), "identity": hex.EncodeToString(identityDigest[:]),
			"family": fmt.Sprintf("rendezvous-source-family-%d", index), "endpoint_handle": fmt.Sprintf("rendezvous-source-%d", index), "root_ca": sourceServer.root,
			"leaf_key_digest": hex.EncodeToString(sourceServer.sourcePin[:])}
	}
	return map[string]any{"schema": "ardents-node-plan-v1", "state_root": root, "local_role_state_root": sourceRoot, "network_id": hex.EncodeToString(fixture.network[:]),
		"authority_public": []string{hex.EncodeToString(fixture.authorityPublic)}, "threshold": 1, "at": fixture.now.Format(time.RFC3339), "listen": "127.0.0.1:1",
		"server_certificate": fixture.rendezvous.certificatePath, "server_key": fixture.rendezvous.credentials.key,
		"client_root": sourceServers[0].root, "client_key_digests": []string{hex.EncodeToString(sourceClient.sourcePin[:])},
		"materialization_index": fixture.rendezvousIndex, "clock_observation_file": observation, "order_seed": strings.Repeat("39", 32), "source_client_certificate": sourceClient.certificate,
		"source_client_key": sourceClient.key, "sources": sources, "node_id": hex.EncodeToString(fixture.rendezvous.nodeID[:]), "identity_key": identity,
		"maximum_duty_ms": 1000, "drain_timeout_ms": 1000, "rendezvous": map[string]any{"handshake_limit": 2, "waiting_limit": 2, "pair_limit": 1, "pair_byte_limit": 1 << 20, "drain_timeout_ms": 1000}}
}

func (fixture rendezvousStateFixture) leg(attachment [32]byte, role byte) route.LegBinding {
	sender := fixture.initiator.nodeID
	if role == route.ResponderRole {
		sender = fixture.responder.nodeID
	}
	return route.LegBinding{NetworkID: fixture.network, Epoch: fixture.epoch.number, Digest: fixture.epoch.digest, AttachmentID: attachment,
		SenderRole: role, PeerRole: route.RendezvousRole, SenderNodeID: sender, PeerNodeID: fixture.rendezvous.nodeID, NotAfter: fixture.now.Add(5 * time.Minute)}
}

func openRendezvousProcessLeg(ctx context.Context, endpoint string, certificate tls.Certificate, server [32]byte, binding route.LegBinding) (*tls.Conn, error) {
	raw, err := (&net.Dialer{}).DialContext(ctx, "tcp", endpoint)
	if err != nil {
		return nil, err
	}
	connection := tls.Client(raw, &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, Certificates: []tls.Certificate{certificate},
		InsecureSkipVerify: true, SessionTicketsDisabled: true, NextProtos: []string{route.Profile}, VerifyConnection: func(state tls.ConnectionState) error {
			if len(state.PeerCertificates) != 1 {
				return fmt.Errorf("Rendezvous server certificate is missing")
			}
			public, ok := state.PeerCertificates[0].PublicKey.(ed25519.PublicKey)
			if !ok || !bytes.Equal(public, server[:]) {
				return fmt.Errorf("Rendezvous server identity differs from State")
			}
			return nil
		}})
	if err := connection.SetDeadline(binding.NotAfter); err != nil {
		_ = raw.Close()
		return nil, err
	}
	if err := connection.HandshakeContext(ctx); err != nil {
		_ = raw.Close()
		return nil, err
	}
	if err := route.WriteNodeLegBinding(connection, binding); err != nil {
		_ = raw.Close()
		return nil, err
	}
	peer, err := route.ReadNodeLegBinding(connection)
	if err != nil || binding.VerifyReciprocal(peer) != nil {
		_ = raw.Close()
		return nil, fmt.Errorf("Rendezvous reciprocal LegBinding is invalid: read=%v verify=%v", err, binding.VerifyReciprocal(peer))
	}
	if err := connection.SetDeadline(time.Time{}); err != nil {
		_ = raw.Close()
		return nil, err
	}
	return connection, nil
}

func submitRejectedRendezvousProcessLeg(ctx context.Context, endpoint string, certificate tls.Certificate, server [32]byte, binding route.LegBinding) error {
	connection, err := openRendezvousProcessLeg(ctx, endpoint, certificate, server, binding)
	if connection != nil {
		_ = connection.Close()
	}
	return err
}

func readProcessExact(t *testing.T, reader io.Reader, length int) []byte {
	t.Helper()
	value := make([]byte, length)
	if _, err := io.ReadFull(reader, value); err != nil {
		t.Fatal(err)
	}
	return value
}
