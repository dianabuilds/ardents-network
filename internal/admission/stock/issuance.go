//go:build linux

package stock

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/route/client"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

// IssuanceIntent contains the receiving duties and Source selected by the
// authorized Endpoint operation. It cannot grant a permission or install stock.
type IssuanceIntent struct {
	Challenges      []credential.ClosedTokenContext
	Selection       client.ClosedBootstrapSelection
	Prefix          Prefix
	Refill          bool
	Joined          bool
	ExpectedPrefix  Prefix
	DiscardCanceled bool
}

// BeginIssuanceLocked owns the entire permission-to-operation transition under
// the shared context mutex. A busy or stale owner refuses before any new debit.
// The caller runs the returned operation after releasing that mutex.
func (owner *Owner) BeginIssuanceLocked(intent IssuanceIntent) (Operation, error) {
	profile, now, err := owner.host.ProfileLocked()
	if err != nil || owner.issuance != nil || !owner.permission.CurrentFor(profile, now) {
		return Operation{}, errors.New("text issuance owner is unavailable")
	}
	selection, err := owner.host.SelectBootstrapLocked()
	if err != nil || selection != intent.Selection || selection.ProfileDigest != profile.Digest {
		return Operation{}, errors.New("text issuance source selection unavailable")
	}
	batch, err := owner.permission.reserveBatch(profile, now, intent.Challenges, selection,
		intent.Refill, intent.Prefix, intent.Joined, intent.ExpectedPrefix)
	if err != nil {
		return Operation{}, err
	}
	operation := newOperation(owner, owner.permission, profile, batch, intent.DiscardCanceled)
	owner.issuance = operation
	return Operation{value: operation}, nil
}
