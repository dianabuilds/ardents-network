package quota

import (
	"bytes"
	"crypto/ed25519"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuerprofile"
)

func TestProfileBindingPreservesIndependentAuthority(t *testing.T) {
	signer := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{8}, 32))
	start := time.Unix(3600, 0).UTC()
	b := issuerprofile.Binding{Network: [32]byte{1}, Issuer: [32]byte{2}, Signer: [32]byte(signer.Public().(ed25519.PublicKey)), Start: start, End: start.Add(time.Hour)}
	keys := []issuerprofile.Key{{Window: 3600, Class: 1, SPKI: fixtureSPKI(t, 1)}, {Window: 3600, Class: 2, SPKI: fixtureSPKI(t, 2)}, {Window: 3600, Class: 3, SPKI: fixtureSPKI(t, 3)}}
	request, err := issuerprofile.Prepare(b, keys)
	if err != nil {
		t.Fatal(err)
	}
	_, transcript, _ := request.Snapshot()
	raw, err := request.Complete(ed25519.Sign(signer, transcript))
	if err != nil {
		t.Fatal(err)
	}
	verified, err := issuerprofile.Verify(raw, b)
	if err != nil {
		t.Fatal(err)
	}
	base := LedgerBinding{Network: b.Network, Issuer: b.Issuer, Authority: [32]byte{4}, Profile: [32]byte{5}, Duty: 9, Start: b.Start, End: b.End}
	bound, err := PrepareLedgerBinding(base, verified)
	if err != nil || bound.Profile != base.Profile || bound.Authority != base.Authority || bound.Duty != base.Duty || len(bound.Keys) != 3 {
		t.Fatal("State facts replaced", err)
	}
	if _, err := PrepareLedgerBinding(bound, verified); err == nil {
		t.Fatal("preexisting keys replaced")
	}
	for _, field := range []string{"network", "issuer", "authority", "profile", "duty", "start", "end"} {
		t.Run(field, func(t *testing.T) {
			bad := base
			switch field {
			case "network":
				bad.Network = [32]byte{3}
			case "issuer":
				bad.Issuer = [32]byte{3}
			case "authority":
				bad.Authority = [32]byte{}
			case "profile":
				bad.Profile = [32]byte{}
			case "duty":
				bad.Duty = 0
			case "start":
				bad.Start = bad.Start.Add(time.Hour)
			case "end":
				bad.End = bad.End.Add(time.Hour)
			}
			if _, err := PrepareLedgerBinding(bad, verified); err == nil {
				t.Fatal("unbound authority accepted")
			}
		})
	}
	if _, err := PrepareLedgerBinding(LedgerBinding{}, issuerprofile.Verified{}); err == nil {
		t.Fatal("zero evidence accepted")
	}
}
