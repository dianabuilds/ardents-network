package state

import (
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFrozenAssignmentTransition(t *testing.T) {
	t.Parallel()
	path := filepath.Join("testdata", "assignment-transition.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var vector struct {
		Schema string `json:"schema"`
		Epoch1 struct {
			Seed        string            `json:"seed_label"`
			Assignments map[string]string `json:"assignments"`
		} `json:"epoch_1"`
		Epoch2 struct {
			Seed        string            `json:"seed_label"`
			Assignments map[string]string `json:"assignments"`
		} `json:"epoch_2"`
	}
	if err := json.Unmarshal(raw, &vector); err != nil {
		t.Fatal(err)
	}
	if vector.Schema != "ardents-h3-role-assignment-vector-v1" {
		t.Fatalf("assignment schema=%q", vector.Schema)
	}
	network := sha256.Sum256([]byte("ardents-h3-stage-1-network"))
	items := []struct {
		seed        string
		assignments map[string]string
	}{{vector.Epoch1.Seed, vector.Epoch1.Assignments}, {vector.Epoch2.Seed, vector.Epoch2.Assignments}}
	for number, item := range items {
		epoch := epochEnvelope{
			networkID: network, number: uint64(number + 1),
			assignmentSeed: sha256.Sum256([]byte(item.seed)),
			domains:        []roleDomain{{id: "alpha"}, {id: "beta"}},
		}
		for family, expected := range item.assignments {
			actual, assignErr := assignedDomain(epoch, family)
			if assignErr != nil || actual != expected {
				t.Fatalf("epoch=%d family=%s assignment=%s err=%v, want %s", number+1, family, actual, assignErr, expected)
			}
		}
	}
	if vector.Epoch1.Assignments["family-b"] == vector.Epoch2.Assignments["family-b"] {
		t.Fatal("frozen assignment-transition record did not change domains")
	}
}

func FuzzCanonicalParsers(f *testing.F) {
	f.Add([]byte("AREP"), []byte("ARNR"))
	f.Add([]byte{}, []byte{})
	f.Fuzz(func(t *testing.T, epoch, record []byte) {
		_, _ = parseEpoch(epoch)
		_, _ = parseRecord(record)
	})
}

func TestEpochChainBoundMatchesRestartRetention(t *testing.T) {
	t.Parallel()
	var digest [32]byte
	digest[0] = 1
	current := epochVerificationSnapshot{Epoch: maximumEpochChain, Digest: digest}
	if err := verifyEpochChain(&current, epochEnvelope{number: maximumEpochChain + 1, previous: digest}); err == nil {
		t.Fatal("write path accepted an epoch that restart retention cannot load")
	}
}
