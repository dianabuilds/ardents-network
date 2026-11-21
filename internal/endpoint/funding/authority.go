package funding

import (
	"github.com/dianabuilds/ardents-network/internal/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/network"
	"github.com/dianabuilds/ardents-network/internal/network/state"
)

// Authority borrows the original live Network observation and the caller's
// presentation guard. It owns no State lease and grants no cached authority.
type Authority struct {
	current      func() (network.RuntimeView, error)
	witness      func() (state.RuntimeWitness, error)
	presentation func(stock.Presentation) error
}

// Observe binds existing live owner operations. The caller retains and joins
// that owner after every funding/Route borrower; no Close is transferred here.
func Observe(current func() (network.RuntimeView, error), witness func() (state.RuntimeWitness, error), presentation func(stock.Presentation) error) Authority {
	return Authority{current: current, witness: witness, presentation: presentation}
}
