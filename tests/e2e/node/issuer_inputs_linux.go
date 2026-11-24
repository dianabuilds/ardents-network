//go:build linux && node_process_native

package node

import (
	"testing"

	"github.com/dianabuilds/ardents-network/internal/admission/issuance"
	"github.com/dianabuilds/ardents-network/internal/admission/issuer"
	"github.com/dianabuilds/ardents-network/internal/admission/quota"
)

// RequireIssuerRootsReleased reopens the original quota, keys and result store.
func RequireIssuerRootsReleased(t testing.TB, p issuer.Plan) {
	t.Helper()
	ledger, err := quota.Open(p.AdmissionRoot, p.AdmissionBinding)
	if err != nil {
		t.Fatal("joined quota lease unavailable", err)
	}
	defer ledger.Close()
	keys, err := issuance.Open(t.Context(), p.KeyRoot, p.KeyBinding)
	if err != nil {
		t.Fatal("joined original key custody unavailable", err)
	}
	defer keys.Close()
	results, err := issuance.OpenResults(t.Context(), p.ResultRoot, keys, p.AdmissionBinding)
	if err != nil {
		t.Fatal("joined original result lease unavailable", err)
	}
	if err := results.Close(); err != nil {
		t.Fatal(err)
	}
}
