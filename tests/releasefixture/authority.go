package releasefixture

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

// ReleaseAuthority signs consecutive snapshots without manufacturing a
// second first-pin or replacing the retained Root's five distinct public keys.
type ReleaseAuthority struct {
	rootBytes []byte
	signers   []signature.Signer
}

// NewReleaseAuthority makes an independent fixture Root with five ephemeral keys.
func NewReleaseAuthority(t testing.TB, expires time.Time) *ReleaseAuthority {
	t.Helper()
	root := metadata.Root(expires)
	root.Signed.UnrecognizedFields = map[string]any{"ardents_schema_version": 1, "ardents_profile": "ardents-h3-release-v1", "ardents_environment": "h3-test", "ardents_network": "new-release-test"}
	owned := &ReleaseAuthority{}
	var ids []string
	for range 5 {
		public, private, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		signer, err := signature.LoadSigner(private, crypto.Hash(0))
		if err != nil {
			t.Fatal(err)
		}
		key, err := metadata.KeyFromPublicKey(public)
		if err != nil {
			t.Fatal(err)
		}
		id, err := key.ID()
		if err != nil {
			t.Fatal(err)
		}
		root.Signed.Keys[id] = key
		ids = append(ids, id)
		owned.signers = append(owned.signers, signer)
	}
	for _, role := range []string{"root", "timestamp", "snapshot", "targets"} {
		root.Signed.Roles[role] = &metadata.Role{KeyIDs: append([]string(nil), ids...), Threshold: 3}
	}
	for _, signer := range owned.signers[:3] {
		if _, err := root.Sign(signer); err != nil {
			t.Fatal(err)
		}
	}
	var err error
	owned.rootBytes, err = root.ToBytes(false)
	if err != nil {
		t.Fatal(err)
	}
	return owned
}

// Sign signs fixture metadata with the original three-of-five quorum.
func (a *ReleaseAuthority) Sign(t testing.TB, value interface {
	Sign(signature.Signer) (*metadata.Signature, error)
}) {
	t.Helper()
	for _, signer := range a.signers[:3] {
		if _, err := value.Sign(signer); err != nil {
			t.Fatal(err)
		}
	}
}

// RootBytes returns the canonical signed Root independently of the artifact.
func (a *ReleaseAuthority) RootBytes() []byte {
	return append([]byte(nil), a.rootBytes...)
}
