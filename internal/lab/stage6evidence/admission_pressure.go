package stage6evidence

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"runtime"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/nameadmission"
)

const admissionPressureSamples = 10_000

func measureAdmissionCapacity(secret [32]byte, evidence admissionCellEvidence, profileIndex int,
	profile admissionProfileEvidence,
) (admissionCapacityEvidence, error) {
	gate, err := nameadmission.NewAdmission(evidence.Node, evidence.Network, evidence.Epoch, secret)
	if err != nil {
		return admissionCapacityEvidence{}, err
	}
	proofs := make([]nameadmission.Proof, profile.MaximumSpent)
	hashes := make([]uint64, profile.MaximumSpent)
	var workers sync.WaitGroup
	jobs := make(chan int)
	for range 12 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for ordinal := range jobs {
				challenge, issueErr := pressureChallenge(gate, evidence, profileIndex, profile.Surface, ordinal)
				if issueErr == nil {
					proofs[ordinal], hashes[ordinal] = challenge.Solve()
				}
			}
		}()
	}
	for ordinal := range proofs {
		jobs <- ordinal
	}
	close(jobs)
	workers.Wait()
	result := admissionCapacityEvidence{WorkNonces: make([]uint64, len(proofs)), SolveHashes: hashes}
	for ordinal, proof := range proofs {
		if proof.Challenge.Surface == "" {
			return admissionCapacityEvidence{}, errors.New("admission pressure challenge failed")
		}
		ok, reason := gate.Verify(evidence.Now, proof)
		if !ok || reason != "" {
			return admissionCapacityEvidence{}, errors.New("admission capacity rejected an in-bound proof")
		}
		result.WorkNonces[ordinal] = proof.WorkNonce
	}
	overflowChallenge, err := pressureChallenge(gate, evidence, profileIndex, profile.Surface, len(proofs))
	if err != nil {
		return admissionCapacityEvidence{}, err
	}
	overflow := nameadmission.Proof{Challenge: overflowChallenge}
	_, result.Overflow = gate.Verify(evidence.Now, overflow)
	if result.Overflow != "capacity" {
		return admissionCapacityEvidence{}, errors.New("admission capacity did not fail closed")
	}
	busyChallenge, err := pressureChallengeUntil(gate, evidence, profileIndex, profile.Surface, len(proofs)+1, 1_000)
	if err != nil {
		return admissionCapacityEvidence{}, err
	}
	if err := measureAdmissionBusy(secret, evidence, proofs, nameadmission.Proof{Challenge: busyChallenge},
		profile.MaximumInFlight, &result); err != nil {
		return admissionCapacityEvidence{}, err
	}
	if !containsAdmissionOutcome(result.BusyOutcomes, "busy") {
		return admissionCapacityEvidence{}, errors.New("admission in-flight limit was not observed")
	}
	result.PressureNanos = make([]int64, admissionPressureSamples)
	for index := range result.PressureNanos {
		started := time.Now()
		_, reason := gate.Verify(evidence.Now, overflow)
		result.PressureNanos[index] = time.Since(started).Nanoseconds()
		if reason != "capacity" {
			return admissionCapacityEvidence{}, errors.New("admission pressure outcome changed")
		}
	}
	return result, nil
}

func pressureChallenge(gate *nameadmission.Admission, evidence admissionCellEvidence, profileIndex int,
	surface string, ordinal int,
) (nameadmission.Challenge, error) {
	return pressureChallengeUntil(gate, evidence, profileIndex, surface, ordinal, 951)
}

func pressureChallengeUntil(gate *nameadmission.Admission, evidence admissionCellEvidence, profileIndex int,
	surface string, ordinal int, expires int64,
) (nameadmission.Challenge, error) {
	label := []byte("ardents-stage-6-admission-pressure-v1\x00" + surface)
	label = binary.BigEndian.AppendUint32(label, uint32(ordinal))
	operation := sha256.Sum256(label)
	var nonce [16]byte
	nonce[0] = byte(profileIndex + 1)
	binary.BigEndian.PutUint64(nonce[8:], uint64(ordinal+1))
	return gate.Issue(900, surface, operation, evidence.Isolation, expires, nonce)
}

func measureAdmissionBusy(secret [32]byte, evidence admissionCellEvidence, proofs []nameadmission.Proof,
	probe nameadmission.Proof, maximum int, result *admissionCapacityEvidence,
) error {
	previousProcs := runtime.GOMAXPROCS(0)
	if previousProcs < maximum+1 {
		runtime.GOMAXPROCS(maximum + 1)
		defer runtime.GOMAXPROCS(previousProcs)
	}
	attempts := maximum*16 + 1
	for range 8 {
		gate, err := nameadmission.NewAdmission(evidence.Node, evidence.Network, evidence.Epoch, secret)
		if err != nil {
			return err
		}
		for _, proof := range proofs {
			if ok, reason := gate.Verify(evidence.Now, proof); !ok || reason != "" {
				return errors.New("admission busy fixture did not reach capacity")
			}
		}
		result.BusyOutcomes = concurrentAdmissionOutcomes(gate, probe, attempts)
		if containsAdmissionOutcome(result.BusyOutcomes, "busy") {
			return nil
		}
	}
	return errors.New("admission in-flight limit was not observed")
}

func concurrentAdmissionOutcomes(gate *nameadmission.Admission, proof nameadmission.Proof, attempts int) []string {
	start := make(chan struct{})
	outcomes := make(chan string, attempts)
	var ready, workers sync.WaitGroup
	for range attempts {
		ready.Add(1)
		workers.Add(1)
		go func() {
			defer workers.Done()
			ready.Done()
			<-start
			_, reason := gate.Verify(951, proof)
			outcomes <- reason
		}()
	}
	ready.Wait()
	close(start)
	workers.Wait()
	close(outcomes)
	result := make([]string, 0, attempts)
	for outcome := range outcomes {
		result = append(result, outcome)
	}
	return result
}

func containsAdmissionOutcome(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
