//go:build ignore

package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sort"
	"time"
)

type measurement struct {
	Schema            string `json:"schema"`
	GOOS              string `json:"goos"`
	GOARCH            string `json:"goarch"`
	GoVersion         string `json:"go_version"`
	Participants      int    `json:"participants"`
	Threshold         uint8  `json:"threshold"`
	Signatures        int    `json:"signatures"`
	Iterations        int    `json:"iterations"`
	LogicalProofBytes int    `json:"logical_policy_proof_bytes"`
	HeapBytesPerRun   uint64 `json:"heap_bytes_per_run"`
	P50Nanos          int64  `json:"p50_nanos"`
	P95Nanos          int64  `json:"p95_nanos"`
	MaximumNanos      int64  `json:"maximum_nanos"`
	Outcome           string `json:"outcome"`
}

func main() {
	policy, proof := measurementFixture()
	const iterations = 10_000
	durations := make([]time.Duration, iterations)
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for i := range durations {
		started := time.Now()
		result := Authorize(policy, proof)
		durations[i] = time.Since(started)
		if result.Outcome != Authorized || result.ValidSigners != 8 {
			fmt.Fprintf(os.Stderr, "authorization failed: %+v\n", result)
			os.Exit(1)
		}
	}
	runtime.ReadMemStats(&after)
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	result := measurement{Schema: "ardents-r044-measurement-v1", GOOS: runtime.GOOS,
		GOARCH: runtime.GOARCH, GoVersion: runtime.Version(), Participants: len(policy.Participants),
		Threshold: policy.Threshold, Signatures: len(proof.Signatures), Iterations: iterations,
		LogicalProofBytes: logicalProofBytes(policy, proof),
		HeapBytesPerRun:   (after.TotalAlloc - before.TotalAlloc) / iterations,
		P50Nanos:          durations[iterations/2].Nanoseconds(), P95Nanos: durations[9499].Nanoseconds(),
		MaximumNanos: durations[iterations-1].Nanoseconds(), Outcome: string(Authorized)}
	encoded, err := json.Marshal(result)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(string(encoded))
}

func measurementFixture() (Policy, Proof) {
	currentSeed := sha256.Sum256([]byte("r044-current-authority"))
	current := ed25519.NewKeyFromSeed(currentSeed[:]).Public().(ed25519.PublicKey)
	policy := Policy{Network: [32]byte{7}, Name: "alice", Generation: 3, Revision: 2,
		Threshold: 5, Delay: 72 * time.Hour}
	copy(policy.CurrentAuthority[:], current)
	var privateKeys []ed25519.PrivateKey
	for i := 0; i < 8; i++ {
		seed := sha256.Sum256([]byte(fmt.Sprintf("r044-recovery-authority-%d", i)))
		private := ed25519.NewKeyFromSeed(seed[:])
		public := private.Public().(ed25519.PublicKey)
		var participant [32]byte
		copy(participant[:], public)
		policy.Participants = append(policy.Participants, participant)
		privateKeys = append(privateKeys, private)
	}
	sort.Slice(policy.Participants, func(i, j int) bool {
		return bytes.Compare(policy.Participants[i][:], policy.Participants[j][:]) < 0
	})
	sort.Slice(privateKeys, func(i, j int) bool {
		return bytes.Compare(privateKeys[i].Public().(ed25519.PublicKey),
			privateKeys[j].Public().(ed25519.PublicKey)) < 0
	})
	proof := Proof{Operation: Initiate, PolicyDigest: PolicyDigest(policy),
		OperationID: sha256.Sum256([]byte("r044-operation")), StartedAt: 1_000,
		CompletesAt: 1_000 + policy.Delay.Milliseconds(),
		Successor:   sha256.Sum256([]byte("r044-successor"))}
	for _, private := range privateKeys {
		public := private.Public().(ed25519.PublicKey)
		var signer [32]byte
		copy(signer[:], public)
		proof.Signatures = append(proof.Signatures, Signature{Signer: signer,
			Bytes: ed25519.Sign(private, RecoveryTranscript(policy, proof))})
	}
	return policy, proof
}

func logicalProofBytes(policy Policy, proof Proof) int {
	policyBytes := 32 + 4 + len(policy.Name) + 8 + 8 + 32 + 1 + 1 +
		len(policy.Participants)*32 + 8
	proofBytes := 4 + len(proof.Operation) + 32 + 32 + 32 + 8 + 8 + 1
	for _, signature := range proof.Signatures {
		proofBytes += 32 + len(signature.Bytes)
	}
	return policyBytes + proofBytes
}
