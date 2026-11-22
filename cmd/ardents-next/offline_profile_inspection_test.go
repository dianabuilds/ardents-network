package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission/issuerprofile"
	"github.com/dianabuilds/ardents-network/internal/network/closedprofile"
)

func TestOfflineProfileInspectionUsesIndependentPublicPins(t *testing.T) {
	now := time.Unix(2_000_401_600, 0).UTC().Truncate(time.Hour)
	state := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{3}, ed25519.SeedSize))
	node := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{4}, ed25519.SeedSize))
	network, issuer, generation, epoch := [32]byte{1}, [32]byte{2}, [32]byte{5}, [32]byte{6}
	var nodeKey [32]byte
	copy(nodeKey[:], node.Public().(ed25519.PublicKey))
	write := func(name string, raw []byte) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), name)
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	check := func(args []string, wanted string) {
		t.Helper()
		var output bytes.Buffer
		if code := run(t.Context(), args, &output, io.Discard); code != 0 {
			t.Fatalf("inspection exit %d: %s", code, output.Bytes())
		}
		var receipt struct {
			Schema string `json:"schema"`
		}
		if err := json.Unmarshal(output.Bytes(), &receipt); err != nil || receipt.Schema != wanted {
			t.Fatalf("unexpected receipt %s: %v", output.Bytes(), err)
		}
	}
	refuse := func(args []string) {
		t.Helper()
		var output bytes.Buffer
		if code := run(t.Context(), args, &output, io.Discard); code == 0 || output.Len() != 0 {
			t.Fatalf("substituted public pin produced receipt %s, exit %d", output.Bytes(), code)
		}
	}
	issuerProfile := signedPublicIssuerProfile(t, network, issuer, node, now)
	issuerArgs := []string{"issuance", "inspect-public-profile", write("issuer.bin", issuerProfile), hex.EncodeToString(nodeKey[:]), hex.EncodeToString(network[:]), hex.EncodeToString(issuer[:])}
	check(issuerArgs, "ardents-closed-issuer-inspection-v1")
	statePin := hex.EncodeToString(state.Public().(ed25519.PublicKey))
	issuerArgs[3] = statePin
	refuse(issuerArgs)

	profile, err := issuerprofile.Inspect(issuerProfile, issuerprofile.PublicPins{Network: network, Issuer: issuer, Signer: nodeKey})
	if err != nil {
		t.Fatal(err)
	}
	_, keys, ok := profile.Snapshot()
	if !ok {
		t.Fatal("issuer fixture has no public inventory")
	}
	input := closedprofile.Input{
		NetworkID: network, StateGeneration: generation, EpochDigest: epoch, Epoch: 7,
		IssuerNodeID: issuer, IssuanceAuthorityKey: [32]byte{7}, NotBefore: now, NotAfter: now.Add(time.Hour),
		Nodes: []closedprofile.NodeInput{{NodeID: issuer, RecordDigest: [32]byte{8}, RoleDomain: 2, Subrole: 6, DutyGeneration: 1}},
	}
	for _, key := range keys {
		input.TokenKeys = append(input.TokenKeys, closedprofile.TokenKeyInput{
			WindowStart: time.Unix(int64(key.Window), 0).UTC(), Class: key.Class, SPKI: key.SPKI,
		})
	}
	body, err := closedprofile.Prepare(input)
	if err != nil {
		t.Fatal(err)
	}
	signed := append(body, ed25519.Sign(state, append([]byte("ardents-closed-profile-v3\x00"), body...))...)
	args := []string{"network", "inspect-closed-profile", write("closed.bin", signed), statePin, hex.EncodeToString(network[:]), hex.EncodeToString(generation[:]), hex.EncodeToString(epoch[:]), "7", now.Format(time.RFC3339)}
	check(args, "ardents-closed-profile-inspection-v1")
	args[3] = hex.EncodeToString(nodeKey[:])
	refuse(args)
}
