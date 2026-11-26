package processfixture

import (
	"bytes"
	"crypto/ed25519"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/admission"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/allocation"
)

// FixturePermission commits the test allocation before using the independent
// authority key to sign its permission. It does not create holder tokens.
func FixturePermission(t testing.TB, raw []byte, facts admission.AuthorityFacts, authority ed25519.PrivateKey, now time.Time) []byte {
	t.Helper()
	prepared, err := allocation.Prepare(raw, facts.NetworkID, now)
	if err != nil {
		t.Fatal(err)
	}
	decision, err := prepared.Decide(nil, facts.IssuanceAuthorityKey, now)
	if err != nil {
		t.Fatal(err)
	}
	journal := filepath.Join(t.TempDir(), "allocation")
	original := decision.Journal()
	if err := os.WriteFile(journal, original, 0600); err != nil {
		t.Fatal(err)
	}
	committed, err := os.ReadFile(journal)
	if err != nil || !bytes.Equal(committed, original) {
		t.Fatal("durable allocation readback", err)
	}
	permission := decision.Permission()
	copy(permission.Signature[:], ed25519.Sign(authority, admission.PermissionTranscript(permission)))
	encoded, err := admission.EncodePermission(permission)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}
