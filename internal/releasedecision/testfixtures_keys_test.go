package releasedecision

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"sync"
	"testing"

	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

// makeFiveEd25519Keys creates the five top-level release keys. The
// helpers are independent: each signSyntheticMetadata call uses the
// supplied keys, so the same set may be reused for root, timestamp,
// snapshot, and targets.
func makeFiveEd25519Keys(t *testing.T) []syntheticKey {
	t.Helper()
	keys := make([]syntheticKey, 0, totalTopLevelKeys)
	for index := 0; index < totalTopLevelKeys; index++ {
		public, private, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		signer, err := signature.LoadSigner(private, crypto.Hash(0))
		if err != nil {
			t.Fatal(err)
		}
		keys = append(keys, syntheticKey{
			id:     fmt.Sprintf("key-%02d", index),
			signer: signer,
			public: public,
		})
	}
	return keys
}

var signSyntheticOnce sync.Mutex

// signSyntheticMetadata attaches the supplied signers to the
// metadata. It serializes its work behind a process-level mutex
// because the underlying sigstore signer holds a private-state
// counter and is not safe to call concurrently.
func signSyntheticMetadata(t *testing.T, meta signable, keys []syntheticKey) {
	signSyntheticMetadataCount(t, meta, keys, len(keys))
}

func signSyntheticMetadataCount(t *testing.T, meta signable, keys []syntheticKey, count int) {
	t.Helper()
	signSyntheticOnce.Lock()
	defer signSyntheticOnce.Unlock()
	if count > len(keys) {
		count = len(keys)
	}
	for _, key := range keys[:count] {
		if _, err := meta.Sign(key.signer); err != nil {
			t.Fatal(err)
		}
	}
}

// signable is the subset of the go-tuf metadata methods we use to
// sign in tests. Both Metadata[T] types implement it.
type signable interface {
	Sign(signer signature.Signer) (*metadata.Signature, error)
}
