//go:build live

package network_test

import (
	"encoding/hex"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/localroles"
)

func newBlockedNegativeFixture(t *testing.T, clientBinary, serverBinary string) blockedEntryFixture {
	t.Helper()
	fixture := newBlockedEntryFixture(t, clientBinary, serverBinary)
	root := fixture.root
	for _, role := range []string{"negative-endpoint", "recovery-endpoint", "fault-zero", "fault-one"} {
		mustMkdir(t, filepath.Join(root, "input", role))
		mustMkdirShared(t, filepath.Join(root, "sync", role))
	}
	endpoint := filepath.Join(root, "input", "negative-endpoint")
	for _, name := range []string{"route.json", "transition.bin", "cert.pem", "key.pem", "time-confidence"} {
		copyFile(t, filepath.Join(root, "input", "endpoint", name), filepath.Join(endpoint, name))
	}
	copyTree(t, filepath.Join(root, "input", "endpoint", "route-state"), filepath.Join(endpoint, "route-state"))
	now := time.Now().UTC()
	network := prepareBlockedBridgeNetwork(t, filepath.Join(root, "negative-bridge-network-source"), now)
	copyTree(t, network.root, filepath.Join(endpoint, "bridge-network"))
	roles, err := localroles.Open(localroles.Config{Root: filepath.Join(endpoint, "local-roles"), Clock: time.Now, Create: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := roles.Close(); err != nil {
		t.Fatal(err)
	}
	certificate, key, pin := writeBlockedFrontIdentity(t, now)
	envelopes := [][]byte{blockedEnvelope([4]byte{203, 0, 113, 20}, 8481, pin),
		blockedEnvelope([4]byte{203, 0, 113, 21}, 8482, pin)}
	for slot, envelope := range envelopes {
		inviteName := "invite-" + string(rune('0'+slot)) + ".bin"
		planName := "import-" + string(rune('0'+slot))
		writeLiveFile(t, filepath.Join(endpoint, inviteName), blockedInviteForSlot(t, network, envelope, now, byte(slot)))
		plan := blockedImportPlan(network, "negative")
		plan["invite_file"] = "/run/secure/" + inviteName
		writeLivePlan(t, endpoint, planName, plan)
	}
	entryPlan := map[string]any{"schema": "ardents-h3-bridge-entry-plan-v1",
		"bridge_state_root": "/run/state/bridge", "network_state_root": "/run/state/bridge-network",
		"network_id": liveHex(network.snapshot.NetworkID), "network_authorities": []string{hex.EncodeToString(network.authorityPublic)},
		"network_threshold": 1, "network_profile": "h3-role-probe-v1", "route_profile": "h3-route-tracer-v1",
		"local_role_state_root": "/run/state/local-roles", "binary": "/candidate/webtunnel-client",
		"candidate_state_root": "/run/state/candidate", "route_manifest_digest": liveHex(fixture.manifest),
		"transition_handle": "3", "time_confidence_file": "/run/secure/time-confidence"}
	writeLivePlan(t, endpoint, "entry", entryPlan)
	recovery := filepath.Join(root, "input", "recovery-endpoint")
	for _, name := range []string{"route.json", "transition.bin", "cert.pem", "key.pem", "time-confidence"} {
		copyFile(t, filepath.Join(endpoint, name), filepath.Join(recovery, name))
	}
	copyTree(t, filepath.Join(endpoint, "route-state"), filepath.Join(recovery, "route-state"))
	copyTree(t, network.root, filepath.Join(recovery, "bridge-network"))
	recoveryRoles, err := localroles.Open(localroles.Config{Root: filepath.Join(recovery, "local-roles"), Clock: time.Now, Create: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := recoveryRoles.Close(); err != nil {
		t.Fatal(err)
	}
	writeLiveFile(t, filepath.Join(recovery, "invite.bin"), blockedInviteForSlot(t, network, envelopes[1], now, 0))
	recoveryImport := blockedImportPlan(network, "recovery")
	recoveryImport["invite_file"] = "/run/secure/invite.bin"
	writeLivePlan(t, recovery, "import", recoveryImport)
	writeLivePlan(t, recovery, "entry", entryPlan)
	fault := filepath.Join(root, "input", "fault-one")
	writeLiveFile(t, filepath.Join(fault, "front-cert.pem"), certificate)
	writeLiveFile(t, filepath.Join(fault, "front-key.pem"), key)
	writeLivePlan(t, fault, "fault", map[string]any{"envelope": hex.EncodeToString(envelopes[1]),
		"identity": liveHex(network.snapshot.Candidates[0].NodeID)})
	return fixture
}
