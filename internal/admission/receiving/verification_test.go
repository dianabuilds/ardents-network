package receiving

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha512"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/token"
	"github.com/dianabuilds/ardents-network/internal/network/state"
)

func TestReceiverVerificationBindsCurrentAuthorityAndToken(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	spki, err := encodeSelectedRSAPSSSPKI(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	window := time.Unix(1_800_000_000, 0).UTC().Truncate(time.Hour)
	now := window.Add(time.Minute)
	receiver := Receiver{NetworkID: [32]byte{1}, StateGeneration: [32]byte{2}, StateDigest: [32]byte{3}, ProfileDigest: [32]byte{4}, NodeID: [32]byte{5}, DutyGeneration: 6}
	profile := state.ClosedProfileView{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest, Digest: receiver.ProfileDigest, IssuerNodeID: [32]byte{7}, NotBefore: window, NotAfter: window.Add(time.Hour), TokenKeyCount: 1}
	profile.TokenKeys[0] = state.ClosedProfileTokenKey{WindowStart: window, Class: 2}
	copy(profile.TokenKeys[0].SPKI[:], spki)
	context := token.ClosedTokenContext{NetworkID: receiver.NetworkID, ProfileDigest: receiver.ProfileDigest, IssuerNodeID: profile.IssuerNodeID, ReceiverNodeID: receiver.NodeID, ReceiverDutyGeneration: receiver.DutyGeneration, Class: 2, WindowStart: window}
	_, input, _, err := token.ClosedTokenChallenge(context, spki, [32]byte{8})
	if err != nil {
		t.Fatal(err)
	}
	digest := sha512.Sum384(input)
	signature, err := rsa.SignPSS(rand.Reader, key, crypto.SHA384, digest[:], &rsa.PSSOptions{SaltLength: crypto.SHA384.Size(), Hash: crypto.SHA384})
	if err != nil {
		t.Fatal(err)
	}
	raw := append(input, signature...)
	if err := token.VerifyClosedToken(context, spki, raw); err != nil {
		t.Fatalf("independent signed token fixture: %v", err)
	}
	if got, err := VerifyToken(profile, receiver, admission.ForwardClass, raw, now); err != nil || got != window {
		t.Fatalf("valid receiver verification: %s %v", got, err)
	}
	for _, fault := range []string{"network", "state-generation", "state-digest", "profile", "receiver", "duty", "issuer", "class", "zero-class", "future-profile", "expired-profile", "old-key-hour", "wrong-key-class", "wrong-key", "oversize-key-count", "missing-key", "signature", "framing", "zero-time"} {
		t.Run(fault, func(t *testing.T) {
			p, r, class, clock := profile, receiver, admission.ForwardClass, now
			body := append([]byte(nil), raw...)
			switch fault {
			case "network":
				p.NetworkID[0]++
			case "state-generation":
				p.StateGeneration[0]++
			case "state-digest":
				p.StateDigest[0]++
			case "profile":
				p.Digest[0]++
			case "receiver":
				r.NodeID[0]++
			case "duty":
				r.DutyGeneration++
			case "issuer":
				p.IssuerNodeID[0]++
			case "class":
				class = admission.ControlClass
			case "zero-class":
				class = 0
			case "future-profile":
				p.NotBefore = now.Add(time.Second)
			case "expired-profile":
				clock = p.NotAfter
			case "old-key-hour":
				p.TokenKeys[0].WindowStart = window.Add(-time.Hour)
			case "wrong-key-class":
				p.TokenKeys[0].Class = 1
			case "wrong-key":
				p.TokenKeys[0].SPKI[0]++
			case "oversize-key-count":
				p.TokenKeyCount = 255
			case "missing-key":
				p.TokenKeyCount = 0
			case "signature":
				body[len(body)-1]++
			case "framing":
				body = body[:len(body)-1]
			case "zero-time":
				clock = time.Time{}
			}
			if got, err := VerifyToken(p, r, class, body, clock); err == nil || !got.IsZero() {
				t.Fatalf("invalid binding granted spend hour %s: %v", got, err)
			}
		})
	}
}
