package node

import (
	"context"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func startDuty(config runtimeConfig, snapshot dutyFacts) (*probeServer, error) {
	if snapshot.Profile != route.Profile {
		return config.probe.startProbe(newProbeDuty(snapshot))
	}
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
}
