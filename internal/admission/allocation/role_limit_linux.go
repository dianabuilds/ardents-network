//go:build linux

package allocation

import "github.com/dianabuilds/ardents-network/internal/admission"

// RoleLimit bounds the maintained Linux holder's hourly request. Issuance,
// journals and quota decisions belong to new Admission Allocation and Custody.
func RoleLimit(role admission.AllocationRole) uint64 {
	switch role {
	case admission.AllocationUser:
		return 4096
	case admission.AllocationPublisher:
		return 16384
	default:
		return 0
	}
}
