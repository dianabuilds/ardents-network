package state_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/source"
	"github.com/dianabuilds/ardents-network/internal/network/state"
)

// Real Source commands must distribute the same signed closed Epoch that the
// offline command accepted. This does not qualify a Node duty or publication.
func TestClosedSourceProcessesDistributeAcceptedState(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{3}, ed25519.SeedSize))
	nodeKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{4}, ed25519.SeedSize))
	network := [32]byte{1}
	config, expected, _, endpoint, arguments := closedProvisioningState(t, network, [32]byte{2}, authority, nodeKey, now, "ardents-carrier-tcp-tls-v2")
	node := buildCommand(t, "ardents-node")
	clientAuthority := makeAuthority(t, "closed-source-client")
	client := makeLeaf(t, clientAuthority, "closed-source-client.test", false)
	transport := source.Config{ClientCertificate: loadCertificate(t, client), OrderSeed: sha256.Sum256([]byte("closed source process"))}
	for index := range transport.Sources {
		root := t.TempDir()
		args := append([]string(nil), arguments...)
		args[2] = root
		runProvisioningCommand(t, endpoint, args...)
		serverAuthority := makeAuthority(t, fmt.Sprintf("closed-source-%d", index))
		serverName := fmt.Sprintf("closed-source-%d.test", index)
		server := makeLeaf(t, serverAuthority, serverName, true)
		address := freeAddress(t)
		roleRoot := t.TempDir()
		if err := os.Chmod(roleRoot, 0o700); err != nil {
			t.Fatal(err)
		}
		plan := nativeDutySourcePlan(network, authority.Public().(ed25519.PublicKey), now, root, roleRoot, address, server, clientAuthority.root, client.sourcePin)
		delete(plan, "native_rendezvous_profile")
		plan["state_profile"] = "ardents-route-v3"
		plan["state_profile_authority"] = hex.EncodeToString(authority.Public().(ed25519.PublicKey))
		if index == 0 {
			for _, invalid := range []struct {
				field  string
				value  any
				reason string
			}{
				{"state_profile", "unselected", "unsupported or ambiguous"},
				{"native_rendezvous_profile", true, "unsupported or ambiguous"},
				{"state_profile_authority", "", "source State profile authority:"},
				{"state_profile_authority", hex.EncodeToString(nodeKey.Public().(ed25519.PublicKey)), "not pinned by State"},
				{"state_profile", "", "requires an explicit State profile"},
			} {
				original, present := plan[invalid.field]
				plan[invalid.field] = invalid.value
				path := writeJSON(t, "rejected-source.json", plan)
				if present {
					plan[invalid.field] = original
				} else {
					delete(plan, invalid.field)
				}
				ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
				output, err := exec.CommandContext(ctx, node, "source", "--config", path).CombinedOutput()
				cancel()
				if err == nil || !bytes.Contains(output, []byte(invalid.reason)) || bytes.Contains(output, []byte("source-ready")) {
					t.Fatalf("invalid source profile %s=%v: %v / %s", invalid.field, invalid.value, err, output)
				}
			}
		}
		stop := startSource(t, node, writeJSON(t, fmt.Sprintf("closed-source-%d.json", index), plan))
		t.Cleanup(stop)
		pem, err := os.ReadFile(server.root)
		if err != nil {
			t.Fatal(err)
		}
		transport.Sources[index] = source.Source{Address: address, ServerName: serverName,
			Identity: sha256.Sum256([]byte(serverName)), Family: fmt.Sprintf("source-%d", index),
			EndpointHandle: fmt.Sprintf("endpoint-%d", index), RootPEM: pem, LeafKeyDigest: server.sourcePin}
	}
	config.Root, config.Source = t.TempDir(), transport
	config.LocalRoleStateRoot = t.TempDir()
	if err := os.Chmod(config.LocalRoleStateRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	owner, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	actual, refreshErr := owner.Refresh(t.Context())
	closeErr := owner.Close()
	if refreshErr != nil || closeErr != nil || actual.Generation != expected.Generation || actual.Digest != expected.Digest || actual.Epoch != expected.Epoch {
		t.Fatalf("closed Source processes did not supply accepted State: %v / %v; generation=%s want=%s digest=%s",
			refreshErr, closeErr, actual.Generation, expected.Generation, hex.EncodeToString(actual.Digest[:]))
	}
}
