//go:build linux

package funding

import (
	"errors"
	"github.com/dianabuilds/ardents-network/internal/network/state"
)

// Witness retains the original Network refusal witness, never a copied approval.
func (a Authority) Witness() (state.RuntimeWitness, error) {
	if a.witness == nil {
		return state.RuntimeWitness{}, errors.New("network witness unavailable")
	}
	return a.witness()
}

// HasPresentation reports configuration only; it supplies no accepting fact.
func (a Authority) HasPresentation() bool { return a.presentation != nil }
