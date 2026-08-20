//go:build ignore

package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime"
	"sort"
	"time"
)

type surfaceMeasurement struct {
	Surface            Surface `json:"surface"`
	WorkBits           uint8   `json:"work_bits"`
	MaxSpent           int     `json:"max_spent"`
	SolveSamples       int     `json:"solve_samples"`
	SolveP50Nanos      int64   `json:"solve_p50_nanos"`
	SolveP95Nanos      int64   `json:"solve_p95_nanos"`
	SolveMaximumNanos  int64   `json:"solve_maximum_nanos"`
	HashesP50          uint64  `json:"hashes_p50"`
	HashesP95          uint64  `json:"hashes_p95"`
	VerifyIterations   int     `json:"verify_iterations"`
	VerifyP50Nanos     int64   `json:"verify_p50_nanos"`
	VerifyP95Nanos     int64   `json:"verify_p95_nanos"`
	VerifyMaximumNanos int64   `json:"verify_maximum_nanos"`
	RetainedHeapBytes  uint64  `json:"retained_heap_bytes_at_cap"`
	LogicalProofBytes  int     `json:"logical_proof_bytes"`
}

type admissionMeasurement struct {
	Schema    string               `json:"schema"`
	Candidate string               `json:"candidate"`
	GOOS      string               `json:"goos"`
	GOARCH    string               `json:"goarch"`
	GoVersion string               `json:"go_version"`
	Surfaces  []surfaceMeasurement `json:"surfaces"`
}

func main() {
	candidate := flag.String("candidate", "o1", "frozen candidate profile: o1 or o1b")
	flag.Parse()
	profiles, ok := measurementProfiles(*candidate)
	if !ok {
		fmt.Fprintln(os.Stderr, "candidate is invalid")
		os.Exit(2)
	}
	result := admissionMeasurement{Schema: "ardents-r045-measurement-v1",
		Candidate: *candidate, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, GoVersion: runtime.Version()}
	for _, profile := range profiles {
		measurement, err := measureSurface(profile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		result.Surfaces = append(result.Surfaces, measurement)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(string(encoded))
}

func measurementProfiles(candidate string) ([]Profile, bool) {
	profiles := []Profile{
		{Surface: Resolution, WorkBits: 18, MaxSpent: 4_096, MaxInFlight: 64},
		{Surface: Update, WorkBits: 19, MaxSpent: 2_048, MaxInFlight: 32},
		{Surface: Recovery, WorkBits: 20, MaxSpent: 1_024, MaxInFlight: 16},
		{Surface: Claim, WorkBits: 22, MaxSpent: 1_024, MaxInFlight: 8},
	}
	if candidate == "o1" {
		return profiles, true
	}
	if candidate != "o1b" {
		return nil, false
	}
	profiles[0].WorkBits = 16
	profiles[1].WorkBits = 16
	profiles[2].WorkBits = 17
	profiles[3].WorkBits = 18
	return profiles, true
}

func measureSurface(profile Profile) (surfaceMeasurement, error) {
	gate, err := NewAdmission(measurementConfig(profile))
	if err != nil {
		return surfaceMeasurement{}, err
	}
	const solveSamples = 20
	solveDurations := make([]time.Duration, solveSamples)
	hashes := make([]uint64, solveSamples)
	var proof Proof
	for i := range solveDurations {
		challenge, issueErr := gate.Issue(1_000, measurementRequest(profile.Surface, byte(i+1)))
		if issueErr != nil {
			return surfaceMeasurement{}, issueErr
		}
		started := time.Now()
		proof, hashes[i] = Solve(challenge)
		solveDurations[i] = time.Since(started)
	}
	sort.Slice(solveDurations, func(i, j int) bool { return solveDurations[i] < solveDurations[j] })
	sort.Slice(hashes, func(i, j int) bool { return hashes[i] < hashes[j] })
	verifyDurations, err := measureVerification(gate, proof)
	if err != nil {
		return surfaceMeasurement{}, err
	}
	return surfaceMeasurement{Surface: profile.Surface, WorkBits: profile.WorkBits,
		MaxSpent: profile.MaxSpent, SolveSamples: solveSamples,
		SolveP50Nanos:     solveDurations[solveSamples/2].Nanoseconds(),
		SolveP95Nanos:     solveDurations[18].Nanoseconds(),
		SolveMaximumNanos: solveDurations[solveSamples-1].Nanoseconds(),
		HashesP50:         hashes[solveSamples/2], HashesP95: hashes[18],
		VerifyIterations: len(verifyDurations), VerifyP50Nanos: verifyDurations[len(verifyDurations)/2].Nanoseconds(),
		VerifyP95Nanos:     verifyDurations[94_999].Nanoseconds(),
		VerifyMaximumNanos: verifyDurations[len(verifyDurations)-1].Nanoseconds(),
		RetainedHeapBytes:  retainedHeapAtCap(profile),
		LogicalProofBytes:  len(challengeBytes(proof.Challenge, true)) + 8}, nil
}

func measureVerification(gate *Admission, proof Proof) ([]time.Duration, error) {
	const iterations = 100_000
	durations := make([]time.Duration, iterations)
	digest := challengeDigest(proof.Challenge)
	for i := range durations {
		started := time.Now()
		result := gate.Verify(1_050, proof)
		durations[i] = time.Since(started)
		if result.Outcome != Admitted {
			return nil, fmt.Errorf("verification failed: %+v", result)
		}
		delete(gate.spent[proof.Challenge.Surface], digest)
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	return durations, nil
}

func retainedHeapAtCap(profile Profile) uint64 {
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	gate, err := NewAdmission(measurementConfig(profile))
	if err != nil {
		return ^uint64(0)
	}
	spent := gate.spent[profile.Surface]
	for i := 0; i < profile.MaxSpent; i++ {
		spent[sha256.Sum256([]byte(fmt.Sprintf("spent-%s-%d", profile.Surface, i)))] = 30_000
	}
	runtime.GC()
	runtime.ReadMemStats(&after)
	runtime.KeepAlive(gate)
	if after.HeapAlloc < before.HeapAlloc {
		return 0
	}
	return after.HeapAlloc - before.HeapAlloc
}

func measurementConfig(profile Profile) Config {
	return Config{Node: [32]byte{1}, Network: [32]byte{2}, Epoch: 3,
		BootSecret: [32]byte{4}, MaxTTLMillis: 30_000, Profiles: []Profile{profile}}
}

func measurementRequest(surface Surface, nonce byte) Request {
	return Request{Surface: surface, OperationDigest: sha256.Sum256([]byte{nonce, 1}),
		IsolationContext: sha256.Sum256([]byte{nonce, 2}), ExpiresAt: 2_000,
		Nonce: [16]byte{nonce}}
}
