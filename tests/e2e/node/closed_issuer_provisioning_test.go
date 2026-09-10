package state_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

// This cell proves public provisioning across real commands and acceptance
// against canonical signed Epoch/Node bytes through durable State. It does not
// qualify installed workers or publication. Issuer readiness and abrupt restart
// use the actual command with each accepted Carrier.
func TestClosedIssuerProfileProvisioningAcrossProcesses(t *testing.T) {
	for _, carrier := range []string{"ardents-carrier-tcp-tls-v2", "ardents-carrier-quic-v2"} {
		t.Run(carrier, func(t *testing.T) { testClosedIssuerProvisioning(t, carrier, 3) })
	}
}

func testClosedIssuerProvisioning(t *testing.T, carrier string, nodeCount int) {
	t.Helper()
	network, issuerNode := [32]byte{1}, [32]byte{2}
	authority, exchange := prepareClosedProcessExchange(t, network, issuerNode, time.Now().UTC())
	testClosedIssuerProvisioningParticipant(t, carrier, nodeCount, authority, exchange, nil)
}

func testClosedIssuerProvisioningParticipant(t *testing.T, carrier string, nodeCount int, admissionAuthority [32]byte, exchange func(state.Config, bool), participant func(state.Config, string, string, map[string]any)) {
	nodeBinary, controlBinary := buildCommand(t, "ardents-node"), buildCommand(t, "ardents-control")
	now := time.Now().UTC().Truncate(time.Hour)
	network, issuerNode := [32]byte{1}, [32]byte{2}
	nodePrivate := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{4}, ed25519.SeedSize))
	statePrivate := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{3}, ed25519.SeedSize))
	issuerRoot := filepath.Join(t.TempDir(), "issuer-root")
	initialize := writeJSON(t, "issuer-initialize.json", map[string]any{
		"schema": "ardents-closed-issuer-initialize-v1", "root": issuerRoot,
		"network_id": hex.EncodeToString(network[:]), "node_id": hex.EncodeToString(issuerNode[:]),
		"identity_key": writePrivateKey(t, "issuer.pem", nodePrivate),
		"not_before":   now.Format(time.RFC3339), "not_after": now.Add(2 * time.Hour).Format(time.RFC3339),
	})
	invoke := func(binary string, arguments ...string) []byte {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		output, err := exec.CommandContext(ctx, binary, arguments...).CombinedOutput()
		if err != nil {
			t.Fatalf("public provisioning command %q: %v / %s", arguments[0], err, output)
		}
		return output
	}
	exported := invoke(nodeBinary, "issuer", "initialize", "--config", initialize)
	if second := invoke(nodeBinary, "issuer", "initialize", "--config", initialize); !bytes.Equal(exported, second) {
		t.Fatal("issuer restart changed the public profile export")
	}
	exportPath := filepath.Join(t.TempDir(), "issuer-public.json")
	if err := os.WriteFile(exportPath, exported, 0o600); err != nil {
		t.Fatal(err)
	}
	inventory := invoke(controlBinary, "inspect-closed-issuer-profile", "--profile", exportPath,
		"--network", hex.EncodeToString(network[:]), "--node", hex.EncodeToString(issuerNode[:]),
		"--node-key", hex.EncodeToString(nodePrivate.Public().(ed25519.PublicKey)))
	var inspected map[string]json.RawMessage
	if err := json.Unmarshal(inventory, &inspected); err != nil {
		t.Fatal(err)
	}
	var keys []json.RawMessage
	if err := json.Unmarshal(inspected["TokenKeys"], &keys); err != nil || len(keys) != 6 {
		t.Fatalf("issuer inventory does not supply all six class/window keys: %v", err)
	}
	identifier := func(value byte) string { return hex.EncodeToString(bytes.Repeat([]byte{value}, 32)) }
	config, acceptedState, records, endpointBinary, acceptArguments := closedProvisioningStateSize(t, network, issuerNode, statePrivate, nodePrivate, now, carrier, nodeCount)
	roles := closedTextTopologyRoles(nodeCount)
	nodePlans := make([]map[string]any, len(records))
	for index, record := range records {
		digest := sha256.Sum256(record.Raw)
		nodePlans[index] = map[string]any{"NodeID": hex.EncodeToString(record.NodeID[:]), "RecordDigest": hex.EncodeToString(digest[:]), "RoleDomain": roles[index][0], "Subrole": roles[index][1], "DutyGeneration": index + 1}
	}
	plan := map[string]any{
		"NetworkID": hex.EncodeToString(network[:]), "StateGeneration": acceptedState.Generation, "EpochDigest": hex.EncodeToString(acceptedState.Digest[:]), "Epoch": acceptedState.Epoch,
		"IssuerNodeID": hex.EncodeToString(issuerNode[:]), "IssuanceAuthorityKey": hex.EncodeToString(admissionAuthority[:]),
		"NotBefore": now.Format(time.RFC3339), "NotAfter": now.Add(2 * time.Hour).Format(time.RFC3339), "TokenKeys": keys,
		"Nodes": nodePlans,
	}
	planPath := writeJSON(t, "closed-profile-plan.json", plan)
	preparedPath, signedPath := filepath.Join(t.TempDir(), "unsigned.profile"), filepath.Join(t.TempDir(), "signed.profile")
	invoke(controlBinary, "prepare-closed-profile", "--plan", planPath, "--output", preparedPath)
	invoke(controlBinary, "sign-closed-profile", "--plan", planPath, "--authority-key", writePrivateKey(t, "state.pem", statePrivate), "--output", signedPath)
	unsigned, err := os.ReadFile(preparedPath)
	if err != nil {
		t.Fatal(err)
	}
	signed, err := os.ReadFile(signedPath)
	if err != nil || len(signed) != len(unsigned)+ed25519.SignatureSize || !bytes.Equal(signed[:len(unsigned)], unsigned) {
		t.Fatalf("separate preparation and signing commands disagree: %v", err)
	}
	result := invoke(controlBinary, "inspect-closed-profile", "--plan", planPath, "--profile", signedPath,
		"--authority", hex.EncodeToString(statePrivate.Public().(ed25519.PublicKey)), "--at", now.Format(time.RFC3339))
	var report struct{ Schema string }
	if err := json.Unmarshal(result, &report); err != nil || report.Schema != "ardents-closed-profile-inspection-v1" {
		t.Fatalf("State profile inspection report missing: %v", err)
	}
	// A command-signed profile must still match the exact accepted Node bytes.
	nodes := plan["Nodes"].([]map[string]any)
	originalDigest := nodes[0]["RecordDigest"]
	nodes[0]["RecordDigest"] = identifier(99)
	wrongPlanPath := writeJSON(t, "wrong-node-profile-plan.json", plan)
	nodes[0]["RecordDigest"] = originalDigest
	wrongProfilePath := filepath.Join(t.TempDir(), "wrong-node.profile")
	invoke(controlBinary, "sign-closed-profile", "--plan", wrongPlanPath, "--authority-key", writePrivateKey(t, "wrong-node-state.pem", statePrivate), "--output", wrongProfilePath)
	config.Root = t.TempDir()
	acceptArguments[2] = config.Root
	wrongArguments := append(append([]string(nil), acceptArguments...), "--closed-profile", wrongProfilePath)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	output, commandErr := exec.CommandContext(ctx, endpointBinary, wrongArguments...).CombinedOutput()
	if commandErr == nil || !bytes.Contains(output, []byte("accept closed profile:")) {
		t.Fatalf("command did not reject substituted Node Record digest at profile validation: %v / %s", commandErr, output)
	}
	owner, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	_, rejectedRouteErr := owner.CurrentClosedRoute()
	if closeErr := owner.Close(); closeErr != nil || rejectedRouteErr == nil {
		t.Fatalf("rejected command left an available closed route: %v / %v", rejectedRouteErr, closeErr)
	}
	config.Root = t.TempDir()
	acceptArguments[2] = config.Root
	validArguments := append(append([]string(nil), acceptArguments...), "--closed-profile", signedPath)
	accepted := invoke(endpointBinary, validArguments...)
	var event struct {
		Kind    string
		Profile string `json:"closed_profile_sha256"`
	}
	digest := sha256.Sum256(signed)
	if err := json.Unmarshal(accepted, &event); err != nil || event.Kind != "generation-accepted" || event.Profile != hex.EncodeToString(digest[:]) {
		t.Fatalf("command did not report accepted profile digest: %v / %s", err, accepted)
	}
	owner, err = state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	view, profileErr := owner.CurrentClosedProfile()
	route, routeErr := owner.CurrentClosedRoute()
	closeErr := owner.Close()
	if profileErr != nil || routeErr != nil || closeErr != nil || view.Digest != digest || int(route.NodeCount) != nodeCount {
		t.Fatalf("command profile did not join accepted State: %v / %v / %v", profileErr, routeErr, closeErr)
	}
	for _, node := range route.Nodes[:route.NodeCount] {
		index := int(node.NodeID[0]) - 1
		if index < 0 || index >= len(records) {
			t.Fatal("accepted unexpected topology Node")
		}
		digest := sha256.Sum256(records[index].Raw)
		if node.NodeID != records[index].NodeID || node.RecordDigest != digest || node.RoleDomain != roles[index][0] || node.Subrole != roles[index][1] || node.DutyGeneration != uint64(index+1) {
			t.Fatal("accepted topology changed a signed role binding")
		}
	}
	reopened, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	retained, retainErr := reopened.CurrentClosedRoute()
	closeErr = reopened.Close()
	if retainErr != nil || closeErr != nil || retained != route {
		t.Fatalf("reopened closed State changed: %v / %v", retainErr, closeErr)
	}
	runClosedIssuerProcess(t, nodeBinary, endpointBinary, acceptArguments, signedPath, issuerRoot, network, issuerNode, statePrivate, nodePrivate, now, nodeCount, func(unavailable bool) { exchange(config, unavailable) }, func(resolutionRoot string, sourcePlan map[string]any) {
		if participant != nil {
			participant(config, endpointBinary, resolutionRoot, sourcePlan)
		}
	})
}

func identifierNode(value byte) string {
	node := [32]byte{value}
	return hex.EncodeToString(node[:])
}

// The complete closed text topology is accepted through real State/profile
// commands, then each real Node must report READY and remain alive until all
// sixteen have started. Continuous readiness is not established by this cell.
// This cell does not run an ordinary Endpoint exchange.
func TestClosedTextTopologyProvisioningAcrossProcesses(t *testing.T) {
	for _, carrier := range []string{"ardents-carrier-tcp-tls-v2", "ardents-carrier-quic-v2"} {
		t.Run(carrier, func(t *testing.T) { testClosedIssuerProvisioning(t, carrier, 16) })
	}
}

func closedTextTopologyRoles(count int) [][2]uint8 {
	roles := [][2]uint8{{1, 1}, {2, 6}, {1, 2}}
	if count == 16 {
		roles = append(roles, [][2]uint8{{1, 1}, {1, 2}, {2, 5}, {4, 3}, {4, 1}, {4, 1}, {4, 2}, {4, 2}, {3, 1}, {3, 1}, {3, 2}, {3, 2}, {2, 4}}...)
	}
	return roles
}
