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
		running, err := startRendezvous(plan)
		if err != nil {
			return nil, err
		}
		return &probeServer{Done: running.Done(), Protect: running.Protect, Usage: func() (uint64, uint64, uint64) {
			return rendezvousPressureUsage(running.Usage())
		}, Stop: running.Stop, Drain: running.Drain}, nil
	case "initiator":
		admitter, closeAdmitter, err := openStateEntryAdmitter(config.LocalRoleStateRoot, snapshot,
			func() (dutyFacts, error) { return currentFacts(config) }, config.now)
		if err != nil {
			return nil, err
		}
		plan, err := initiatorDuty(config.Initiator, snapshot, admitter)
		if err != nil {
			return nil, errors.Join(err, closeAdmitter())
		}
		running, err := startInitiator(plan)
		if err != nil {
			return nil, errors.Join(err, closeAdmitter())
		}
		return &probeServer{Done: running.Done(), Protect: running.Protect, Usage: func() (uint64, uint64, uint64) {
			usage := running.Usage()
			return uint64(usage.Handshakes), uint64(usage.Connections), usage.RelayedBytes
		}, Stop: running.Stop, Drain: func(ctx context.Context) error {
			return errors.Join(running.Drain(ctx), closeAdmitter())
		}}, nil
	case "introduction":
		plan, err := introductionDuty(config.Introduction, snapshot, stateTransitGrantAdmitter(config.LocalRoleStateRoot, snapshot,
			func() (dutyFacts, error) { return currentFacts(config) }, config.now))
		if err != nil {
			return nil, err
		}
		running, err := startIntroduction(plan)
		if err != nil {
			return nil, err
		}
		return &probeServer{Done: running.Done(), Protect: running.Protect, Usage: func() (uint64, uint64, uint64) {
			usage := running.Usage()
			return uint64(usage.Handshakes + usage.Deliveries), uint64(usage.Connections), 0
		}, Stop: running.Stop, Drain: running.Drain}, nil
	case "responder":
		plan, err := responderDuty(config.Responder, snapshot, stateTransitGrantAdmitter(config.LocalRoleStateRoot, snapshot,
			func() (dutyFacts, error) { return currentFacts(config) }, config.now))
		if err != nil {
			return nil, err
		}
		running, err := startResponder(plan)
		if err != nil {
			return nil, err
		}
		return &probeServer{Done: running.Done(), Protect: running.Protect, Usage: func() (uint64, uint64, uint64) {
			usage := running.Usage()
			return uint64(usage.Handshakes), uint64(usage.Connections), usage.RelayedBytes
		}, Stop: running.Stop, Drain: running.Drain}, nil
	case "transit-issuance":
		return startTransitIssuer(config, snapshot)
	default:
		return nil, errors.New("native Route assignment is not implemented")
	}
}

func rendezvousPressureUsage(usage RendezvousUsage) (timers, queueItems, queueBytes uint64) {
	// Completed pairs, connections, and relayed bytes are cumulative evidence,
	// not live reservations. The Rendezvous implementation exposes its live
	// bounded work as handshakes and waiting legs; active pairs are protected by
	// the profile's own pair and byte limits.
	return uint64(usage.Handshakes + usage.WaitingLegs), 0, 0
}
