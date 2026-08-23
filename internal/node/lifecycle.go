package node

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/resource"
)

// Run owns one Node duty and returns only after terminal cleanup.
func Run(ctx context.Context, input Config) (result Result, runErr error) {
	config, err := resolveConfig(input)
	if err != nil {
		return Result{}, err
	}
	machine := stateMachine{current: stateAbsent}
	retained := false
	defer func() {
		if retained {
			runErr = errors.Join(runErr, releaseLocalDuty(config))
		}
	}()
	if err := emitState(config, machine, Facts{}, "process started"); err != nil {
		return Result{State: stateNames[stateFailed], Reason: err.Error()}, err
	}
	ticker := time.NewTicker(config.PollInterval)
	defer ticker.Stop()
	for {
		snapshot, currentErr := currentFacts(config)
		if currentErr != nil {
			return fail(config, &machine, nil, "persistent Network State is unavailable", currentErr)
		}
		admission := assessAdmission(config, snapshot)
		switch admission.kind {
		case admissionFailed:
			return fail(config, &machine, nil, admission.reason, errors.New(admission.reason))
		case admissionReady:
			if err := retainLocalDuty(config, snapshot, "prepared"); err != nil {
				return fail(config, &machine, nil, "local role state is unavailable", err)
			}
			retained = true
			return runDuty(ctx, config, &machine, snapshot)
		case admissionPrepared:
			if machine.current == stateAbsent {
				if err := retainLocalDuty(config, snapshot, "prepared"); err != nil {
					return fail(config, &machine, nil, "local role state is unavailable", err)
				}
				retained = true
				if err := moveAndEmit(config, &machine, statePrepared, snapshot, admission.reason); err != nil {
					return fail(config, &machine, nil, "external evidence channel failed", err)
				}
			}
		case admissionAbsent:
			if machine.current == statePrepared {
				return fail(config, &machine, nil, admission.reason, errors.New(admission.reason))
			}
		}
		select {
		case <-ctx.Done():
			return terminalWithoutDuty(config, &machine, snapshot, ctx.Err())
		case <-ticker.C:
		}
	}
}

func runDuty(ctx context.Context, config runtimeConfig, machine *stateMachine, snapshot Facts) (Result, error) {
	if err := retainLocalDuty(config, snapshot, "quarantined"); err != nil {
		return fail(config, machine, nil, "local role state is unavailable", err)
	}
	if machine.current == stateAbsent {
		if err := moveAndEmit(config, machine, statePrepared, snapshot, "verified assignment is quarantined"); err != nil {
			return fail(config, machine, nil, "external evidence channel failed", err)
		}
	}
	if config.Quarantine > 0 {
		timer := time.NewTimer(config.Quarantine)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return terminalWithoutDuty(config, machine, snapshot, ctx.Err())
		case <-timer.C:
		}
	}
	current, err := currentFacts(config)
	if err != nil {
		return fail(config, machine, nil, "persistent Network State is unavailable", err)
	}
	if assessAdmission(config, current).kind != admissionReady || !sameDuty(snapshot, current) {
		return fail(config, machine, nil, "assignment changed during quarantine", errors.New("assignment changed during quarantine"))
	}
	server, err := config.probe.startProbe(newProbeDuty(current))
	if err != nil {
		return fail(config, machine, nil, "role-probe listener failed", err)
	}
	if err := retainLocalDuty(config, current, "live"); err != nil {
		return fail(config, machine, server, "local role state is unavailable", err)
	}
	if err := moveAndEmit(config, machine, stateReady, current, ""); err != nil {
		return fail(config, machine, server, "external evidence channel failed", err)
	}
	ticker := time.NewTicker(config.PollInterval)
	defer ticker.Stop()
	protected := false
	nextResourceEvidence := time.Time{}
	for {
		select {
		case <-ctx.Done():
			return withdraw(config, machine, server, current, "explicit shutdown")
		case terminalErr := <-server.Done:
			if terminalErr == nil {
				terminalErr = errors.New("role-probe listener stopped while Node was READY")
			}
			return fail(config, machine, server, "role-probe listener stopped", terminalErr)
		case <-ticker.C:
			pressure, sample, pressureErr := config.resourcePressure(server)
			if pressureErr != nil {
				return fail(config, machine, server, "resource pressure evidence is unavailable", pressureErr)
			}
			now := config.now()
			if !now.Before(nextResourceEvidence) {
				if err := emitResourceDiagnostic(config, current, now, sample); err != nil {
					return fail(config, machine, server, "external evidence channel failed", err)
				}
				nextResourceEvidence = now.Add(time.Second)
			}
			if pressure == pressureDrain {
				if err := emitResourceState(config, current, "DRAIN", "resource pressure crossed an emergency threshold"); err != nil {
					return fail(config, machine, server, "external evidence channel failed", err)
				}
				result, err := withdraw(config, machine, server, current, "resource pressure crossed an emergency threshold")
				if exitErr := emitResourceState(config, current, "EXIT", "resource drain completed"); exitErr != nil {
					return result, errors.Join(err, exitErr)
				}
				return result, err
			}
			if pressure == pressureProtect && !protected {
				server.Protect(true)
				protected = true
				if err := emitResourceState(config, current, "PROTECT", "resource pressure crossed the fixed profile"); err != nil {
					return fail(config, machine, server, "external evidence channel failed", err)
				}
			} else if pressure == pressureNormal && protected {
				server.Protect(false)
				protected = false
				if err := emitResourceState(config, current, "NORMAL", "resource pressure recovered"); err != nil {
					return fail(config, machine, server, "external evidence channel failed", err)
				}
			}
			updated, readErr := currentFacts(config)
			if readErr != nil {
				return fail(config, machine, server, "persistent Network State is unavailable", readErr)
			}
			admission := assessAdmission(config, updated)
			if admission.kind == admissionFailed {
				return fail(config, machine, server, admission.reason, errors.New(admission.reason))
			}
			if admission.kind != admissionReady || !sameDuty(current, updated) {
				return withdraw(config, machine, server, current, admission.reason)
			}
		}
	}
}

