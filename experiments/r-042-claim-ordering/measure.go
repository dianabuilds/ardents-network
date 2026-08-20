//go:build ignore

package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"flag"
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
	Claims            int    `json:"claims"`
	Iterations        int    `json:"iterations"`
	LogicalProofBytes int    `json:"logical_proof_bytes"`
	P50Nanos          int64  `json:"p50_nanos"`
	P95Nanos          int64  `json:"p95_nanos"`
	MaximumNanos      int64  `json:"maximum_nanos"`
	Outcome           string `json:"outcome"`
}

func main() {
	claimCount := flag.Int("claims", 64, "number of conflicting claims")
	iterationCount := flag.Int("iterations", 1000, "verification iterations")
	flag.Parse()
	if *claimCount < 1 || *claimCount > 64 || *iterationCount < 100 {
		fmt.Fprintln(os.Stderr, "measurement bounds are invalid")
		os.Exit(2)
	}
	policy, proof := measurementFixture(*claimCount)
	durations := make([]time.Duration, *iterationCount)
	var outcome Outcome
	for i := range durations {
		started := time.Now()
		result, err := Verify(policy, proof)
		durations[i] = time.Since(started)
		if err != nil || result.Outcome != Accepted || result.WinnerOrdinal != 1 {
			fmt.Fprintf(os.Stderr, "verification failed: result=%+v err=%v\n", result, err)
			os.Exit(1)
		}
		outcome = result.Outcome
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	result := measurement{Schema: "ardents-r042-measurement-v1", GOOS: runtime.GOOS,
		GOARCH: runtime.GOARCH, GoVersion: runtime.Version(), Claims: len(proof.Claims),
		Iterations: len(durations), LogicalProofBytes: logicalProofBytes(proof),
		P50Nanos:     durations[len(durations)/2].Nanoseconds(),
		P95Nanos:     durations[(len(durations)*95+99)/100-1].Nanoseconds(),
		MaximumNanos: durations[len(durations)-1].Nanoseconds(), Outcome: string(outcome)}
	encoded, err := json.Marshal(result)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(string(encoded))
}

func measurementFixture(claimCount int) (Policy, ClaimSetProof) {
	network := [32]byte{7}
	claimSeed := sha256.Sum256([]byte("r042-claim-authority"))
	claimPrivate := ed25519.NewKeyFromSeed(claimSeed[:])
	claimPublic := claimPrivate.Public().(ed25519.PublicKey)
	claims := make([]Claim, claimCount)
	for i := range claims {
		secret := sha256.Sum256([]byte(fmt.Sprintf("r042-secret-%02d", i)))
		claim := Claim{Ordinal: uint32(i + 1), Name: "alice", Secret: secret}
		copy(claim.Authority[:], claimPublic)
		claim.Commitment = CommitmentFor(network, 11, claim)
		claim.Signature = ed25519.Sign(claimPrivate, RevealTranscript(network, 11, claim))
		claims[i] = claim
	}
	proof := ClaimSetProof{Network: network, Epoch: 11, Rule: claimOrderRule,
		Complete: true, Claims: claims, SetRoot: claimSetRoot(claims)}
	policy := Policy{Network: network, Rule: claimOrderRule, MinimumEpoch: 11,
		MaxClaims: uint32(claimCount), Authorities: make(map[[32]byte]ed25519.PublicKey), Threshold: 2}
	for i := 0; i < 3; i++ {
		seed := sha256.Sum256([]byte(fmt.Sprintf("r042-set-authority-%d", i)))
		private := ed25519.NewKeyFromSeed(seed[:])
		public := private.Public().(ed25519.PublicKey)
		id := sha256.Sum256(public)
		policy.Authorities[id] = public
		proof.SetSignatures = append(proof.SetSignatures, SetSignature{AuthorityID: id,
			Signature: ed25519.Sign(private, claimSetTranscript(proof))})
	}
	sort.Slice(proof.SetSignatures, func(i, j int) bool {
		return bytes.Compare(proof.SetSignatures[i].AuthorityID[:], proof.SetSignatures[j].AuthorityID[:]) < 0
	})
	return policy, proof
}

func logicalProofBytes(proof ClaimSetProof) int {
	total := 32 + 8 + 4 + len(proof.Rule) + 1 + 32 + 4
	for _, claim := range proof.Claims {
		total += 4 + 4 + len(claim.Name) + 32 + 32 + 32 + len(claim.Signature)
	}
	for _, signature := range proof.SetSignatures {
		total += 32 + len(signature.Signature)
	}
	return total
}
