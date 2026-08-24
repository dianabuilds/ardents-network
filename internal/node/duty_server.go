package node

import (
	"context"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func startDuty(config runtimeConfig, snapshot dutyFacts) (*probeServer, error) {
	if snapshot.Profile != route.Profile {
		return config.probe.startProbe(newProbeDuty(snapshot))
	}
	switch snapshot.Assignment {
	case "rendezvous":
		plan, err := rendezvousDuty(config.Rendezvous, snapshot)
		if err != nil {
			return nil, err
		}
		running, err := StartRendezvous(plan)
		if err != nil {
			return nil, err
		}
		return &probeServer{Done: running.Done(), Protect: running.Protect, Usage: func() (uint64, uint64, uint64) {
			usage := running.Usage()
			return uint64(usage.Handshakes + usage.WaitingLegs), uint64(usage.Connections), usage.RelayedBytes
		}, Stop: running.Stop, Drain: func(ctx context.Context) { _ = running.Drain(ctx) }}, nil
	case "initiator":
		plan, err := initiatorDuty(config.Initiator, snapshot)
		if err != nil {
			return nil, err
		}
		running, err := StartInitiator(plan)
		if err != nil {
			return nil, err
		}
		return &probeServer{Done: running.Done(), Protect: running.Protect, Usage: func() (uint64, uint64, uint64) {
			usage := running.Usage()
			return uint64(usage.Handshakes), uint64(usage.Connections), usage.RelayedBytes
		}, Stop: running.Stop, Drain: func(ctx context.Context) { _ = running.Drain(ctx) }}, nil
	case "introduction":
		plan, err := introductionDuty(config.Introduction, snapshot)
		if err != nil {
			return nil, err
		}
		running, err := StartIntroduction(plan)
		if err != nil {
			return nil, err
		}
		return &probeServer{Done: running.Done(), Protect: running.Protect, Usage: func() (uint64, uint64, uint64) {
			usage := running.Usage()
			return uint64(usage.Handshakes + usage.Deliveries), uint64(usage.Connections), 0
		}, Stop: running.Stop, Drain: func(ctx context.Context) { _ = running.Drain(ctx) }}, nil
	default:
		return nil, errors.New("native Route assignment is not implemented")
	}
}
