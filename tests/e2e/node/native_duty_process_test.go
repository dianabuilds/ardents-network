package state_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/tests/epochfixture/assignment"
)

// TestNativeDutyProcessesUseTheirExactStateAssignments proves that the product
// command, rather than the C-2 fixture's direct node.Run call, can materialize
// every selected native duty from independently accepted State and its required
// two authenticated Source inputs. It is a readiness boundary only: a later
// H4-2 process test must carry one route through these product processes.
func TestNativeDutyProcessesUseTheirExactStateAssignments(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	network := sha256.Sum256([]byte("ardents-h4-native-duty-process-network"))
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x61}, ed25519.SeedSize))
	roleAuthority := makeAuthority(t, "native-duty-role-root")
	roles := []string{"initiator", "introduction", "rendezvous", "responder"}
	records := make([]rendezvousStateRecord, 0, len(roles))
	for index, role := range roles {
		certificate := makeLeaf(t, roleAuthority, "native-duty-"+role+".test", true)
		records = append(records, makeRendezvousStateRecord(t, network, byte(0x51+index), "native-duty-"+role+"-family",
			freeAddress(t), certificate, now))
	}
	sort.Slice(records, func(first, second int) bool {
		return bytes.Compare(records[first].nodeID[:], records[second].nodeID[:]) < 0
	})
	for seedCounter := uint64(1); ; seedCounter++ {
		seed := sha256.Sum256([]byte(fmt.Sprintf("native-duty-process-%d", seedCounter)))
		if nativeDutyAssignments(network, seed, records, roles) {
			inputs, accepted := make([][]byte, len(records)), make([]Record, len(records))
			for index, record := range records {
				inputs[index] = record.raw
				accepted[index] = Record{Raw: record.raw, NodeID: record.nodeID, Family: record.family, Capacity: 4}
			}
			epoch, err := BuildEpoch(EpochSpec{NetworkID: network, Number: 1, ValidFrom: now.Add(-time.Minute), ValidUntil: now.Add(10 * time.Minute),
				Inputs: inputs, Accepted: accepted, AssignmentSeed: seed, Profile: route.Profile, Domains: roles, Authorities: []ed25519.PrivateKey{authority}})
			if err != nil {
				t.Fatal(err)
			}
			runNativeDutyProcesses(t, network, authority, now, roles, records, epoch, seed)
			return
		}
	}
}

func nativeDutyAssignments(network [32]byte, seed [32]byte, records []rendezvousStateRecord, roles []string) bool {
	for _, role := range roles {
		found := false
		for _, record := range records {
			if record.family != "native-duty-"+role+"-family" {
				continue
			}
			selected, err := assignment.Select(network, 1, seed, record.family, roles)
			if err != nil || selected != role {
				return false
			}
			found = true
		}
		if !found {
			return false
		}
	}
	return true
}

func runNativeDutyProcesses(t *testing.T, network [32]byte, authority ed25519.PrivateKey, now time.Time, roles []string,
	records []rendezvousStateRecord, epoch Epoch, seed [32]byte,
) {
	t.Helper()
	ardents, nodeBinary := buildCommand(t, "ardents"), buildCommand(t, "ardents-node")
	public := authority.Public().(ed25519.PublicKey)
	sourceClientAuthority := makeAuthority(t, "native-duty-source-client-root")
	sourceClient := makeLeaf(t, sourceClientAuthority, "native-duty-source-client.test", false)
	var sourceServers [2]processCert
	var sourceAddresses [2]string
	for index := range sourceServers {
		sourceAuthority := makeAuthority(t, fmt.Sprintf("native-duty-source-%d-root", index))
		sourceServers[index] = makeLeaf(t, sourceAuthority, fmt.Sprintf("native-duty-source-%d.test", index), true)
		sourceAddresses[index] = freeAddress(t)
		stateRoot, roleRoot := t.TempDir(), t.TempDir()
		if err := os.Chmod(roleRoot, 0o700); err != nil {
			t.Fatal(err)
		}
		acceptNativeDutyEpoch(t, ardents, stateRoot, network, public, now, epoch, 0)
		plan := writeJSON(t, fmt.Sprintf("native-duty-source-%d.json", index), nativeDutySourcePlan(network, public, now, stateRoot,
			roleRoot, sourceAddresses[index], sourceServers[index], sourceClientAuthority.root, sourceClient.sourcePin))
		stop := startSource(t, nodeBinary, plan)
		t.Cleanup(stop)
	}
	for _, role := range roles {
		var record rendezvousStateRecord
		materialIndex := -1
		for index, candidate := range records {
			if candidate.family == "native-duty-"+role+"-family" {
				record, materialIndex = candidate, index
				break
			}
		}
		if materialIndex < 0 {
			t.Fatalf("native duty %q record is unavailable", role)
		}
		stateRoot, localRoleRoot := t.TempDir(), t.TempDir()
		clockObservation := filepath.Join(t.TempDir(), "native-duty-"+role+".clock")
		stopClock := startClockObserver(t, clockObservation)
		t.Cleanup(stopClock)
		if err := os.Chmod(localRoleRoot, 0o700); err != nil {
			t.Fatal(err)
		}
		acceptNativeDutyEpoch(t, ardents, stateRoot, network, public, now, epoch, uint32(materialIndex))
		plan := writeJSON(t, "native-duty-node-"+role+".json", nativeDutyNodePlan(t, network, public, now, epoch, stateRoot,
			localRoleRoot, clockObservation, sourceAddresses, sourceServers, sourceClient, record, uint32(materialIndex), role))
		process := startNode(t, nodeBinary, plan)
		t.Cleanup(func() { stopProcess(process) })
		ready := waitNodeState(t, process, "READY", 5*time.Second)
		if ready.Epoch != epoch.Number || ready.Assignment != role {
			t.Fatalf("native duty %q READY event = %+v", role, ready)
		}
		want := assignment.Digest(network, epoch.Number, epoch.Seed, record.family, role)
		if ready.AssignmentDigest != want {
			t.Fatalf("native duty %q assignment digest = %x, want %x", role, ready.AssignmentDigest, want)
		}
	}
}

