//go:build linux

package stock

import (
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/network/state"
)

// Permission is a comparable observation handle, not the retained mutable
// aggregate. Copying or overwriting this value cannot replace the owner's
// permission, restore a reservation, or revive erased secrets. All observations
// require the owner's shared mutex; retaining a handle grants no authority.
type Permission struct{ value *permission }

func (view Permission) Present() bool { return view.value != nil }

func (view Permission) Grant() admission.Permission { return view.value.Grant() }

func (view Permission) RequestExpiry(digest [32]byte) (time.Time, bool) {
	return view.value.RequestExpiry(digest)
}

func (view Permission) BootstrapAllowance() uint8 { return view.value.BootstrapAllowance() }

func (view Permission) HasAccepted() bool { return view.value.HasAccepted() }

func (view Permission) HasPending() bool { return view.value.HasPending() }

func (view Permission) CurrentFor(profile state.ClosedProfileView, now time.Time) bool {
	return view.value.CurrentFor(profile, now)
}

func (view Permission) StockCountFor(digest, receiver [32]byte, class uint8) int {
	return view.value.StockCountFor(digest, receiver, class)
}

func (view Permission) StockCountForDuty(digest, receiver [32]byte, duty uint64, class uint8) int {
	return view.value.StockCountForDuty(digest, receiver, duty, class)
}

func (view Permission) MissingStockFor(digest [32]byte, receivers [][32]byte, class uint8) [][32]byte {
	return view.value.MissingStockFor(digest, receivers, class)
}

func (view Permission) Remaining(class uint8) uint32 { return view.value.Remaining(class) }
