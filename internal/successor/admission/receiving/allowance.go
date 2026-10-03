package receiving

import (
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
)

// Allowance is the finite result of redemption. Replenishment replaces the
// remaining byte allowance and never changes the original terminal time.
type Allowance struct {
	class    admission.Class
	deadline time.Time
	bytes    uint64
}

func NewAllowance(class admission.Class, now, operationEnd, dutyEnd time.Time) (Allowance, error) {
	if class.Lifetime() == 0 || now.IsZero() || operationEnd.IsZero() || dutyEnd.IsZero() {
		return Allowance{}, errors.New("invalid admission allowance")
	}
	end := now.Add(class.Lifetime())
	if operationEnd.Before(end) {
		end = operationEnd
	}
	if dutyEnd.Before(end) {
		end = dutyEnd
	}
	if !now.Before(end) {
		return Allowance{}, errors.New("admission allowance expired")
	}
	return Allowance{class: class, deadline: end, bytes: class.ByteLimit()}, nil
}

func (a Allowance) Deadline() time.Time { return a.deadline }
func (a Allowance) Bytes() uint64       { return a.bytes }
func (a Allowance) Replenish(now time.Time, remaining uint64) (Allowance, error) {
	if a.class != admission.ForwardClass || now.IsZero() || !now.Before(a.deadline) || remaining == 0 || remaining > a.bytes {
		return Allowance{}, errors.New("admission replenishment unavailable")
	}
	return Allowance{class: a.class, deadline: a.deadline, bytes: a.class.ByteLimit()}, nil
}
