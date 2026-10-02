//go:build linux

package issuer

import (
	"bytes"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuance"
	"github.com/dianabuilds/ardents-network/internal/successor/nodeidentity"
)

func TestProfileCompositionOpensConsistentOwnersAndClosesThem(t *testing.T) {
	base := t.TempDir()
	if e := os.Chmod(base, 0700); e != nil {
		t.Fatal(e)
	}
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{19}, 32))
	der, e := x509.MarshalPKCS8PrivateKey(private)
	if e != nil {
		t.Fatal(e)
	}
	source := filepath.Join(base, "source.pem")
	if e = os.WriteFile(source, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0600); e != nil {
		t.Fatal(e)
	}
	identityBinding := nodeidentity.Binding{Network: [32]byte{1}, Node: [32]byte{2}, Signer: [32]byte(private.Public().(ed25519.PublicKey))}
	start := time.Unix(3600, 0).UTC()
	keyBinding := issuance.Binding{Network: identityBinding.Network, Issuer: identityBinding.Node, Signer: identityBinding.Signer, Start: start, End: start.Add(time.Hour)}
	p := ProfilePlan{IdentityRoot: filepath.Join(base, "identity"), IdentityBinding: identityBinding, KeyRoot: filepath.Join(base, "keys"), KeyBinding: keyBinding, ProfileRoot: filepath.Join(base, "profile")}
	if e = nodeidentity.Import(t.Context(), p.IdentityRoot, source, identityBinding); e != nil {
		t.Fatal(e)
	}
	if e = issuance.Initialize(t.Context(), p.KeyRoot, keyBinding); e != nil {
		t.Fatal(e)
	}
	held, e := issuance.Open(t.Context(), p.KeyRoot, keyBinding)
	if e != nil {
		t.Fatal(e)
	}
	if got := InitializeProfile(t.Context(), p); got.Outcome != "busy" {
		t.Fatal(got)
	}
	if _, e = os.Stat(p.ProfileRoot); !os.IsNotExist(e) {
		t.Fatal("busy created profile")
	}
	if e = held.Close(); e != nil {
		t.Fatal(e)
	}
	created := InitializeProfile(t.Context(), p)
	if created.Outcome != "profile-initialized" || len(created.Response) == 0 {
		t.Fatal(created)
	}
	reopened := InspectProfile(t.Context(), p)
	if reopened.Outcome != "profile-verified" || !bytes.Equal(created.Response, reopened.Response) {
		t.Fatal(reopened)
	}
	identity, e := nodeidentity.Open(t.Context(), p.IdentityRoot, identityBinding)
	if e != nil {
		t.Fatal(e)
	}
	if e = identity.Close(); e != nil {
		t.Fatal(e)
	}
	// A second valid Node identity cannot be used to inspect the first one's keys.
	otherBinding := identityBinding
	otherBinding.Node[0] ^= 1
	otherRoot := filepath.Join(base, "other")
	if e = nodeidentity.Import(t.Context(), otherRoot, source, otherBinding); e != nil {
		t.Fatal(e)
	}
	bad := p
	bad.IdentityRoot = otherRoot
	bad.IdentityBinding = otherBinding
	if got := InspectProfile(t.Context(), bad); got.Outcome != "invalid-input" || got.Response != nil {
		t.Fatal(got)
	}
}
