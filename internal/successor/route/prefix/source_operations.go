package prefix

import (
	"context"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route"
	"github.com/dianabuilds/ardents-network/internal/successor/route/selection"
)

func (h *SourceHandle) ResolveSubmission(node [32]byte, excluded []route.Member) (network.RetainedDuty, error) {
	if err := h.Check(); err != nil {
		return network.RetainedDuty{}, err
	}
	return h.generation.physical.ResolveSubmission(node, excluded)
}

func (h *SourceHandle) Rendezvous(choice *selection.Rendezvous, slot uint8, excluded []route.Member) (network.RetainedDuty, time.Time, error) {
	if err := h.Check(); err != nil {
		return network.RetainedDuty{}, time.Time{}, err
	}
	return h.generation.physical.Rendezvous(choice, slot, excluded)
}

func (h *SourceHandle) CheckRendezvous(duty network.RetainedDuty, excluded []route.Member) error {
	if err := h.Check(); err != nil {
		return err
	}
	return h.generation.physical.CheckRendezvous(duty, excluded)
}

func (h *SourceHandle) OpenSubmissionChannel(ctx context.Context, duty network.RetainedDuty, end time.Time, excluded []route.Member) (*IntroductionChannel, error) {
	if err := h.Check(); err != nil {
		return nil, err
	}
	return h.generation.physical.OpenSubmissionChannel(ctx, duty, end, excluded)
}
