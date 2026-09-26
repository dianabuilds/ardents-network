package state_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/tests/epochfixture/assignment"
)

// Canonical test inputs still pass the production Epoch, Node Record and
// durable State validators. No accepted projection is fabricated here.
func closedProvisioningState(t *testing.T, network, issuer [32]byte, authority, issuerKey ed25519.PrivateKey, now time.Time, carrier string) (state.Config, state.Snapshot, []Record, string, []string) {
	t.Helper()
	return closedProvisioningStateSize(t, network, issuer, authority, issuerKey, now, carrier, 3)
}
func closedProvisioningStateSize(t *testing.T, network, issuer [32]byte, authority, issuerKey ed25519.PrivateKey, now time.Time, carrier string, count int) (state.Config, state.Snapshot, []Record, string, []string) {
	t.Helper()
	records := make([]Record, count)
	addresses := make(map[string]struct{}, count)
	roles := closedTextTopologyRoles(count)
	seed := sha256.Sum256([]byte("closed command provisioning"))
	domains := closedTopologyDomains(roles)
	for index := range records {
		key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{byte(6 + index)}, ed25519.SeedSize))
		if index == 1 {
			key = issuerKey
		}
		node := network
		if index == 1 {
			node = issuer
		}
		if index >= 2 {
			node = [32]byte{byte(index + 1)}
		}
		address := closedProvisioningAddress(t, carrier)
		for {
			if _, exists := addresses[address]; !exists {
				addresses[address] = struct{}{}
				break
			}
			address = closedProvisioningAddress(t, carrier)
		}
		family := closedRoleFamily(t, network, seed, domains, closedRoleDomainName(roles[index][0]), fmt.Sprintf("closed-node-%d", index+1))
		record, err := BuildRecord(RecordSpec{NetworkID: network, NodeID: node, Generation: uint64(index + 1),
			ValidFrom: now, ValidUntil: now.Add(2 * time.Hour), Family: family,
			Endpoint: address, Carrier: carrier,
			Capability: 2, Capacity: 4, PrivateKey: key})
		if err != nil {
			t.Fatal(err)
		}
		records[index] = record
	}
	rawInputs := make([][]byte, len(records))
	for index := range records {
		rawInputs[index] = records[index].Raw
	}
	epoch, err := BuildEpoch(EpochSpec{NetworkID: network, Number: 1, ValidFrom: now, ValidUntil: now.Add(2 * time.Hour),
		Inputs: rawInputs, Accepted: records, AssignmentSeed: seed,
		Profile: "ardents-route-v3", Version: 3, Domains: domains, Authorities: []ed25519.PrivateKey{authority}})
	if err != nil {
		t.Fatal(err)
	}
	public := authority.Public().(ed25519.PublicKey)
	config := state.Config{Root: t.TempDir(), NetworkID: network, Authorities: map[[32]byte]ed25519.PublicKey{sha256.Sum256(public): public},
		Threshold: 1, ClosedProfileAuthority: public, AcceptedProfile: "ardents-route-v3", Now: time.Now().UTC(), ClockObservation: time.Now().UTC()}
	binary := buildCommand(t, "ardents")
	directory := t.TempDir()
	inputs := filepath.Join(directory, "inputs")
	if err := os.Mkdir(inputs, 0o700); err != nil {
		t.Fatal(err)
	}
	epochPath, materialPath := filepath.Join(directory, "epoch.bin"), filepath.Join(directory, "material.bin")
	files := map[string][]byte{epochPath: epoch.Raw, materialPath: epoch.Materials[0]}
	for index := 1; index < len(epoch.Materials); index++ {
		files[fmt.Sprintf("%s.%d", materialPath, index)] = epoch.Materials[index]
	}
	for index, raw := range epoch.Inputs {
		files[filepath.Join(inputs, fmt.Sprintf("%04d.bin", index))] = raw
	}
	for path, raw := range files {
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	arguments := []string{"accept-offline", "--state-root", config.Root, "--network-id", hex.EncodeToString(network[:]),
		"--authorities", hex.EncodeToString(public), "--threshold", "1", "--at", config.Now.Format(time.RFC3339),
		"--epoch", epochPath, "--inputs", inputs, "--materialization", materialPath, "--profile", config.AcceptedProfile,
		"--closed-profile-authority", hex.EncodeToString(public)}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	if output, err := exec.CommandContext(ctx, binary, arguments...).CombinedOutput(); err != nil {
		t.Fatalf("command acceptance of canonical closed State: %v / %s", err, output)
	}
	owner, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, acceptErr := owner.Current()
	closeErr := owner.Close()
	if acceptErr != nil || closeErr != nil {
		t.Fatalf("accept canonical closed State: %v / %v", acceptErr, closeErr)
	}
	return config, snapshot, records, binary, arguments
}