func emitResourceDiagnostic(config runtimeConfig, snapshot Facts, at time.Time, sample resource.Sample) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return config.Emit(ctx, Event{Schema: eventSchema, Kind: "resource-sample", State: "OBSERVED", At: at,
		Epoch: snapshot.Epoch, Generation: snapshot.Generation, Assignment: snapshot.Assignment,
		AssignmentDigest: snapshot.AssignmentDigest, Resource: &sample})
}

func emitResourceState(config runtimeConfig, snapshot Facts, state, reason string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return config.Emit(ctx, Event{Schema: eventSchema, Kind: "resource", State: state, At: config.now(),
		Epoch: snapshot.Epoch, Generation: snapshot.Generation, Assignment: snapshot.Assignment,
		AssignmentDigest: snapshot.AssignmentDigest, Reason: reason})
}

func withdraw(config runtimeConfig, machine *stateMachine, server *probeServer, snapshot Facts, reason string) (Result, error) {
	server.Stop()
	if err := moveAndEmit(config, machine, stateDraining, snapshot, reason); err != nil {
		return fail(config, machine, server, "external evidence channel failed", err)
	}
	server.Drain(context.Background())
	if err := moveAndEmit(config, machine, stateWithdrawn, snapshot, reason); err != nil {
		return fail(config, machine, nil, "external evidence channel failed", err)
	}
	return resultFor(machine, snapshot, reason), nil
}

func fail(config runtimeConfig, machine *stateMachine, server *probeServer, reason string, cause error) (Result, error) {
	if server != nil {
		server.Stop()
	}
	if moveErr := machine.move(stateFailed); moveErr == nil {
		_ = emitState(config, *machine, Facts{}, reason)
	}
	if server != nil {
		server.Drain(context.Background())
	}
	return Result{State: stateNames[stateFailed], Reason: reason}, cause
}

func terminalWithoutDuty(config runtimeConfig, machine *stateMachine, snapshot Facts, cause error) (Result, error) {
	if machine.current != statePrepared {
		return fail(config, machine, nil, "shutdown before assignment admission", cause)
	}
	if err := machine.move(stateFailed); err != nil {
		return Result{}, err
	}
	_ = emitState(config, *machine, snapshot, "shutdown before assignment admission")
	return resultFor(machine, snapshot, "shutdown before assignment admission"), cause
}

func sameDuty(first, second Facts) bool {
	return first.Digest == second.Digest && first.NodeID == second.NodeID &&
		first.AssignmentDigest == second.AssignmentDigest
}

func currentFacts(config runtimeConfig) (Facts, error) {
	view, err := config.Current()
	if err != nil {
		return Facts{}, err
	}
	if view == nil {
		return Facts{}, errors.New("Node duty view is unavailable")
	}
	return Facts{Generation: view.DutyGeneration(), NetworkID: view.DutyNetworkID(), Epoch: view.DutyEpoch(),
		Digest: view.DutyDigest(), EpochValidFrom: view.DutyEpochValidFrom(), ValidUntil: view.DutyValidUntil(),
		Profile: view.DutyProfile(), Fresh: view.DutyFresh(), Conflicting: view.DutyConflicting(),
		RecordPresent: view.DutyRecordPresent(), NodeID: view.DutyNodeID(), NodePublicKey: view.DutyNodePublicKey(),
		RecordValidFrom: view.DutyRecordValidFrom(), RecordValidUntil: view.DutyRecordValidUntil(),
		DeclaredFamily: view.DutyDeclaredFamily(), ProbeEndpoint: view.DutyProbeEndpoint(),
		ProbeCapacity: view.DutyProbeCapacity(), Assignment: view.DutyAssignment(),
		AssignmentDigest: view.DutyAssignmentDigest()}, nil
}

func newProbeDuty(snapshot Facts) probeDuty {
	return probeDuty{NetworkID: snapshot.NetworkID, EpochDigest: snapshot.Digest, NodeID: snapshot.NodeID,
		AssignmentDigest: snapshot.AssignmentDigest, EpochValidFrom: snapshot.EpochValidFrom,
		EpochValidUntil: snapshot.ValidUntil, RecordValidFrom: snapshot.RecordValidFrom,
		RecordValidUntil: snapshot.RecordValidUntil, Capacity: snapshot.ProbeCapacity}
}

func moveAndEmit(config runtimeConfig, machine *stateMachine, next lifecycleState, snapshot Facts, reason string) error {
	if err := machine.move(next); err != nil {
		return err
	}
	return emitState(config, *machine, snapshot, reason)
}

func emitState(config runtimeConfig, machine stateMachine, snapshot Facts, reason string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return config.Emit(ctx, Event{Schema: eventSchema, Kind: "lifecycle", State: machine.name(), At: config.now(),
		Epoch: snapshot.Epoch, Generation: snapshot.Generation, Assignment: snapshot.Assignment,
		AssignmentDigest: snapshot.AssignmentDigest, Reason: reason})
}

func resultFor(machine *stateMachine, snapshot Facts, reason string) Result {
	return Result{State: machine.name(), Epoch: snapshot.Epoch, Assignment: snapshot.Assignment,
		AssignmentDigest: snapshot.AssignmentDigest, Reason: reason}
}
