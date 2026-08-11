package networkstate

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestMerkleProofsCoverEveryCanonicalTreeShape(t *testing.T) {
	t.Parallel()
	for length := 1; length <= 64; length++ {
		values := make([][]byte, length)
		for index := range values {
			values[index] = []byte(fmt.Sprintf("record-%02d-of-%02d", index, length))
		}
		root := recordMerkleRoot(values, emptyViewTag)
		for index, value := range values {
			proof := propertyProof(values, index)
			if !verifyProof(value, uint32(index), uint32(length), proof, root) {
				t.Fatalf("proof failed for length=%d index=%d", length, index)
			}
		}
		changed := append([]byte(nil), values[0]...)
		changed[0] ^= 1
		values[0] = changed
		if recordMerkleRoot(values, emptyViewTag) == root {
			t.Fatalf("root did not change for length=%d", length)
		}
	}
}

func TestFrozenAssignmentTransition(t *testing.T) {
	t.Parallel()
	path := filepath.Join("..", "..", "tests", "qualification", "h3-s1-offline-v1", "testdata", "assignment-transition.json")
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

func propertyProof(values [][]byte, index int) [][32]byte {
	if len(values) <= 1 {
		return nil
	}
	split := merkleSplit(len(values))
	if index < split {
		return append(propertyProof(values[:split], index), recordMerkleRoot(values[split:], emptyViewTag))
	}
	return append(propertyProof(values[split:], index-split), recordMerkleRoot(values[:split], emptyViewTag))
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
	current := Snapshot{Epoch: maximumEpochChain, Digest: digest}
	if err := verifyEpochChain(&current, epochEnvelope{number: maximumEpochChain + 1, previous: digest}); err == nil {
		t.Fatal("write path accepted an epoch that restart retention cannot load")
	}
}