func runProvisioningCommand(t *testing.T, binary string, arguments ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	if output, err := exec.CommandContext(ctx, binary, arguments...).CombinedOutput(); err != nil {
		t.Fatalf("provisioning command %s: %v / %s", arguments[0], err, output)
	}
}

// Windows can exclude a UDP range while allowing TCP binds to the same ports.
// Probe the selected Carrier's socket family before signing its Node Record.
func closedProvisioningAddress(t *testing.T, carrier string) string {
	t.Helper()
	if carrier != "ardents-carrier-quic-v2" {
		return freeAddress(t)
	}
	listener, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.LocalAddr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return address
}

// closedTextTopologyRoles is the semantic closed Route topology: each entry is
// a signed profile Role Domain number and its subrole. It lives in this
// cross-platform file because closedProvisioningStateSize derives the Epoch
// role-name domains and each record's family assignment from it on every
// platform, while only the Linux process cells start the matching Node duties.
func closedTextTopologyRoles(count int) [][2]uint8 {
	roles := [][2]uint8{{1, 1}, {2, 6}, {1, 2}}
	if count == 16 {
		roles = append(roles, [][2]uint8{{1, 1}, {1, 2}, {2, 5}, {4, 3}, {4, 1}, {4, 1}, {4, 2}, {4, 2}, {3, 1}, {3, 1}, {3, 2}, {3, 2}, {2, 4}}...)
	}
	return roles
}

// closedRoleDomainName maps a closed-profile Role Domain number to the Epoch
// assignment text the State join expects: Initiator=1, Rendezvous=2,
// Responder=3, Introduction=4 (docs/technical/protected-route-protocol.md).
func closedRoleDomainName(domain uint8) string {
	switch domain {
	case 1:
		return "initiator"
	case 2:
		return "rendezvous"
	case 3:
		return "responder"
	case 4:
		return "introduction"
	default:
		return ""
	}
}

// closedTopologyDomains returns the distinct role-name domains the topology
// uses, in the strict ascending order decodeSummaries requires.
func closedTopologyDomains(roles [][2]uint8) []string {
	seen := make(map[string]struct{}, 4)
	for _, role := range roles {
		seen[closedRoleDomainName(role[0])] = struct{}{}
	}
	domains := make([]string, 0, len(seen))
	for name := range seen {
		domains = append(domains, name)
	}
	sort.Strings(domains)
	return domains
}

// closedRoleFamily searches a deterministic family name whose Epoch assignment
// under the role-name domains lands on the topology's required domain, so the
// signed profile Role Domain equals the record's authenticated Epoch
// assignment. Family text is free-form, so a real closed operator selects it
// the same way; the search mirrors that selection rather than fabricating an
// assignment.
func closedRoleFamily(t *testing.T, network, seed [32]byte, domains []string, target, base string) string {
	t.Helper()
	for attempt := 0; attempt < 4096; attempt++ {
		family := base
		if attempt > 0 {
			family = fmt.Sprintf("%s-%d", base, attempt)
		}
		selected, err := assignment.Select(network, 1, seed, family, domains)
		if err != nil {
			t.Fatalf("closed role family assignment: %v", err)
		}
		if selected == target {
			return family
		}
	}
	t.Fatalf("no closed role family reached domain %q within the search bound", target)
	return ""
}
