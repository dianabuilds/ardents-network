//go:build ignore

// R-073 measures the fixed proof envelope for one signed Record.
package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/naming/namespace"
)

const (
	recordCount = 127
	proofLimit  = 4096
)

type result struct {
	Records              int `json:"records"`
	ThresholdSignatures  int `json:"threshold_signatures"`
	MaximumConflictBytes int `json:"maximum_conflict_bytes"`
	FirstRejectedBytes   int `json:"first_rejected_bytes"`
	SignedRecordBytes    int `json:"signed_record_bytes"`
	ProofBytes           int `json:"proof_bytes"`
	FixedProofLimit      int `json:"fixed_proof_limit"`
}

func main() {
	network := [32]byte{7, 3}
	policy, attesters := materializationPolicy(network)
	low, high := 0, proofLimit*2
	for low < high {
		middle := (low + high + 1) / 2
		if _, _, err := measure(network, policy, attesters, middle); err == nil {
			low = middle
		} else {
			high = middle - 1
		}
	}
	signed, proof, err := measure(network, policy, attesters, low)
	must(err)
	if _, _, err := measure(network, policy, attesters, low+1); err == nil {
		fail(fmt.Errorf("next Record size unexpectedly fit"))
	}
	encoded, err := json.Marshal(result{Records: recordCount, ThresholdSignatures: len(attesters),
		MaximumConflictBytes: low, FirstRejectedBytes: low + 1, SignedRecordBytes: signed,
		ProofBytes: proof, FixedProofLimit: proofLimit})
	must(err)
	fmt.Println(string(encoded))
}

func measure(network [32]byte, policy namespace.MaterializationPolicy, attesters []ed25519.PrivateKey,
	conflictBytes int,
) (int, int, error) {
	root, err := os.MkdirTemp("", "ardents-r073-")
	if err != nil {
		return 0, 0, err
	}
	defer os.RemoveAll(root)
	store, err := namespace.Open(root, policy)
	if err != nil {
		return 0, 0, err
	}
	defer store.Close()
	keySeed := sha256.Sum256([]byte("r073-record-authority"))
	key := ed25519.NewKeyFromSeed(keySeed[:])
	records := make([][]byte, recordCount)
	for index := range records {
		name := fmt.Sprintf("b%d", index)
		if index == 0 {
			name = "a"
		}
		record := namespace.Record{Name: name, Generation: 1, Revision: 1, Lease: "active",
			Consistency: "current", Recovery: "stable", Authority: hex.EncodeToString(key.Public().(ed25519.PublicKey)),
			LeaseExpiresAt: 1_000, GraceExpiresAt: 2_000, Continuity: 1}
		if index == 0 {
			record.Consistency = "conflict"
			record.ConflictIdentifier = strings.Repeat("x", conflictBytes)
		}
		records[index], err = namespace.SignRecord(network, record, key)
		if err != nil {
			return 0, 0, err
		}
	}
	epoch := namespace.Epoch{Number: 1, Digest: [32]byte{1}, CutoffOffset: 1,
		TransitionRoot: sha256.Sum256([]byte("r073-transitions")), TransitionLength: recordCount,
		RejectionRoot: sha256.Sum256([]byte("r073-rejections"))}
	if err := store.Commit(epoch, records, thresholdAttester(attesters)); err != nil {
		return 0, 0, err
	}
	proof, err := store.Lookup("a", epoch.Number)
	if err != nil {
		return 0, 0, err
	}
	return len(records[0]), len(proof), nil
}

func materializationPolicy(network [32]byte) (namespace.MaterializationPolicy, []ed25519.PrivateKey) {
	policy := namespace.MaterializationPolicy{Network: network, Rule: "ardents-namespace-materialization-v1",
		Authorities: make(map[[32]byte]ed25519.PublicKey), Threshold: 2}
	keys := make([]ed25519.PrivateKey, 16)
	for index := range keys {
		seed := sha256.Sum256([]byte(fmt.Sprintf("r073-attester-%d", index)))
		keys[index] = ed25519.NewKeyFromSeed(seed[:])
		public := keys[index].Public().(ed25519.PublicKey)
		policy.Authorities[sha256.Sum256(public)] = public
	}
	return policy, keys
}

func thresholdAttester(keys []ed25519.PrivateKey) func([]byte) ([][32]byte, [][]byte, error) {
	return func(transcript []byte) ([][32]byte, [][]byte, error) {
		type signature struct {
			id  [32]byte
			raw []byte
		}
		values := make([]signature, len(keys))
		for index, key := range keys {
			values[index] = signature{id: sha256.Sum256(key.Public().(ed25519.PublicKey)), raw: ed25519.Sign(key, transcript)}
		}
		sort.Slice(values, func(first, second int) bool { return bytes.Compare(values[first].id[:], values[second].id[:]) < 0 })
		ids, signatures := make([][32]byte, len(values)), make([][]byte, len(values))
		for index, value := range values {
			ids[index], signatures[index] = value.id, value.raw
		}
		return ids, signatures, nil
	}
}

func must(err error) {
	if err != nil {
		fail(err)
	}
}

func fail(err error) {
	_, _ = fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