func acceptNativeDutyEpoch(t *testing.T, binary string, root string, network [32]byte, authority ed25519.PublicKey,
	now time.Time, epoch Epoch, materializationIndex uint32,
) {
	t.Helper()
	directory, inputs := t.TempDir(), ""
	inputs = filepath.Join(directory, "inputs")
	if err := os.Mkdir(inputs, 0o700); err != nil {
		t.Fatal(err)
	}
	epochPath, materialPath := filepath.Join(directory, "epoch.bin"), filepath.Join(directory, "material.bin")
	if err := os.WriteFile(epochPath, epoch.Raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(materialPath, epoch.Materials[materializationIndex], 0o600); err != nil {
		t.Fatal(err)
	}
	for index, raw := range epoch.Inputs {
		if err := os.WriteFile(filepath.Join(inputs, fmt.Sprintf("%04d.bin", index)), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	arguments := []string{"accept-offline", "--state-root", root, "--network-id", hex.EncodeToString(network[:]), "--authorities", hex.EncodeToString(authority),
		"--threshold", "1", "--at", now.Format(time.RFC3339), "--epoch", epochPath, "--inputs", inputs, "--materialization", materialPath,
		"--profile", route.Profile}
	if output, err := exec.Command(binary, arguments...).CombinedOutput(); err != nil {
		t.Fatalf("accept native duty State Epoch: %v\n%s", err, output)
	}
}

func nativeDutySourcePlan(network [32]byte, authority ed25519.PublicKey, now time.Time, stateRoot, roleRoot, address string,
	server processCert, clientRoot string, clientPin [32]byte,
) map[string]any {
	return map[string]any{"schema": "ardents-source-server-v1", "state_root": stateRoot, "local_role_state_root": roleRoot,
		"network_id": hex.EncodeToString(network[:]), "authority_public": []string{hex.EncodeToString(authority)}, "threshold": 1,
		"at": now.Format(time.RFC3339), "listen": address, "server_certificate": server.certificate, "server_key": server.key,
		"client_root": clientRoot, "client_key_digests": []string{hex.EncodeToString(clientPin[:])}, "materialization_index": 0,
		"native_rendezvous_profile": true}
}

func nativeDutyNodePlan(t *testing.T, network [32]byte, authority ed25519.PublicKey, now time.Time, epoch Epoch, stateRoot, localRoleRoot, clockObservation string,
	sourceAddresses [2]string, sourceServers [2]processCert, sourceClient processCert, record rendezvousStateRecord, materializationIndex uint32, role string,
) map[string]any {
	t.Helper()
	sources := make([]map[string]any, len(sourceAddresses))
	for index, address := range sourceAddresses {
		identity := sha256.Sum256([]byte(fmt.Sprintf("native-duty-source-%d", index)))
		sources[index] = map[string]any{"address": address, "server_name": fmt.Sprintf("native-duty-source-%d.test", index),
			"identity": hex.EncodeToString(identity[:]), "family": fmt.Sprintf("native-duty-source-family-%d", index),
			"endpoint_handle": fmt.Sprintf("native-duty-source-%d", index), "root_ca": sourceServers[index].root,
			"leaf_key_digest": hex.EncodeToString(sourceServers[index].sourcePin[:])}
	}
	plan := map[string]any{"schema": "ardents-node-plan-v1", "state_root": stateRoot, "local_role_state_root": localRoleRoot,
		"network_id": hex.EncodeToString(network[:]), "authority_public": []string{hex.EncodeToString(authority)}, "threshold": 1,
		"at": now.Format(time.RFC3339), "listen": "127.0.0.1:1", "server_certificate": record.certificatePath,
		"server_key": record.credentials.key, "client_root": sourceServers[0].root, "client_key_digests": []string{hex.EncodeToString(sourceClient.sourcePin[:])},
		"materialization_index": materializationIndex, "order_seed": hex.EncodeToString(epoch.Digest[:]),
		"source_client_certificate": sourceClient.certificate, "source_client_key": sourceClient.key, "sources": sources,
		"node_id": hex.EncodeToString(record.nodeID[:]), "identity_key": writePrivateKey(t, "native-duty-"+role+"-identity.pem", record.private),
		"clock_observation_file": clockObservation, "maximum_duty_ms": 1000, "drain_timeout_ms": 1000}
	switch role {
	case "rendezvous":
		plan["rendezvous"] = map[string]any{"handshake_limit": 2, "waiting_limit": 2, "pair_limit": 1, "pair_byte_limit": 1 << 20, "admission_timeout_ms": 1000, "drain_timeout_ms": 1000}
	case "initiator":
		plan["initiator"] = map[string]any{"handshake_limit": 2, "relay_limit": 1, "relay_byte_limit": 1 << 20, "admission_timeout_ms": 1000, "drain_timeout_ms": 1000}
	case "introduction":
		plan["introduction"] = map[string]any{"handshake_limit": 2, "slot_limit": 1, "delivery_limit": 1, "admission_timeout_ms": 1000, "drain_timeout_ms": 1000}
	case "responder":
		plan["responder"] = map[string]any{"handshake_limit": 2, "relay_limit": 1, "relay_byte_limit": 1 << 20, "admission_timeout_ms": 1000, "drain_timeout_ms": 1000}
	default:
		t.Fatalf("unsupported native duty %q", role)
	}
	return plan
}
