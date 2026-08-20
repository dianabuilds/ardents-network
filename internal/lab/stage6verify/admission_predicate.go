package stage6verify

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
)

type admissionCellEvidence struct {
	Node       [32]byte
	Network    [32]byte
	Epoch      uint64
	Now        int64
	Isolation  [32]byte
	Profiles   []admissionProfileEvidence
	Replay     string
	Expired    string
	Restarted  string
	CrossScope string
}

type admissionProfileEvidence struct {
	Surface         string
	WorkBits        uint8
	MaximumSpent    int
	MaximumInFlight int
	Proof           admissionProof
	Accepted        bool
	Capacity        admissionCapacityEvidence
}

type admissionCapacityEvidence struct {
	WorkNonces    []uint64
	SolveHashes   []uint64
	Overflow      string
	BusyOutcomes  []string
	PressureNanos []int64
}

type admissionProof struct {
	Challenge admissionChallenge
	WorkNonce uint64
}

type admissionChallenge struct {
	Node              [32]byte
	Network           [32]byte
	Epoch             uint64
	Surface           string
	OperationDigest   [32]byte
	IsolationBinding  [32]byte
	IssuedAt          int64
	ExpiresAt         int64
	Nonce             [16]byte
	WorkBits          uint8
	AuthenticationTag [32]byte
}

func verifyAdmissionTrace(trace traceRecord, secret [32]byte) bool {
	var evidence admissionCellEvidence
	wantProfiles := []admissionProfileEvidence{{Surface: "resolution", WorkBits: 16, MaximumSpent: 4096, MaximumInFlight: 64},
		{Surface: "renewal-update", WorkBits: 16, MaximumSpent: 2048, MaximumInFlight: 32},
		{Surface: "policy-recovery", WorkBits: 17, MaximumSpent: 1024, MaximumInFlight: 16},
		{Surface: "root-claim", WorkBits: 18, MaximumSpent: 1024, MaximumInFlight: 8}}
	if decodeNestedJSON(trace.Auxiliary, &evidence) != nil || evidence.Node == [32]byte{} ||
		evidence.Network == [32]byte{} || evidence.Epoch == 0 || evidence.Isolation == [32]byte{} ||
		evidence.Now != 950 || len(evidence.Profiles) != len(wantProfiles) || len(trace.Input) != 0 || len(trace.Output) != 0 ||
		!equalStrings(trace.Fields, []string{"accepted", "replay", "expired", "restart", "cross-scope"}) {
		return false
	}
	for index, profile := range evidence.Profiles {
		want := wantProfiles[index]
		if profile.Surface != want.Surface || profile.WorkBits != want.WorkBits ||
			profile.MaximumSpent != want.MaximumSpent || profile.MaximumInFlight != want.MaximumInFlight ||
			profile.Proof.Challenge.IssuedAt != 900 || profile.Proof.Challenge.ExpiresAt != 1_000 ||
			!profile.Accepted || !validAdmissionProof(evidence, profile.Proof, secret) ||
			!verifyAdmissionCapacity(evidence, profile, index, secret) {
			return false
		}
	}
	return evidence.Replay == "replay" && evidence.Expired == "invalid-scope" &&
		evidence.Restarted == "invalid-challenge" && evidence.CrossScope == "invalid-scope"
}

func validAdmissionProof(evidence admissionCellEvidence, proof admissionProof, secret [32]byte) bool {
	challenge := proof.Challenge
	if challenge.Node != evidence.Node || challenge.Network != evidence.Network || challenge.Epoch != evidence.Epoch ||
		challenge.IssuedAt < 0 || challenge.ExpiresAt <= challenge.IssuedAt ||
		challenge.ExpiresAt-challenge.IssuedAt > 30_000 || evidence.Now < challenge.IssuedAt ||
		evidence.Now >= challenge.ExpiresAt || challenge.WorkBits == 0 {
		return false
	}
	bindingInput := []byte("ardents-name-admission-isolation-v1\x00")
	bindingInput = append(bindingInput, challenge.Node[:]...)
	bindingInput = append(bindingInput, challenge.OperationDigest[:]...)
	bindingInput = append(bindingInput, evidence.Isolation[:]...)
	bindingInput = append(bindingInput, challenge.Nonce[:]...)
	if sha256.Sum256(bindingInput) != challenge.IsolationBinding {
		return false
	}
	mac := hmac.New(sha256.New, secret[:])
	_, _ = mac.Write(admissionBytes(challenge, false))
	if !hmac.Equal(mac.Sum(nil), challenge.AuthenticationTag[:]) {
		return false
	}
	digest := sha256.Sum256(binary.BigEndian.AppendUint64(admissionBytes(challenge, true), proof.WorkNonce))
	return leadingZeroBits(digest, int(challenge.WorkBits))
}

func admissionBytes(challenge admissionChallenge, includeTag bool) []byte {
	out := claimText(nil, "ardents-name-admission-challenge-v1")
	out = append(out, challenge.Node[:]...)
	out = append(out, challenge.Network[:]...)
	out = binary.BigEndian.AppendUint64(out, challenge.Epoch)
	out = claimText(out, challenge.Surface)
	out = append(out, challenge.OperationDigest[:]...)
	out = append(out, challenge.IsolationBinding[:]...)
	out = binary.BigEndian.AppendUint64(out, uint64(challenge.IssuedAt))
	out = binary.BigEndian.AppendUint64(out, uint64(challenge.ExpiresAt))
	out = append(out, challenge.Nonce[:]...)
	out = append(out, challenge.WorkBits)
	if includeTag {
		out = append(out, challenge.AuthenticationTag[:]...)
	}
	return out
}

func leadingZeroBits(digest [32]byte, bits int) bool {
	original := bits
	for bits >= 8 {
		if digest[(original-bits)/8] != 0 {
			return false
		}
		bits -= 8
	}
	return bits == 0 || digest[original/8]>>(8-bits) == 0
}
