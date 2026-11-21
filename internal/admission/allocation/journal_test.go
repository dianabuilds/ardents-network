package allocation

import (
	"github.com/dianabuilds/ardents-network/internal/admission"
	"testing"
)

func TestAdmissionJournalAcceptsCompleteHourlyReservationSet(t *testing.T) {
	allocations := make([]reservation, 0, userLimit+publisherLimit)
	for index := uint64(0); index < userLimit+publisherLimit; index++ {
		role := admission.AllocationUser
		if index >= userLimit {
			role = admission.AllocationPublisher
		}
		allocation := reservation{window: 1, role: role, tokens: 1}
		value := index + 1
		allocation.id[28] = byte(value >> 24)
		allocation.id[29] = byte(value >> 16)
		allocation.id[30] = byte(value >> 8)
		allocation.id[31] = byte(value)
		allocation.digest[0] = 1
		allocation.digest[28] = allocation.id[28]
		allocation.digest[29] = allocation.id[29]
		allocation.digest[30] = allocation.id[30]
		allocation.digest[31] = allocation.id[31]
		allocations = append(allocations, allocation)
	}
	raw, err := encodeJournal(allocations)
	if err != nil {
		t.Fatalf("encode complete reservation set: %v", err)
	}
	decoded, err := decodeJournal(raw)
	if err != nil || len(decoded) != len(allocations) {
		t.Fatalf("decode complete reservation set = %d / %v", len(decoded), err)
	}
}

func TestAdmissionAllocationsForWindowExpiresPreviousReservations(t *testing.T) {
	allocations := []reservation{
		{window: 100, role: admission.AllocationUser, tokens: 1, id: [32]byte{1}, digest: [32]byte{1}},
		{window: 101, role: admission.AllocationPublisher, tokens: 1, id: [32]byte{2}, digest: [32]byte{2}},
	}
	current := forWindow(allocations, 101)
	if len(current) != 1 || current[0].window != 101 || current[0].role != admission.AllocationPublisher {
		t.Fatalf("current hourly allocations = %#v", current)
	}
}

func TestRoleLimitsRejectUnknownRoles(t *testing.T) {
	if RoleLimit(admission.AllocationUser) != 4096 || RoleLimit(admission.AllocationPublisher) != 16384 || RoleLimit(0) != 0 || RoleLimit(255) != 0 {
		t.Fatal("role allowance differs from closed admission contract")
	}
}
