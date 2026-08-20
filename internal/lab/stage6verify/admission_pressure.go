package stage6verify

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"sort"
)

const admissionPressureSamples = 10_000

func verifyAdmissionCapacity(evidence admissionCellEvidence, profile admissionProfileEvidence,
	profileIndex int, secret [32]byte,
) bool {
	capacity := profile.Capacity
	if len(capacity.WorkNonces) != profile.MaximumSpent || len(capacity.SolveHashes) != profile.MaximumSpent ||
		capacity.Overflow != "capacity" || len(capacity.BusyOutcomes) != profile.MaximumInFlight*16+1 ||
		!validAdmissionBusyOutcomes(capacity.BusyOutcomes) ||
		len(capacity.PressureNanos) != admissionPressureSamples {
		return false
	}
	for ordinal, nonce := range capacity.WorkNonces {
		proof := pressureAdmissionProof(evidence, profile, profileIndex, ordinal, nonce, secret)
		if capacity.SolveHashes[ordinal] != nonce+1 || !validAdmissionProof(evidence, proof, secret) {
			return false
		}
	}
	durations := append([]int64(nil), capacity.PressureNanos...)
	for _, duration := range durations {
		if duration < 0 {
			return false
		}
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	return durations[9499] <= 100_000
}

func validAdmissionBusyOutcomes(values []string) bool {
	busy, expired := false, false
	for _, value := range values {
		switch value {
		case "busy":
			busy = true
		case "insufficient-work":
			expired = true
		default:
			return false
		}
	}
	return busy && expired
}

func pressureAdmissionProof(evidence admissionCellEvidence, profile admissionProfileEvidence,
	profileIndex, ordinal int, workNonce uint64, secret [32]byte,
) admissionProof {
	label := []byte("ardents-stage-6-admission-pressure-v1\x00" + profile.Surface)
	label = binary.BigEndian.AppendUint32(label, uint32(ordinal))
	var nonce [16]byte
	nonce[0] = byte(profileIndex + 1)
	binary.BigEndian.PutUint64(nonce[8:], uint64(ordinal+1))
	challenge := admissionChallenge{Node: evidence.Node, Network: evidence.Network, Epoch: evidence.Epoch,
		Surface: profile.Surface, OperationDigest: sha256.Sum256(label),
		IsolationBinding: pressureIsolationBinding(evidence.Node, sha256.Sum256(label), evidence.Isolation, nonce),
		IssuedAt:         900, ExpiresAt: 951, Nonce: nonce, WorkBits: profile.WorkBits}
	mac := hmac.New(sha256.New, secret[:])
	_, _ = mac.Write(admissionBytes(challenge, false))
	copy(challenge.AuthenticationTag[:], mac.Sum(nil))
	return admissionProof{Challenge: challenge, WorkNonce: workNonce}
}

func pressureIsolationBinding(node, operation, isolation [32]byte, nonce [16]byte) [32]byte {
	input := []byte("ardents-name-admission-isolation-v1\x00")
	input = append(input, node[:]...)
	input = append(input, operation[:]...)
	input = append(input, isolation[:]...)
	input = append(input, nonce[:]...)
	return sha256.Sum256(input)
}
