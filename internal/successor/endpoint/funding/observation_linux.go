//go:build linux

package funding

import (
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
)

// Available reports whether a live observer was supplied; it grants no authority.
func (a Authority) Available() bool { return a.current != nil }

// Current reobserves the original owner on every call.
func (a Authority) Current() (network.RuntimeView, error) {
	if a.current == nil {
		return network.RuntimeView{}, errors.New("network observation unavailable")
	}
	return a.current()
}

// Witness retains the original Network refusal witness, never a copied approval.
func (a Authority) Witness() (state.RuntimeWitness, error) {
	if a.witness == nil {
		return state.RuntimeWitness{}, errors.New("network witness unavailable")
	}
	return a.witness()
}

// Presentation checks the caller's original presentation guard.
func (a Authority) Presentation(p stock.Presentation) error {
	if a.presentation == nil {
		return errors.New("presentation guard unavailable")
	}
	return a.presentation(p)
}
