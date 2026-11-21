//go:build linux

package allocation_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/allocation"
)

func requestFixture(t *testing.T, role admission.AllocationRole, maxima [3]uint32, hour time.Time) (admission.PermissionRequest, ed25519.PrivateKey, []byte) {
	t.Helper()
	request, holder, err := admission.PreparePermissionRequest([32]byte{1}, [32]byte{2}, [32]byte{3}, 4, role, hour, maxima)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { clear(holder) })
	raw, err := admission.EncodePermissionRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	return request, holder, raw
}

func decideFixture(t *testing.T, raw, journal []byte, hour time.Time) allocation.Decision {
	t.Helper()
	request, err := allocation.Prepare(raw, [32]byte{2}, hour)
	if err != nil {
		t.Fatal(err)
	}
	decision, err := request.Decide(journal, [32]byte{1}, hour)
	if err != nil {
		t.Fatal(err)
	}
	return decision
}

func TestMixedRoleLimitsExactRetryAndChangedRequest(t *testing.T) {
	hour := time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC)
	unsigned, holder, raw := requestFixture(t, admission.AllocationUser, [3]uint32{1000, 3096, 0}, hour)
	first := decideFixture(t, raw, nil, hour)
	journal := first.Journal()
	if first.Repeated() || first.Permission() != unsigned.Permission || allocation.ValidateJournal(journal) != nil {
		t.Fatal("first allocation decision invalid")
	}
	_, _, publisher := requestFixture(t, admission.AllocationPublisher, [3]uint32{0, 0, 16384}, hour)
	second := decideFixture(t, publisher, journal, hour)
	journal = second.Journal()
	// Independently selected maxima exactly fill each role's hourly allowance.
	for _, role := range []admission.AllocationRole{admission.AllocationUser, admission.AllocationPublisher} {
		_, _, extra := requestFixture(t, role, [3]uint32{1, 0, 0}, hour)
		request, err := allocation.Prepare(extra, [32]byte{2}, hour)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := request.Decide(journal, [32]byte{1}, hour); err == nil {
			t.Fatal("exhausted role received another allocation")
		}
	}
	repeated := decideFixture(t, raw, journal, hour)
	if !repeated.Repeated() || repeated.Journal() != nil || repeated.Permission() != first.Permission() {
		t.Fatal("exact retry changed permission or created successor")
	}
	changed := unsigned
	changed.Permission.Maxima = [3]uint32{999, 3097, 0}
	changed, err := admission.SealPermissionRequest(changed, holder)
	if err != nil {
		t.Fatal(err)
	}
	changedRaw, err := admission.EncodePermissionRequest(changed)
	if err != nil {
		t.Fatal(err)
	}
	request, err := allocation.Prepare(changedRaw, [32]byte{2}, hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := request.Decide(journal, [32]byte{1}, hour); err == nil {
		t.Fatal("same ID accepted changed maxima on healthy ledger")
	}
	if !bytes.Equal(journal, second.Journal()) {
		t.Fatal("refusal mutated current journal")
	}
}

func TestRequestAndDecisionDoNotExposeRetainedState(t *testing.T) {
	hour := time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC)
	unsigned, _, raw := requestFixture(t, admission.AllocationUser, [3]uint32{1, 0, 0}, hour)
	request, err := allocation.Prepare(raw, [32]byte{2}, hour)
	if err != nil {
		t.Fatal(err)
	}
	clear(raw)
	decision, err := request.Decide(nil, [32]byte{1}, hour)
	if err != nil {
		t.Fatal(err)
	}
	expected := decision.Journal()
	copyOfJournal := decision.Journal()
	clear(copyOfJournal)
	copyOfPermission := decision.Permission()
	copyOfPermission.Maxima[0] = 4096
	if decision.Permission() != unsigned.Permission || !bytes.Equal(expected, decision.Journal()) {
		t.Fatal("caller modified retained decision")
	}
	var zero allocation.Request
	if _, err := zero.Decide(nil, [32]byte{1}, hour); err == nil {
		t.Fatal("zero request allocated")
	}
	if _, err := request.Decide(nil, [32]byte{9}, hour); err == nil {
		t.Fatal("wrong authority allocated")
	}
	if _, err := request.Decide(nil, [32]byte{1}, hour.Add(time.Hour)); err == nil {
		t.Fatal("expired prepared request allocated after unlock")
	}
}

func TestWindowAdvanceCannotResetPriorQuotaAfterRollback(t *testing.T) {
	hour := time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC)
	_, _, old := requestFixture(t, admission.AllocationUser, [3]uint32{4096, 0, 0}, hour)
	first := decideFixture(t, old, nil, hour)
	_, _, next := requestFixture(t, admission.AllocationUser, [3]uint32{1, 0, 0}, hour.Add(time.Hour))
	second := decideFixture(t, next, first.Journal(), hour.Add(time.Hour))
	request, err := allocation.Prepare(old, [32]byte{2}, hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := request.Decide(second.Journal(), [32]byte{1}, hour); err == nil {
		t.Fatal("clock rollback discarded newer floor and restored quota")
	}
}

func TestPrepareRefusesUnprovenOrForeignRequest(t *testing.T) {
	hour := time.Date(2026, 10, 3, 7, 0, 0, 0, time.UTC)
	_, _, raw := requestFixture(t, admission.AllocationUser, [3]uint32{1, 0, 0}, hour)
	if _, err := allocation.Prepare(raw, [32]byte{8}, hour); err == nil {
		t.Fatal("foreign network accepted")
	}
	if _, err := allocation.Prepare(raw, [32]byte{2}, hour.Add(time.Hour)); err == nil {
		t.Fatal("old request accepted")
	}
	forged := bytes.Clone(raw)
	if _, err := rand.Read(forged[len(forged)-ed25519.SignatureSize:]); err != nil {
		t.Fatal(err)
	}
	if _, err := allocation.Prepare(forged, [32]byte{2}, hour); err == nil {
		t.Fatal("invalid holder proof accepted")
	}
}
