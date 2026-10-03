package allocation

import (
	"bytes"
	"encoding/binary"
	"math"
	"sort"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
)

// MaximumJournalBytes bounds the existing encrypted allocation payload.
const MaximumJournalBytes = 3 << 20

const (
	userLimit        = uint64(4096)
	publisherLimit   = uint64(16384)
	issuerLimit      = uint64(65536)
	journalEntrySize = 77
)

type reservation struct {
	window uint64
	role   admission.AllocationRole
	tokens uint32
	id     [32]byte
	digest [32]byte
}

// ValidateJournal validates the persisted allocation grammar without granting
// permission or claiming that its encrypted envelope/floor is current.
func ValidateJournal(raw []byte) error {
	_, err := decodeJournal(raw)
	return err
}

// RoleLimit is the finite local hourly allocation bound; unknown roles have no
// allowance. It does not grant an allocation or identify a recipient.
func RoleLimit(role admission.AllocationRole) uint64 {
	switch role {
	case admission.AllocationUser:
		return userLimit
	case admission.AllocationPublisher:
		return publisherLimit
	default:
		return 0
	}
}

func tokenTotal(maxima [3]uint32) uint32 {
	var total uint64
	for _, maximum := range maxima {
		total += uint64(maximum)
	}
	if total > math.MaxUint32 {
		return 0
	}
	return uint32(total)
}

func withinBudget(allocations []reservation) bool {
	var user, publisher, issuer uint64
	for _, allocation := range allocations {
		if allocation.tokens == 0 || uint64(allocation.tokens) > issuerLimit || allocation.window == 0 ||
			allocation.role != admission.AllocationUser && allocation.role != admission.AllocationPublisher {
			return false
		}
		issuer += uint64(allocation.tokens)
		if allocation.role == admission.AllocationUser {
			user += uint64(allocation.tokens)
		} else {
			publisher += uint64(allocation.tokens)
		}
	}
	return user <= RoleLimit(admission.AllocationUser) && publisher <= RoleLimit(admission.AllocationPublisher) && issuer <= issuerLimit
}

func forWindow(allocations []reservation, window uint64) []reservation {
	current := make([]reservation, 0, len(allocations))
	for _, allocation := range allocations {
		if allocation.window == window {
			current = append(current, allocation)
		}
	}
	return current
}

// windowRegressed rejects an issuance request behind the
// newest committed hourly reservation. Dropping that reservation on a wall
// clock rollback would make its original hour allocatable again on recovery.
func windowRegressed(allocations []reservation, window uint64) bool {
	for _, allocation := range allocations {
		if allocation.window > window {
			return true
		}
	}
	return false
}

func encodeJournal(allocations []reservation) ([]byte, error) {
	if len(allocations) > int(userLimit+publisherLimit) || !withinBudget(allocations) {
		return nil, errInvalid
	}
	ordered := append([]reservation(nil), allocations...)
	sort.Slice(ordered, func(i, j int) bool { return bytes.Compare(ordered[i].id[:], ordered[j].id[:]) < 0 })
	raw := make([]byte, 0, 12+len(ordered)*journalEntrySize)
	raw = append(raw, "ARDALJ01"...)
	raw = binary.BigEndian.AppendUint32(raw, uint32(len(ordered)))
	for index, allocation := range ordered {
		if index > 0 && bytes.Compare(ordered[index-1].id[:], allocation.id[:]) >= 0 {
			return nil, errInvalid
		}
		raw = binary.BigEndian.AppendUint64(raw, allocation.window)
		raw = append(raw, byte(allocation.role))
		raw = binary.BigEndian.AppendUint32(raw, allocation.tokens)
		raw = append(raw, allocation.id[:]...)
		raw = append(raw, allocation.digest[:]...)
	}
	if len(raw) > MaximumJournalBytes {
		return nil, errInvalid
	}
	return raw, nil
}

func decodeJournal(raw []byte) ([]reservation, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	if len(raw) < 12 || len(raw) > MaximumJournalBytes || string(raw[:8]) != "ARDALJ01" {
		return nil, errInvalid
	}
	count := int(binary.BigEndian.Uint32(raw[8:12]))
	if count > int(userLimit+publisherLimit) || len(raw) != 12+count*journalEntrySize {
		return nil, errInvalid
	}
	allocations := make([]reservation, count)
	offset := 12
	for index := range allocations {
		allocation := &allocations[index]
		allocation.window = binary.BigEndian.Uint64(raw[offset : offset+8])
		offset += 8
		allocation.role = admission.AllocationRole(raw[offset])
		offset++
		allocation.tokens = binary.BigEndian.Uint32(raw[offset : offset+4])
		offset += 4
		copy(allocation.id[:], raw[offset:offset+32])
		offset += 32
		copy(allocation.digest[:], raw[offset:offset+32])
		offset += 32
		if allocation.id == [32]byte{} || allocation.digest == [32]byte{} || (index > 0 && bytes.Compare(allocations[index-1].id[:], allocation.id[:]) >= 0) {
			return nil, errInvalid
		}
	}
	if !withinBudget(allocations) {
		return nil, errInvalid
	}
	return allocations, nil
}
