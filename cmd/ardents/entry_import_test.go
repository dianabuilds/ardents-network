package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	localroles "github.com/dianabuilds/ardents-network/internal/network/duty"
)

func TestImportCommandUsesAuthenticatedNetworkState(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)
	network := prepareCommandNetwork(t, directory, now, "ardents-interactive-route-v2")
	invite := commandInvite(network, now)
	invitePath := filepath.Join(directory, "bridge.invite")
	if err := os.WriteFile(invitePath, invite, 0o600); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(directory, "import.json")
	confidencePath := filepath.Join(directory, "time-confidence")
	if err := os.WriteFile(confidencePath, []byte("observed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(confidencePath, time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}
	rolesRoot := filepath.Join(directory, "local-roles")
	roles, err := localroles.Open(localroles.Config{Root: rolesRoot, Clock: func() time.Time { return now }, Create: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := roles.Close(); err != nil {
		t.Fatal(err)
	}
	plan := map[string]any{
		"state_root": filepath.Join(directory, "bridge-state"), "network_state_root": network.root,
		"invite_file": invitePath, "network_id": hex32(network.snapshot.NetworkID),
		"network_authorities": []string{hex.EncodeToString(network.authorityPublic)},
		"network_threshold":   1, "network_profile": "ardents-interactive-route-v2",
		"local_role_state_root": rolesRoot,
		"time_confidence_file":  confidencePath,
	}
	rawPlan, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(planPath, rawPlan, 0o600); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if err := os.Chtimes(confidencePath, time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := run(t.Context(), []string{"entry", "import", planPath}, &output); err != nil {
		t.Fatalf("run import: %v", err)
	}
	var event struct {
		Class      string `json:"class"`
		InviteID   string `json:"invite_id"`
		Slot       uint8  `json:"slot"`
		Generation uint8  `json:"generation"`
	}
	if err := json.Unmarshal(output.Bytes(), &event); err != nil {
		t.Fatal(err)
	}
	if event.Class != "accepted" || len(event.InviteID) != 64 || event.Slot != 0 || event.Generation != 1 {
		t.Fatalf("unexpected import event: %+v", event)
	}
	output.Reset()
	if err := os.Chtimes(confidencePath, time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := run(t.Context(), []string{"entry", "import", planPath}, &output); err != nil {
		t.Fatalf("run idempotent import: %v", err)
	}
	if err := json.Unmarshal(output.Bytes(), &event); err != nil || event.Class != "already-present" {
		t.Fatalf("idempotent event = %+v, %v", event, err)
	}
	plan["state_root"] = filepath.Join(directory, "uncertain-bridge-state")
	if err := os.Chtimes(confidencePath, now.Add(-time.Minute), now.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	rawPlan, err = json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(planPath, rawPlan, 0o600); err != nil {
		t.Fatal(err)
	}
	output.Reset()
	if err := run(t.Context(), []string{"entry", "import", planPath}, &output); err != nil {
		t.Fatalf("run uncertain import: %v", err)
	}
	if err := json.Unmarshal(output.Bytes(), &event); err != nil || event.Class != "incompatible" {
		t.Fatalf("uncertain event = %+v, %v", event, err)
	}
	if err := os.Chtimes(confidencePath, time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}

	plan["state_root"] = filepath.Join(directory, "conflicting-bridge-state")
	candidateFacts, _ := network.snapshot.BridgeCandidateByKey(network.snapshot.Candidates[0].KeyID)
	roles, err = localroles.Open(localroles.Config{Root: rolesRoot, Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	if err := roles.Replace([32]byte{9}, []localroles.Duty{{Identity: candidateFacts.NodeID,
		Family: candidateFacts.FamilyID, Class: "route-rendezvous", State: "live", NotAfter: now.Add(time.Hour)}}); err != nil {
		t.Fatal(err)
	}
	if err := roles.Close(); err != nil {
		t.Fatal(err)
	}
	rawPlan, err = json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(planPath, rawPlan, 0o600); err != nil {
		t.Fatal(err)
	}
	output.Reset()
	if err := os.Chtimes(confidencePath, time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := run(t.Context(), []string{"entry", "import", planPath}, &output); err != nil {
		t.Fatalf("run conflicting import: %v", err)
	}
	if err := json.Unmarshal(output.Bytes(), &event); err != nil || event.Class != "conflicting-role" {
		t.Fatalf("conflicting event = %+v, %v", event, err)
	}
}

func commandInvite(fixture commandNetwork, now time.Time) []byte {
	snapshot := fixture.snapshot
	var body bytes.Buffer
	_ = binary.Write(&body, binary.BigEndian, uint16(2))
	body.Write(snapshot.NetworkID[:])
	_ = binary.Write(&body, binary.BigEndian, snapshot.Epoch)
	body.Write(snapshot.Digest[:])
	writeCommandBytes(&body, []byte("ardents-interactive-route-v2"), 1)
	recipient := [32]byte{91}
	body.Write(recipient[:])
	candidateFacts, _ := snapshot.BridgeCandidateByKey(snapshot.Candidates[0].KeyID)
	body.Write(candidateFacts.KeyID[:])
	body.Write(candidateFacts.NodeID[:])
	body.Write(candidateFacts.FamilyID[:])
	body.Write(candidateFacts.RecordDigest[:])
	body.Write(candidateFacts.DomainProofDigest[:])
	_ = binary.Write(&body, binary.BigEndian, candidateFacts.AssignmentNotAfter.Unix())
	_ = binary.Write(&body, binary.BigEndian, now.Add(-time.Minute).Unix())
	_ = binary.Write(&body, binary.BigEndian, now.Add(30*time.Minute).Unix())
	body.Write([]byte{1, 0, 0})

	var raw bytes.Buffer
	raw.WriteString("ardents-entry-invite-v2")
	_ = binary.Write(&raw, binary.BigEndian, uint16(body.Len()))
	raw.Write(body.Bytes())
	signed := append([]byte("ardents-entry-invite-signature-v2\x00"), body.Bytes()...)
	raw.Write(ed25519.Sign(fixture.nodePrivate, signed))
	return raw.Bytes()
}

func writeCommandBytes(target *bytes.Buffer, raw []byte, width int) {
	if width == 1 {
		target.WriteByte(byte(len(raw)))
	} else {
		_ = binary.Write(target, binary.BigEndian, uint16(len(raw)))
	}
	target.Write(raw)
}

func hex32(value [32]byte) string { return hex.EncodeToString(value[:]) }
