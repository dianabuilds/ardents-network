package endpoint

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/admission/stock"
	"github.com/dianabuilds/ardents-network/internal/endpoint/funding"
	"github.com/dianabuilds/ardents-network/internal/network"
)

func presentationGuard(current func() (network.RuntimeView, error)) func(stock.Presentation) error {
	return func(p stock.Presentation) error {
		view, err := current()
		if err != nil {
			return err
		}
		facts, now, err := funding.NetworkFacts(view)
		if err != nil {
			return err
		}
		member, err := view.Member(p.RecipientNodeID, now)
		if err != nil || member.DutyGeneration != p.RecipientDutyGeneration || facts.NetworkID != p.NetworkID || facts.StateGeneration != p.StateGeneration || facts.StateDigest != p.StateDigest || facts.Digest != p.ProfileDigest || !now.Before(p.Deadline) || p.Deadline.After(member.NotAfter()) || p.Deadline.After(facts.NotAfter) {
			return errors.Join(errors.New("network presentation binding is unavailable"), err)
		}
		return nil
	}
}
