//go:build linux && (document_worker_installed || endpoint_launch_native)

package endpoint

import (
	"bytes"
	"crypto/ed25519"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
)

func TestFixturePermissionApprovalRetainsExactJournalOnRetry(t *testing.T) {
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{17}, ed25519.SeedSize))
	defer clear(key)
	public := key.Public().(ed25519.PublicKey)
	network, issuer := [32]byte{1}, [32]byte{2}
	facts := admission.AuthorityFacts{NetworkID: network, IssuanceAuthorityKey: [32]byte(public)}
	hour := time.Date(2030, 1, 2, 3, 0, 0, 0, time.UTC)
	now := hour.Add(time.Minute)
	request, holder, err := admission.PreparePermissionRequest(facts.IssuanceAuthorityKey, network, issuer, 4, admission.AllocationUser, hour, [3]uint32{0, 1, 0})
	if err != nil {
		t.Fatal(err)
	}
	defer clear(holder)
	raw, err := admission.EncodePermissionRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	journal := filepath.Join(t.TempDir(), "allocation")
	response := ApproveFixturePermission(t, journal, raw, facts, key, now, true)
	before, err := os.Stat(journal)
	if err != nil {
		t.Fatal(err)
	}
	journalBytes, err := os.ReadFile(journal)
	if err != nil || len(journalBytes) == 0 || before.Mode().Perm() != 0600 {
		t.Fatalf("original allocation journal unavailable or unprotected: %v", err)
	}
	signed, err := admission.DecodePermission(response)
	if err != nil {
		t.Fatal(err)
	}
	if err := admission.VerifyPermission(signed, public, network, issuer, 4, now); err != nil {
		t.Fatal(err)
	}
	retry := ApproveFixturePermission(t, journal, raw, facts, key, now, true)
	after, err := os.Stat(journal)
	if err != nil || !os.SameFile(before, after) || !bytes.Equal(response, retry) {
		t.Fatalf("exact retry replaced the original journal or response: %v", err)
	}
	afterBytes, err := os.ReadFile(journal)
	if err != nil || !bytes.Equal(afterBytes, journalBytes) {
		t.Fatalf("exact retry changed journal bytes: %v", err)
	}
	fresh := ApproveFixturePermission(t, filepath.Join(t.TempDir(), "allocation"), raw, facts, key, now, false)
	if !bytes.Equal(fresh, response) {
		t.Fatal("fresh preview does not preserve the actual signed decision")
	}
}
