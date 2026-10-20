//go:build linux && (network_preparation_native || network_signing_native)

package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/closedprofile"
	"github.com/dianabuilds/ardents-network/internal/successor/network/epoch"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// These purpose-specific test signers consume the exact prepared messages.
// They are isolated fixture keys, never a product signer or operator evidence.
func signPreparedNodeForTest(key ed25519.PrivateKey, message []byte) []byte {
	return append(bytes.Clone(message), ed25519.Sign(key, message)...)
}
func signPreparedEpochForTest(key ed25519.PrivateKey, message []byte) []byte {
	digest := sha256.Sum256(message)
	id := sha256.Sum256(key.Public().(ed25519.PublicKey))
	raw := append(bytes.Clone(message), 1)
	raw = append(raw, id[:]...)
	return append(raw, ed25519.Sign(key, digest[:])...)
}
func signPreparedProfileForTest(key ed25519.PrivateKey, message []byte) []byte {
	return append(bytes.Clone(message), ed25519.Sign(key, append([]byte("ardents-closed-profile-v3\x00"), message...))...)
}

func TestNetworkPreparationCommandsGenuineSignedStateChain(t *testing.T) {
	runNetworkPreparedStateChain(t, nil)
}

func runNetworkPreparedStateChain(t *testing.T, signer func(*testing.T, string, any, ed25519.PrivateKey, []byte) ([]byte, [][]byte), generated ...ed25519.PrivateKey) {
	command := compiledCommand(t)
	for label, path := range map[string]string{"command": command, "test": os.Args[0]} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("actual %s SHA256 %x", label, sha256.Sum256(raw))
	}
	start := time.Now().UTC().Truncate(time.Hour)
	end := start.Add(2 * time.Hour)
	key := func(n byte) ed25519.PrivateKey { return ed25519.NewKeyFromSeed(bytes.Repeat([]byte{n}, 32)) }
	stateKey, nodeKey, permissionKey, secondKey := key(41), key(42), key(43), key(44)
	if len(generated) == 3 {
		stateKey, nodeKey, secondKey = generated[0], generated[1], generated[2]
	}
	var permissionPublic [32]byte
	copy(permissionPublic[:], permissionKey.Public().(ed25519.PublicKey))
	spec := networkfixture.ClosedSpec{NetworkID: [32]byte{1}, Seed: [32]byte{13}, IssuanceAuthority: permissionPublic, Number: 1, NotBefore: start, NotAfter: end, Authority: stateKey, Nodes: []networkfixture.ClosedNode{
		{RecordSpec: networkfixture.RecordSpec{NodeID: [32]byte{2}, Generation: 1, ValidFrom: start, ValidUntil: end, Endpoint: "127.0.0.1:4401", Carrier: epoch.CarrierClosedTCP, Capability: 2, Capacity: 3, PrivateKey: nodeKey}, RoleDomain: 2, Subrole: 6},
		{RecordSpec: networkfixture.RecordSpec{NodeID: [32]byte{3}, Generation: 1, ValidFrom: start, ValidUntil: end, Endpoint: "127.0.0.1:4402", Carrier: epoch.CarrierClosedTCP, Capability: 2, Capacity: 3, PrivateKey: secondKey}, RoleDomain: 4, Subrole: 3},
	}}
	fixture, err := networkfixture.BuildClosed(spec)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := closedprofile.Verify(fixture.Profile, closedprofile.Context{NetworkID: spec.NetworkID, StateGeneration: fixture.Epoch.Digest, EpochDigest: fixture.Epoch.Digest, Epoch: 1, Authority: stateKey.Public().(ed25519.PublicKey), Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	// Independent fixture fixes its family once before any command invocation.
	// Production receives that exact family and seed and performs no search.
	prepare := func(operation string, plan any, wantName string) []byte {
		t.Helper()
		parent := t.TempDir()
		if err := os.Chmod(parent, 0700); err != nil {
			t.Fatal(err)
		}
		input := filepath.Join(parent, "plan.json")
		root := filepath.Join(parent, "prepared")
		if err := os.WriteFile(input, preparationJSON(t, plan), 0600); err != nil {
			t.Fatal(err)
		}
		cmd := exec.CommandContext(t.Context(), compiledCommand(t), "network", operation, input, root)
		var out, diagnostic bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &diagnostic
		if err := cmd.Run(); err != nil {
			t.Fatal(operation, err, out.String(), diagnostic.String())
		}
		message, err := os.ReadFile(filepath.Join(root, wantName))
		if err != nil {
			t.Fatal(err)
		}
		var receipt struct {
			SHA256   string `json:"unsigned_sha256"`
			Signed   bool   `json:"signed"`
			Accepted bool   `json:"state_accepted"`
			Artifact string `json:"artifact"`
		}
		digest := sha256.Sum256(message)
		if json.Unmarshal(out.Bytes(), &receipt) != nil || receipt.Signed || receipt.Accepted || receipt.SHA256 != preparationHex(digest[:]) || receipt.Artifact != wantName {
			t.Fatal("misleading preparation receipt", out.String())
		}
		// An exact retry cannot adopt an existing completed output directory.
		retry := exec.CommandContext(t.Context(), compiledCommand(t), "network", operation, input, root)
		if e := retry.Run(); e == nil {
			t.Fatal("existing output adopted")
		}
		return message
	}
	p := preparationRecordPlan(nodeKey.Public().(ed25519.PublicKey))
	p["valid_from"] = start.Format(time.RFC3339)
	p["valid_until"] = end.Format(time.RFC3339)
	p["family"] = fixture.Nodes[0].Family
	nodeMessage := prepare("prepare-initial-node-record", p, "node-record.signing-input")
	record := signPreparedNodeForTest(nodeKey, nodeMessage)
	if signer != nil {
		record, _ = signer(t, "node-record", p, nodeKey, nodeMessage)
	}
	if !bytes.Equal(record, fixture.Epoch.Inputs[0]) {
		t.Fatal("record differs from independent canonical oracle")
	}
	secondPlan := preparationRecordPlan(secondKey.Public().(ed25519.PublicKey))
	secondPlan["node_id"], secondPlan["family"], secondPlan["endpoint"] = preparationIdentifier(3), fixture.Nodes[1].Family, "127.0.0.1:4402"
	secondPlan["valid_from"], secondPlan["valid_until"] = start.Format(time.RFC3339), end.Format(time.RFC3339)
	secondMessage := prepare("prepare-initial-node-record", secondPlan, "node-record.signing-input")
	secondRecord := signPreparedNodeForTest(secondKey, secondMessage)
	if signer != nil {
		secondRecord, _ = signer(t, "node-record", secondPlan, secondKey, secondMessage)
	}
	if !bytes.Equal(secondRecord, fixture.Epoch.Inputs[1]) {
		t.Fatal("second record differs from independent oracle")
	}
	epochPlan := map[string]any{"schema": "ardents-initial-epoch-plan-v1", "network_id": preparationIdentifier(1), "assignment_seed": preparationIdentifier(13), "valid_from": start.Format(time.RFC3339), "valid_until": end.Format(time.RFC3339), "domains": []string{"initiator", "introduction", "rendezvous", "responder"}, "records": []string{preparationHex(record), preparationHex(secondRecord)}, "authority_keys": []string{preparationHex(stateKey.Public().(ed25519.PublicKey))}}
	epochMessage := prepare("prepare-initial-epoch", epochPlan, "epoch.unsigned")
	signedEpoch := signPreparedEpochForTest(stateKey, epochMessage)
	materials := fixture.Epoch.Materials
	if signer != nil {
		signedEpoch, materials = signer(t, "epoch", epochPlan, stateKey, epochMessage)
		if len(materials) != len(fixture.Epoch.Materials) {
			t.Fatal("product materialization inventory differs")
		}
		for i := range materials {
			if !bytes.Equal(materials[i], fixture.Epoch.Materials[i]) {
				t.Fatal("product materialization differs from independent oracle", i)
			}
		}
	}
	if !bytes.Equal(signedEpoch, fixture.Epoch.Raw) {
		t.Fatal("Epoch differs from independent commitment/assignment oracle")
	}
	for _, fault := range []string{"signature", "network", "duplicate", "seed", "domains", "authority collision", "quic"} {
		t.Run("Epoch refusal "+fault, func(t *testing.T) {
			var q map[string]any
			_ = json.Unmarshal(preparationJSON(t, epochPlan), &q)
			switch fault {
			case "signature":
				bad := bytes.Clone(record)
				bad[len(bad)-1] ^= 1
				q["records"] = []string{preparationHex(bad)}
			case "network":
				q["network_id"] = preparationIdentifier(99)
			case "duplicate":
				q["records"] = []string{preparationHex(record), preparationHex(record)}
			case "seed":
				q["assignment_seed"] = preparationIdentifier(0)
			case "domains":
				q["domains"] = []string{"responder", "initiator"}
			case "authority collision":
				q["authority_keys"] = []string{preparationHex(nodeKey.Public().(ed25519.PublicKey))}
			case "quic":
				r := fixture.Nodes[0].RecordSpec
				r.Carrier = epoch.CarrierClosedQUIC
				bad, e := networkfixture.BuildRecord(r)
				if e != nil {
					t.Fatal(e)
				}
				q["records"] = []string{preparationHex(bad.Raw)}
			}
			if _, message, e := prepareNetworkPublic("prepare-initial-epoch", preparationJSON(t, q)); e == nil || message != nil {
				t.Fatal("invalid Epoch preparation accepted")
			}
		})
	}
	nodes := []map[string]any{}
	keys := []map[string]any{}
	for _, n := range profile.Nodes {
		nodes = append(nodes, map[string]any{"node_id": preparationHex(n.NodeID[:]), "record_digest": preparationHex(n.RecordDigest[:]), "role_domain": n.RoleDomain, "subrole": n.Subrole, "duty_generation": n.DutyGeneration})
	}
	for _, k := range profile.Keys {
		keys = append(keys, map[string]any{"window_start": time.Unix(int64(k.WindowStart), 0).UTC().Format(time.RFC3339), "class": k.Class, "spki": preparationHex(k.SPKI)})
	}
	profilePlan := map[string]any{"schema": "ardents-closed-profile-plan-v1", "network_id": preparationIdentifier(1), "state_generation": preparationHex(fixture.Epoch.Digest[:]), "epoch_digest": preparationHex(fixture.Epoch.Digest[:]), "issuer_node_id": preparationIdentifier(2), "issuance_authority_key": preparationHex(permissionPublic[:]), "epoch": 1, "not_before": start.Format(time.RFC3339), "not_after": end.Format(time.RFC3339), "nodes": nodes, "token_keys": keys}
	profileMessage := prepare("prepare-closed-profile", profilePlan, "profile.unsigned")
	signedProfile := signPreparedProfileForTest(stateKey, profileMessage)
	if signer != nil {
		signedProfile, _ = signer(t, "closed-profile", profilePlan, stateKey, profileMessage)
	}
	if !bytes.Equal(signedProfile, fixture.Profile) {
		t.Fatal("profile differs from independent canonical oracle")
	}
	for _, fault := range []string{"spki", "duplicate node", "interval", "fraction", "nested unknown", "nested duplicate"} {
		t.Run("profile refusal "+fault, func(t *testing.T) {
			var q map[string]any
			_ = json.Unmarshal(preparationJSON(t, profilePlan), &q)
			switch fault {
			case "spki":
				q["token_keys"].([]any)[0].(map[string]any)["spki"] = "00"
			case "duplicate node":
				q["nodes"] = []any{q["nodes"].([]any)[0], q["nodes"].([]any)[0]}
			case "interval":
				q["not_after"] = start.Format(time.RFC3339)
			case "fraction":
				q["not_before"] = start.Add(time.Nanosecond).Format(time.RFC3339Nano)
			case "nested unknown":
				q["nodes"].([]any)[0].(map[string]any)["private_key"] = "no"
			}
			raw := preparationJSON(t, q)
			if fault == "nested duplicate" {
				raw = bytes.Replace(raw, []byte(`"class":1`), []byte(`"class":1,"class":1`), 1)
			}
			if _, message, e := prepareNetworkPublic("prepare-closed-profile", raw); e == nil || message != nil {
				t.Fatal("invalid profile preparation accepted")
			}
		})
	}
	clock := filepath.Join(t.TempDir(), "clock")
	if err := os.WriteFile(clock, nil, 0600); err != nil {
		t.Fatal(err)
	}
	var public [32]byte
	copy(public[:], stateKey.Public().(ed25519.PublicKey))
	accept := func(plan networkAuthorityPlan, operation string, value any, want bool) {
		t.Helper()
		if err := os.Chtimes(clock, time.Now(), time.Now()); err != nil {
			t.Fatal(err)
		}
		config := map[string]any{"network": plan}
		switch operation {
		case "accept-epoch":
			config["epoch"] = value
			config["inputs"] = [][]byte{record, secondRecord}
			config["materials"] = materials
		case "accept-profile":
			config["profile"] = value
		}
		cmd := exec.CommandContext(t.Context(), compiledCommand(t), "network", operation, "--config", hostingConfig(t, config))
		out, err := cmd.CombinedOutput()
		if (err == nil) != want {
			t.Fatalf("actual State command %s accepted=%v: %v %s", operation, want, err, out)
		}
	}
	for _, fault := range []string{"healthy", "changed Epoch", "foreign State key", "purpose confused profile", "Epoch purpose profile", "changed profile", "foreign record digest", "foreign role", "foreign issuer", "foreign duty"} {
		t.Run("State "+fault, func(t *testing.T) {
			plan := networkAuthorityPlan{Root: filepath.Join(t.TempDir(), "state"), NetworkID: spec.NetworkID, Authorities: [][32]byte{public}, Threshold: 1, ProfileAuthority: public, ClockObservationFile: clock}
			candidateEpoch := bytes.Clone(signedEpoch)
			if fault == "changed Epoch" {
				candidateEpoch[len(candidateEpoch)-1] ^= 1
				accept(plan, "accept-epoch", candidateEpoch, false)
				accept(plan, "accept-epoch", signedEpoch, true)
				return
			}
			if fault == "foreign State key" {
				candidateEpoch = signPreparedEpochForTest(permissionKey, epochMessage)
				accept(plan, "accept-epoch", candidateEpoch, false)
				accept(plan, "accept-epoch", signedEpoch, true)
				return
			}
			accept(plan, "accept-epoch", candidateEpoch, true)
			candidateProfile := bytes.Clone(signedProfile)
			switch fault {
			case "purpose confused profile":
				candidateProfile = signPreparedNodeForTest(stateKey, profileMessage)
			case "Epoch purpose profile":
				digest := sha256.Sum256(profileMessage)
				candidateProfile = append(bytes.Clone(profileMessage), ed25519.Sign(stateKey, digest[:])...)
			case "changed profile":
				candidateProfile[len(candidateProfile)-1] ^= 1
			case "foreign record digest":
				body := bytes.Clone(profileMessage)
				body[244] ^= 1
				candidateProfile = signPreparedProfileForTest(stateKey, body)
			case "foreign role":
				body := bytes.Clone(profileMessage)
				// A valid grammar role for the second real Node, different from
				// its Epoch assignment. Only State's membership join can refuse it.
				body[334], body[335] = 1, 1
				candidateProfile = signPreparedProfileForTest(stateKey, body)
			case "foreign issuer":
				body := bytes.Clone(profileMessage)
				body[130], body[196] = 1, 1
				candidateProfile = signPreparedProfileForTest(stateKey, body)
			case "foreign duty":
				body := bytes.Clone(profileMessage)
				body[269] = 2
				candidateProfile = signPreparedProfileForTest(stateKey, body)
			}
			if fault == "foreign record digest" || fault == "foreign role" || fault == "foreign issuer" || fault == "foreign duty" {
				if _, e := closedprofile.Verify(candidateProfile, closedprofile.Context{NetworkID: spec.NetworkID, StateGeneration: fixture.Epoch.Digest, EpochDigest: fixture.Epoch.Digest, Epoch: 1, Authority: stateKey.Public().(ed25519.PublicKey), Now: time.Now()}); e != nil {
					t.Fatal("State join control lacks a genuine valid signature/public grammar", e)
				}
			}
			accept(plan, "accept-profile", candidateProfile, fault == "healthy")
			if fault != "healthy" {
				accept(plan, "accept-profile", signedProfile, true)
			}
			accept(plan, "inspect", nil, true)
		})
	}
}
