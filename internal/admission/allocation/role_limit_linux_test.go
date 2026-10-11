//go:build linux

package allocation

import (
	"github.com/dianabuilds/ardents-network/internal/admission"
	"testing"
)

func TestRoleLimitsRejectUnknownRoles(t *testing.T) {
	if RoleLimit(admission.AllocationUser) != 4096 || RoleLimit(admission.AllocationPublisher) != 16384 || RoleLimit(0) != 0 || RoleLimit(255) != 0 {
		t.Fatal("role allowance differs from closed admission contract")
	}
}
