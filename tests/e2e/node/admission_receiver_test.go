package node_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/issuerprofile"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/receiving"
)

func admissionConsoleFixture(t *testing.T) (string, string, string) {
	t.Helper()
	binary := buildNodeCommand(t)
	if wait := time.Until(time.Now().UTC().Truncate(time.Hour).Add(time.Hour)); wait < time.Minute {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-t.Context().Done():
			t.Fatal(t.Context().Err())
		}
	}
	start := time.Now().UTC().Truncate(time.Hour)
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(private)
	network, issuer := [32]byte{1}, [32]byte{2}
	var signer [32]byte
	copy(signer[:], public)
	raw := signedPublicIssuerProfile(t, network, issuer, private, start)
	profile, err := issuerprofile.Inspect(raw, issuerprofile.PublicPins{Network: network, Issuer: issuer, Signer: signer})
	if err != nil {
		t.Fatal(err)
	}
	_, keys, ok := profile.Snapshot()
	if !ok {
		t.Fatal("missing issuer inventory")
	}
	facts := admission.AuthorityFacts{NetworkID: network, StateGeneration: [32]byte{3}, StateDigest: [32]byte{4}, Digest: [32]byte{5}, IssuanceAuthorityKey: [32]byte{6}, IssuerNodeID: issuer, IssuerDutyGeneration: 1, NotBefore: start, NotAfter: start.Add(time.Hour), TokenKeyCount: uint8(len(keys))}
	for i, key := range keys {
		facts.TokenKeys[i] = admission.TokenKey{WindowStart: time.Unix(int64(key.Window), 0).UTC(), Class: key.Class}
		copy(facts.TokenKeys[i].SPKI[:], key.SPKI)
	}
	dir := t.TempDir()
	write := func(name string, value any) string {
		t.Helper()
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	profilePath := write("profile.json", facts)
	receiver := receiving.Receiver{NetworkID: network, StateGeneration: facts.StateGeneration, StateDigest: facts.StateDigest, ProfileDigest: facts.Digest, NodeID: [32]byte{7}, DutyGeneration: 1}
	root := filepath.Join(dir, "spends")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	config := write("receiver.json", map[string]any{"root": root, "profile": profilePath, "receiver": receiver, "not_after": facts.NotAfter})
	holderRoot := filepath.Join(dir, "holder")
	if err := os.Mkdir(holderRoot, 0700); err != nil {
		t.Fatal(err)
	}
	holderConfig := write("holder.json", map[string]any{"root": holderRoot, "profile": profilePath, "role": admission.AllocationUser})
	return binary, config, holderConfig
}

func TestAdmissionConsolesCloseAndReopen(t *testing.T) {
	binary, config, holderConfig := admissionConsoleFixture(t)
	for range 2 {
		send, closeConsole := processfixture.OpenAdmissionConsole(t, binary, "receiver", config)
		if reply := send(map[string]string{"operation": "release"}); reply.Outcome != "completed" {
			t.Fatal("receiver refused release", reply)
		}
		closeConsole()
	}
	for range 2 {
		send, closeConsole := processfixture.OpenAdmissionConsole(t, binary, "holder", holderConfig)
		if reply := send(map[string]string{"operation": "status"}); reply.Outcome != "completed" {
			t.Fatal("holder refused status", reply)
		}
		closeConsole()
	}
}
