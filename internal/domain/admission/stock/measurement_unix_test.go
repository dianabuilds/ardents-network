//go:build linux || darwin

package stock

import (
	"testing"

	"github.com/dianabuilds/ardents-network/internal/diagnostics/measurement"
	"github.com/dianabuilds/ardents-network/internal/diagnostics/runtimeevent"
)

func TestMeasuredTokenStockFollowsVerifiedImportBurnAndErasure(t *testing.T) {
	capture, err := measurement.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer capture.Stop()
	owner, _, presentation := issuedStockFixture(t)
	defer owner.Close()
	tokens := func() uint64 {
		t.Helper()
		capture.Sample()
		spec, _ := runtimeevent.AdmissionStockTokens.Spec()
		for _, v := range capture.Latest().Values {
			if v.Name == spec.Name {
				if !v.Available {
					t.Fatal("invalid token measurement")
				}
				return v.Current
			}
		}
		t.Fatal("stock instrument absent")
		return 0
	}
	before := tokens()
	if before == 0 {
		t.Fatal("verified fixture batch was not counted")
	}
	raw, err := owner.Take(t.Context(), presentation, 2)
	defer clear(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got := tokens(); got != before-1 {
		t.Fatal("burn did not reduce measured stock", got, before)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if got := tokens(); got != 0 {
		t.Fatal("erased stock remains available", got)
	}

	// The first permission may have spent its entire batch. Close another
	// genuinely issued permission with unspent tokens to exercise erasure.
	unspent, _, _ := issuedStockFixture(t)
	defer unspent.Close()
	if tokens() == 0 {
		t.Fatal("unspent fixture batch was not counted")
	}
	if err := unspent.Close(); err != nil {
		t.Fatal(err)
	}
	if got := tokens(); got != 0 {
		t.Fatal("unspent stock was not retired exactly once", got)
	}
}
