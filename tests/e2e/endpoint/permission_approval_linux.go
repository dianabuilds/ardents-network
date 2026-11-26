//go:build linux && (document_worker_installed || endpoint_launch_native)

package endpoint

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

// ApproveFixturePermission persists and reads back the fixture allocation
// before the driver signs the response. The participant receives no key.
func ApproveFixturePermission(t testing.TB, journal string, request []byte, facts admission.AuthorityFacts, signingKey ed25519.PrivateKey, now time.Time, retainJournal bool) []byte {
	t.Helper()
	prepared, err := allocation.Prepare(request, facts.NetworkID, now)
	if err != nil {
		t.Fatal(err)
	}
	var prior []byte
	if retainJournal {
		prior, err = os.ReadFile(journal)
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	decision, err := prepared.Decide(prior, facts.IssuanceAuthorityKey, now)
	if err != nil {
		t.Fatal(err)
	}
	expected := prior
	if !decision.Repeated() {
		expected = decision.Journal()
		if retainJournal {
			pending := journal + ".next"
			WriteLaunchFile(t, pending, expected, 0600)
			if err := os.Rename(pending, journal); err != nil {
				t.Fatal(err)
			}
			SyncStartupPath(t, filepath.Dir(journal))
		} else {
			WriteStartupFile(t, journal, expected, 0600)
		}
	}
	readback, err := os.ReadFile(journal)
	if err != nil || !bytes.Equal(readback, expected) {
		t.Fatalf("durable fixture allocation readback differs: %v", err)
	}
	permission := decision.Permission()
	copy(permission.Signature[:], ed25519.Sign(signingKey, admission.PermissionTranscript(permission)))
	response, err := admission.EncodePermission(permission)
	if err != nil {
		t.Fatal(err)
	}
	return response
}
