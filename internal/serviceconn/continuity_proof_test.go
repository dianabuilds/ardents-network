package serviceconn

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
)

func TestContinuityProofKnownAnswerAndBindingMutations(t *testing.T) {
	continuity := [32]byte{1, 2, 3}
	credential := Credential{Target: [32]byte{4}, InstancePublic: [32]byte{5}, Generation: 7,
		NotBefore: 10, NotAfter: 90, NetworkID: [32]byte{6}, Capabilities: 3}
	binding := Recovery{CandidateView: [32]byte{7}, IsolationContext: [32]byte{8},
		DestinationBinding: [32]byte{9}, RouteProfile: "h3-recovery-tracer-v1",
		WorkSafetyNotAfter: 70, WorkSafetyMaximum: 90, NoNewRecoveryAfter: 65}
	attachment := &securedAttachment{generation: 2, exporterCommitment: [32]byte{10}}
	state := continuityState{sendBase: 11, sendEnd: 29, recvNext: 17}
	nonce := proofNonce(continuity, attachment, true)
	encoded := encodeContinuityProofNonce(continuity, credential, binding, attachment, true, state, nonce)
	digest := sha256.Sum256(encoded)
	const expected = "f1e7a4bfac65cfe33f7ecf6c8b6414fb8d2fe2dd28ad5cbcdc681f18200275f6"
	if hex.EncodeToString(digest[:]) != expected {
		t.Fatalf("continuity proof vector changed: %x", digest)
	}
	peer, err := decodeContinuityProof(encoded, continuity, credential, binding, attachment, true)
	if err != nil || peer.sendBase != state.sendBase || peer.sendEnd != state.sendEnd ||
		peer.recvNext != state.recvNext || peer.peerNonce != nonce {
		t.Fatalf("valid continuity proof rejected: peer=%+v err=%v", peer, err)
	}
	newAttachment := &securedAttachment{generation: 3, exporterCommitment: [32]byte{11}}
	reusedNonce := encodeContinuityProofNonce(continuity, credential, binding, newAttachment, true, state, nonce)
	if _, err := decodeContinuityProof(reusedNonce, continuity, credential, binding, newAttachment, true); !errors.Is(err, errActiveViolation) {
		t.Fatalf("old nonce with a valid new proof MAC was accepted: %v", err)
	}

	mutations := map[string]func([]byte, *Credential, *Recovery, *securedAttachment, *[32]byte){
		"protocol":   func(value []byte, _ *Credential, _ *Recovery, _ *securedAttachment, _ *[32]byte) { value[4]++ },
		"role":       func(value []byte, _ *Credential, _ *Recovery, _ *securedAttachment, _ *[32]byte) { value[5]++ },
		"generation": func(value []byte, _ *Credential, _ *Recovery, _ *securedAttachment, _ *[32]byte) { value[13]++ },
		"nonce":      func(value []byte, _ *Credential, _ *Recovery, _ *securedAttachment, _ *[32]byte) { value[38]++ },
		"binding":    func(value []byte, _ *Credential, _ *Recovery, _ *securedAttachment, _ *[32]byte) { value[70]++ },
		"exporter":   func(value []byte, _ *Credential, _ *Recovery, _ *securedAttachment, _ *[32]byte) { value[102]++ },
		"mac":        func(value []byte, _ *Credential, _ *Recovery, _ *securedAttachment, _ *[32]byte) { value[134]++ },
		"target": func(_ []byte, value *Credential, _ *Recovery, _ *securedAttachment, _ *[32]byte) {
			value.Target[0]++
		},
		"instance": func(_ []byte, value *Credential, _ *Recovery, _ *securedAttachment, _ *[32]byte) {
			value.InstancePublic[0]++
		},
		"network": func(_ []byte, value *Credential, _ *Recovery, _ *securedAttachment, _ *[32]byte) {
			value.NetworkID[0]++
		},
		"context": func(_ []byte, _ *Credential, value *Recovery, _ *securedAttachment, _ *[32]byte) {
			value.IsolationContext[0]++
		},
		"profile": func(_ []byte, _ *Credential, value *Recovery, _ *securedAttachment, _ *[32]byte) {
			value.RouteProfile += "-other"
		},
		"destination": func(_ []byte, _ *Credential, value *Recovery, _ *securedAttachment, _ *[32]byte) {
			value.DestinationBinding[0]++
		},
		"safety": func(_ []byte, _ *Credential, value *Recovery, _ *securedAttachment, _ *[32]byte) {
			value.NoNewRecoveryAfter--
		},
		"fresh exporter": func(_ []byte, _ *Credential, _ *Recovery, value *securedAttachment, _ *[32]byte) {
			value.exporterCommitment[0]++
		},
		"connection key": func(_ []byte, _ *Credential, _ *Recovery, _ *securedAttachment, value *[32]byte) { value[0]++ },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			proof := append([]byte(nil), encoded...)
			changedCredential, changedBinding, changedAttachment, changedContinuity := credential, binding, *attachment, continuity
			mutate(proof, &changedCredential, &changedBinding, &changedAttachment, &changedContinuity)
			if _, err := decodeContinuityProof(proof, changedContinuity, changedCredential,
				changedBinding, &changedAttachment, true); !errors.Is(err, errActiveViolation) {
				t.Fatalf("mutated continuity proof accepted: %v", err)
			}
		})
	}
	for _, malformed := range [][]byte{nil, encoded[:continuityProofSize-1], append(encoded, 0)} {
		if _, err := decodeContinuityProof(malformed, continuity, credential, binding, attachment, true); !errors.Is(err, errActiveViolation) {
			t.Fatalf("malformed proof accepted: %v", err)
		}
	}
}

func TestCutoverRejectsOffsetRollback(t *testing.T) {
	failed := &securedAttachment{generation: 1}
	fresh := &securedAttachment{generation: 2}
	stream := &recoveryStream{current: failed, sendBase: 10, sendEnd: 20, recvNext: 15}
	rollback := peerContinuity{sendBase: 0, sendEnd: 20, recvNext: 9, peerNonce: [32]byte{2}, localNonce: [32]byte{3}}
	if !errors.Is(stream.commitAttachment(failed, fresh, rollback), errActiveViolation) {
		t.Fatal("acknowledgement rollback was accepted")
	}
}
