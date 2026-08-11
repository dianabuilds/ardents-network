package nativecircuit

import (
	"crypto/ecdh"
	"crypto/rand"
	"testing"
	"time"
)

func TestInvitationHPKELabelRemainsProtocolCompatible(t *testing.T) {
	t.Parallel()
	const establishedLabel = "ardents/carrier-lab/c5-c2/invitation/v1"
	if hpkeInfoLabel != establishedLabel || string(hpkeInfo) != establishedLabel {
		t.Fatalf("HPKE invitation label changed: got %q", hpkeInfo)
	}
}

func TestInvitationIsBoundAndSingleUse(t *testing.T) {
	t.Parallel()
	privateKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_786_294_800, 0)
	want := invitation{
		SchemaVersion:  invitationSchema,
		Profile:        "carrier-lab-c5-c2/v1",
		RunID:          "20260810T120000Z-native",
		Rendezvous:     "rendezvous:37001",
		JoinToken:      randomHandle(t),
		HandshakeNonce: randomHandle(t),
		ExpiresUnix:    now.Add(time.Minute).Unix(),
	}
	sealed, err := sealInvitation(privateKey.PublicKey(), want)
	if err != nil {
		t.Fatal(err)
	}
	guard := newInvitationGuard(want.Profile, want.RunID, want.Rendezvous, now)
	got, err := guard.open(privateKey, sealed)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("opened invitation differs: got %#v want %#v", got, want)
	}
	if _, err := guard.open(privateKey, sealed); err == nil {
		t.Fatal("replayed invitation was accepted")
	}
}

func TestInvitationRejectsWrongBindingBeforeJoin(t *testing.T) {
	t.Parallel()
	for _, changed := range []string{"profile", "run", "rendezvous"} {
		changed := changed
		t.Run(changed, func(t *testing.T) {
			t.Parallel()
			privateKey, err := ecdh.X25519().GenerateKey(rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			now := time.Unix(1_786_294_800, 0)
			sealed, err := sealInvitation(privateKey.PublicKey(), invitation{
				Profile: "carrier-lab-c5-c2/v1", RunID: "run-a", Rendezvous: "rv-a",
				JoinToken: randomHandle(t), HandshakeNonce: randomHandle(t), ExpiresUnix: now.Add(time.Minute).Unix(),
			})
			if err != nil {
				t.Fatal(err)
			}
			profile, runID, rendezvous := "carrier-lab-c5-c2/v1", "run-a", "rv-a"
			switch changed {
			case "profile":
				profile = "carrier-lab-c3/v1"
			case "run":
				runID = "run-b"
			case "rendezvous":
				rendezvous = "rv-b"
			}
			if _, err := newInvitationGuard(profile, runID, rendezvous, now).open(privateKey, sealed); err == nil {
				t.Fatalf("invitation with wrong %s binding was accepted", changed)
			}
		})
	}
}

func randomHandle(t *testing.T) handle {
	t.Helper()
	var value handle
	if _, err := rand.Read(value[:]); err != nil {
		t.Fatal(err)
	}
	return value
}
