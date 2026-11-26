package processfixture

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/domain/admission/issuance"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/quota"
)

// RequireIssuerRootsReleased reopens the original quota, keys and result store.
func RequireIssuerRootsReleased(t testing.TB, p issuer.Plan) {
	t.Helper()
	ledger, err := quota.Open(p.AdmissionRoot, p.AdmissionBinding)
	if err != nil {
		t.Fatal("joined quota lease unavailable", err)
	}
	defer func() {
		if err := ledger.Close(); err != nil {
			t.Error("close reopened quota lease", err)
		}
	}()
	keys, err := issuance.Open(t.Context(), p.KeyRoot, p.KeyBinding)
	if err != nil {
		t.Fatal("joined original key custody unavailable", err)
	}
	defer func() {
		if err := keys.Close(); err != nil {
			t.Error("close reopened key custody", err)
		}
	}()
	results, err := issuance.OpenResults(t.Context(), p.ResultRoot, keys, p.AdmissionBinding)
	if err != nil {
		t.Fatal("joined original result lease unavailable", err)
	}
	if err := results.Close(); err != nil {
		t.Fatal(err)
	}
}

// RequireIssuerRootsHeld checks the native quota, key and result leases.
func RequireIssuerRootsHeld(t *testing.T, p issuer.Plan) {
	t.Helper()
	if ledger, err := quota.Open(p.AdmissionRoot, p.AdmissionBinding); !errors.Is(err, quota.ErrBusy) {
		if err == nil {
			if err := ledger.Close(); err != nil {
				t.Error(err)
			}
		}
		t.Fatal("unjoined Issuer lacks actual occupied quota lease", err)
	}
	if keys, err := issuance.Open(t.Context(), p.KeyRoot, p.KeyBinding); !errors.Is(err, issuance.ErrBusy) {
		if err == nil {
			if err := keys.Close(); err != nil {
				t.Error(err)
			}
		}
		t.Fatal("unjoined Issuer lacks actual occupied key custody", err)
	}
	lock, err := os.OpenFile(filepath.Join(p.ResultRoot, "results.lock"), os.O_RDWR, 0600)
	if err != nil {
		t.Fatal("original result lease absent", err)
	}
	defer func() {
		if err := lock.Close(); err != nil {
			t.Error(err)
		}
	}()
	err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if !errors.Is(err, syscall.EWOULDBLOCK) {
		t.Fatal("unjoined Issuer lacks actual occupied result lease", err)
	}
}
