package funding

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/token"
	"github.com/dianabuilds/ardents-network/internal/route/prefix"
	"github.com/dianabuilds/ardents-network/internal/route/selection"
)

// IssuerPreparation binds the issuance request to its original owners.
// Stock owns permission, request/retry identity and verified token deposit.
// Route supplies the actual mode, exact opening binding and final local guard.
func IssuerPreparation(ctx context.Context, authority Authority, holder *stock.Owner, leg selection.Leg, class uint8, receivers [][32]byte, preparation ...*stock.Preparation) func(prefix.IssuerBinding) (prefix.IssuerBatch, error) {
	return func(binding prefix.IssuerBinding) (prefix.IssuerBatch, error) {
		if class < 1 || class > 3 || ctx.Err() != nil {
			return prefix.IssuerBatch{}, errors.New("route issuance intent invalid")
		}
		view, err := authority.current()
		if err != nil || view.ProfileBinding().Digest != binding.ProfileDigest {
			return prefix.IssuerBatch{}, errors.Join(errors.New("route issuance profile differs"), err)
		}
		if len(receivers) == 0 {
			switch class {
			case 1:
				receivers = [][32]byte{view.IssuerNodeID()}
			case 2:
				receivers = [][32]byte{leg.Entry.NodeID, leg.Interior.NodeID}
			default:
				return prefix.IssuerBatch{}, errors.New("route issuance receivers absent")
			}
		}
		var challenges []token.ClosedTokenContext
		for _, id := range receivers {
			member, err := view.Member(id, view.ObservedAt())
			if err != nil {
				return prefix.IssuerBatch{}, err
			}
			challenges = append(challenges, token.ClosedTokenContext{NetworkID: view.ProfileBinding().Network, ProfileDigest: binding.ProfileDigest, IssuerNodeID: view.IssuerNodeID(), ReceiverNodeID: id, ReceiverDutyGeneration: member.DutyGeneration, Class: class, WindowStart: view.ObservedAt().UTC().Truncate(time.Hour)})
		}
		intent := stock.IssuanceIntent{Challenges: challenges, Selection: stock.ExchangeBinding{ID: binding.ID, ProfileDigest: binding.ProfileDigest}, Bootstrap: binding.Bootstrap, Deadline: binding.Deadline}
		// Only pure draft construction separates this view from validation.
		// Stock.Begin and Request independently reobserve before their effects;
		// the original Route also retains its own I/O and final live gates.
		if err := ValidateIntent(view, intent); err != nil {
			return prefix.IssuerBatch{}, err
		}
		if len(preparation) > 1 {
			return prefix.IssuerBatch{}, errors.New("stock preparation ambiguous")
		}
		var attempt stock.Attempt
		if len(preparation) == 1 {
			attempt, err = preparation[0].Begin(intent)
		} else {
			attempt, err = holder.Begin(intent)
		}
		if err != nil {
			return prefix.IssuerBatch{}, err
		}
		raw, end, err := attempt.Request()
		if err != nil {
			return prefix.IssuerBatch{}, errors.Join(err, attempt.Complete(nil, err))
		}
		return prefix.IssuerBatch{Request: raw, Deadline: end, Complete: attempt.CompleteBound}, nil
	}
}
