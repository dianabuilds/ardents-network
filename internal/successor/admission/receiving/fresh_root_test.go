package receiving

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/spending"
)

// This component supplies external authority facts at Receiving's boundary.
// Signed Network authenticity remains covered by the separate command scenario.
func TestReceivingFreshRootCannotSurviveAttemptOrRetainedReopen(t *testing.T) {
	window := time.Unix(1_800_000_000, 0).UTC().Truncate(time.Hour)
	receiver := Receiver{NetworkID: [32]byte{1}, StateGeneration: [32]byte{2}, StateDigest: [32]byte{3}, ProfileDigest: [32]byte{4}, NodeID: [32]byte{5}, DutyGeneration: 6}
	profile := admission.AuthorityFacts{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest, Digest: receiver.ProfileDigest, IssuerNodeID: [32]byte{7}, IssuanceAuthorityKey: [32]byte{8}, IssuerDutyGeneration: 9, NotBefore: window, NotAfter: window.Add(time.Hour), TokenKeyCount: 3}
	for i := range 3 {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatal(err)
		}
		spki, err := encodeSelectedRSAPSSSPKI(&key.PublicKey)
		if err != nil {
			t.Fatal(err)
		}
		profile.TokenKeys[i] = admission.TokenKey{WindowStart: window, Class: uint8(i + 1)}
		copy(profile.TokenKeys[i].SPKI[:], spki)
	}
	observe := func() (Observation, error) {
		return Observation{Profile: profile, Receiver: receiver, Now: window.Add(time.Minute), NotAfter: profile.NotAfter}, nil
	}
	binding := spending.Binding{NetworkID: receiver.NetworkID, ProfileDigest: receiver.ProfileDigest, ReceiverNodeID: receiver.NodeID, ReceiverDutyGeneration: receiver.DutyGeneration}
	for _, attempt := range []bool{false, true} {
		t.Run(map[bool]string{false: "completed creation", true: "refused admission"}[attempt], func(t *testing.T) {
			root := t.TempDir()
			owner, err := Open(root, receiver, observe)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = owner.Close() })
			fact, err := owner.TakeFreshRoot()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := owner.TakeFreshRoot(); err == nil {
				t.Fatal("receiving issued creation fact twice")
			}
			initialization, err := fact.Begin(binding)
			if err != nil {
				t.Fatal(err)
			}
			if err := initialization.Check(); err != nil {
				t.Fatal(err)
			}
			if attempt {
				// Even an invalid call refused before token verification is an
				// admission attempt; it cannot leave companion-root creation live.
				if _, err := owner.Accept(t.Context(), admission.RegistrationClass, nil, time.Time{}, nil); err == nil {
					t.Fatal("invalid receiving call accepted")
				}
				if err := initialization.Check(); err == nil {
					t.Fatal("refused admission retained creation authority")
				}
			}
			if err := initialization.Complete(); (err != nil) != attempt {
				t.Fatal("creation completion did not match original lifetime", err)
			}
			if err := owner.Close(); err != nil {
				t.Fatal(err)
			}
			if _, err := owner.TakeFreshRoot(); err == nil {
				t.Fatal("closed receiving owner issued creation fact")
			}
			reopened, err := Open(root, receiver, observe)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			if _, err := reopened.TakeFreshRoot(); err == nil {
				t.Fatal("retained header-only root reissued creation fact")
			}
		})
	}
}
