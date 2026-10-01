//go:build linux

package endpoint

import (
	"github.com/dianabuilds/ardents-network/internal/endpoint/introduction"
)

// introductionOwner is the single owner of the context's Introduction
// subsystem: the admitted Introduction prefix lifecycle, the delivery
// dispatch rendezvous, the bounded exchange set, and the cryptographic
// admission window. The zero value is ready for use under dutyContext.mu.
// The type is named introductionOwner rather than introduction so the root
// can import the extracted mechanism package; the field name and every
// owner.introduction selector are unchanged.
type introductionOwner struct {
	dispatch  introduction.Dispatch
	exchanges introduction.ExchangeSet
	admission introduction.Admission
	prefix    introductionPrefixLifecycle
}
