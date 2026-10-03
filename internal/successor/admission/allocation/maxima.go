package allocation

import (
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/admission"
)

// ValidateMaxima owns the aggregate per-role bound, including preflight before
// requesting a signature. Widen before summing caller-controlled counters.
func ValidateMaxima(role admission.AllocationRole, maxima [3]uint32) error {
	total := uint64(maxima[0]) + uint64(maxima[1]) + uint64(maxima[2])
	if total == 0 || total > RoleLimit(role) {
		return errors.New("allocation maxima exceed role allowance")
	}
	return nil
}
