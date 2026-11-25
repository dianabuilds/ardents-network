//go:build linux || darwin

package processfixture

import (
	"crypto/ed25519"
	"os"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/stock"
)

// OpenPermittedStock gives the holder a signed allocation. Scenarios obtain tokens separately.
func OpenPermittedStock(t testing.TB, root string, role admission.AllocationRole, maxima [3]uint32,
	observe func() (admission.AuthorityFacts, time.Time, error), facts admission.AuthorityFacts, signingKey ed25519.PrivateKey, now time.Time) *stock.Owner {
	t.Helper()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	holder, err := stock.Open(root, role, observe)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := holder.Close(); err != nil {
			t.Error(err)
		}
	})
	raw, digest, err := holder.Request(maxima)
	if err != nil {
		t.Fatal(err)
	}
	encoded := FixturePermission(t, raw, facts, signingKey, now)
	if err := holder.Import(digest, encoded); err != nil {
		t.Fatal(err)
	}
	return holder
}
