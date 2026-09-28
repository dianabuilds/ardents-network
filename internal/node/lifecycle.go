package node

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node/hosting"
	"github.com/dianabuilds/ardents-network/internal/node/probe"
	"github.com/dianabuilds/ardents-network/internal/resource"
)

// Run owns one Node duty through its bounded terminal-cleanup attempt. A failed
// result reports cleanup that did not complete or could not be proven.
func Run(ctx context.Context, input Config) (result Result, runErr error) {
	config, err := resolveConfig(input)
	if err != nil {
		return Result{}, err
	}
	if err := config.openClosedHosting(); err != nil {
		return Result{}, err
	}
	config.cleanup = &dutyCleanup{host: hosting.NewLifetime(config.host), release: func() error { return releaseLocalDuty(config) }}
	defer func() {
		if !config.cleanup.attempted {
			runErr = errors.Join(runErr, config.cleanup.Close())
		}
	}()
	machine := stateMachine{current: stateAbsent}
	if err := emitState(config, machine, state.NodeDuty{}, "process started"); err != nil {
		return Result{State: stateNames[stateFailed], Reason: err.Error()}, err
	}
	ticker := time.NewTicker(config.PollInterval)
	defer ticker.Stop()
	for {
		snapshot, currentErr := currentFacts(config.Current)
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
			config.cleanup.retained = true
			return runDuty(ctx, config, &machine, snapshot)
		case admissionPrepared:
			if machine.current == stateAbsent {
				if err := retainLocalDuty(config, snapshot, "prepared"); err != nil {
					return fail(config, &machine, nil, "local role state is unavailable", err)
				}
				config.cleanup.retained = true
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

func runDuty(ctx context.Context, config runtimeConfig, machine *stateMachine, snapshot state.NodeDuty) (Result, error) {
	if err := retainLocalDuty(config, snapshot, "quarantined"); err != nil {
		return fail(config, machine, nil, "local role state is unavailable", err)
	}
	config.cleanup.retained = true
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
	current, err := currentFacts(config.Current)
	if err != nil {
		return fail(config, machine, nil, "persistent Network State is unavailable", err)
	}
	currentAdmission := assessAdmission(config, current)
	if currentAdmission.kind != admissionReady {
		reason := "assignment lost readiness during quarantine: " + currentAdmission.reason
		return fail(config, machine, nil, reason, errors.New(reason))
	}
	if !sameDuty(snapshot, current) {
		return fail(config, machine, nil, "assignment changed during quarantine", errors.New("assignment changed during quarantine"))
	}
	server, err := startDuty(config, current)
	if err != nil {
		return fail(config, machine, nil, "Node listener failed", err)
	}
	if err := retainLocalDuty(config, current, "live"); err != nil {
		return fail(config, machine, server, "local role state is unavailable", err)
	}
	config.cleanup.retained = true
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
				terminalErr = errors.New("node listener stopped while node was READY")
			}
			return fail(config, machine, server, "node listener stopped", terminalErr)
		case <-ticker.C:
			pressure, sample, pressureErr := config.resourcePressure(server)
			if pressureErr != nil {
				return fail(config, machine, server, resourcePressureFailureReason(pressureErr), pressureErr)
			}
			now := config.now()
			if !now.Before(nextResourceEvidence) {
				if err := emitResourceDiagnostic(config, current, now, sample); err != nil {
					return fail(config, machine, server, "external evidence channel failed", err)
				}
				nextResourceEvidence = now.Add(time.Second)
			}
			if pressure == pressureDrain {
				return withdrawForResourcePressure(config, machine, server, current)
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
			updated, readErr := currentFacts(config.Current)
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

func withdrawForResourcePressure(config runtimeConfig, machine *stateMachine, server *dutyHandle, snapshot state.NodeDuty) (Result, error) {
	const reason = "resource pressure crossed an emergency threshold"
	if err := emitResourceState(config, snapshot, "DRAIN", reason); err != nil {
		return fail(config, machine, server, "external evidence channel failed", err)
	}
	result, err := withdraw(config, machine, server, snapshot, reason)
	if err != nil {
		return result, err
	}
	return result, emitResourceState(config, snapshot, "EXIT", "resource drain completed")
}

func emitResourceDiagnostic(config runtimeConfig, snapshot state.NodeDuty, at time.Time, sample resource.Sample) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	elapsed := time.Duration(0)
	if !config.measurementOrigin.IsZero() {
		elapsed = time.Since(config.measurementOrigin)
	}
	return config.Emit(ctx, Event{Elapsed: elapsed, Hosting: config.hostingSample, Schema: eventSchema, Kind: "resource-sample", State: "OBSERVED", At: at,
		Epoch: snapshot.Epoch, Generation: snapshot.Generation, Assignment: snapshot.Assignment,
		CarrierProfile: selectedDutyCarrier(snapshot), AssignmentDigest: snapshot.AssignmentDigest, Resource: &sample})
}

func emitResourceState(config runtimeConfig, snapshot state.NodeDuty, state, reason string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return config.Emit(ctx, Event{Schema: eventSchema, Kind: "resource", State: state, At: config.now(),
		Epoch: snapshot.Epoch, Generation: snapshot.Generation, Assignment: snapshot.Assignment,
		CarrierProfile: selectedDutyCarrier(snapshot), AssignmentDigest: snapshot.AssignmentDigest, Reason: reason})
}

func withdraw(config runtimeConfig, machine *stateMachine, server *dutyHandle, snapshot state.NodeDuty, reason string) (Result, error) {
	server.Stop()
	if err := moveAndEmit(config, machine, stateDraining, snapshot, reason); err != nil {
		return fail(config, machine, server, "external evidence channel failed", err)
	}
	if drainErr := server.Drain(context.Background()); drainErr != nil {
		config.cleanup.deferUntil(server.Joined)
		return fail(config, machine, nil, "Node role cleanup failed", drainErr)
	}
	if !dutyJoined(server.Joined) {
		config.cleanup.deferUntil(server.Joined)
		return fail(config, machine, nil, "Node role cleanup failed", errors.New("node role drain returned before joining its workers"))
	}
	if cleanupErr := config.cleanup.Close(); cleanupErr != nil {
		return fail(config, machine, nil, "Node process cleanup failed", cleanupErr)
	}
	if err := moveAndEmit(config, machine, stateWithdrawn, snapshot, reason); err != nil {
		return fail(config, machine, nil, "external evidence channel failed", err)
	}
	return resultFor(machine, snapshot, reason), nil
}

func fail(config runtimeConfig, machine *stateMachine, server *dutyHandle, reason string, cause error) (Result, error) {
	if server != nil {
		server.Stop()
	}
	var terminalErr error
	if moveErr := machine.move(stateFailed); moveErr == nil {
		terminalErr = emitState(config, *machine, state.NodeDuty{}, reason)
	} else {
		terminalErr = moveErr
	}
	if server != nil {
		drainErr := server.Drain(context.Background())
		config.cleanup.deferUntil(server.Joined)
		terminalErr = errors.Join(terminalErr, drainErr)
	}
	return Result{State: stateNames[stateFailed], Reason: reason}, errors.Join(cause, terminalErr)
}

func dutyJoined(joined <-chan struct{}) bool {
	if joined == nil {
		return true
	}
	select {
	case <-joined:
		return true
	default:
		return false
	}
}

func terminalWithoutDuty(config runtimeConfig, machine *stateMachine, snapshot state.NodeDuty, cause error) (Result, error) {
	if machine.current != statePrepared {
		return fail(config, machine, nil, "shutdown before assignment admission", cause)
	}
	if err := machine.move(stateFailed); err != nil {
		return Result{}, err
	}
	eventErr := emitState(config, *machine, snapshot, "shutdown before assignment admission")
	return resultFor(machine, snapshot, "shutdown before assignment admission"), errors.Join(cause, eventErr)
}

func sameDuty(first, second state.NodeDuty) bool {
	return first.Generation == second.Generation && first.NetworkID == second.NetworkID && first.Epoch == second.Epoch &&
		first.Digest == second.Digest && first.NodeID == second.NodeID && first.Assignment == second.Assignment &&
		first.AssignmentDigest == second.AssignmentDigest
}

// currentFacts receives the State-created duty value copy for one poll and
// revalidates its bound before Node retains it. The value type already copies
// every fact; State keeps freshness and conflict classification ownership.
func currentFacts(current func() (state.NodeDuty, error)) (state.NodeDuty, error) {
	duty, err := current()
	if err != nil {
		return state.NodeDuty{}, err
	}
	if duty.CandidateCount > uint8(len(duty.Candidates)) {
		return state.NodeDuty{}, errors.New("node duty candidate count is outside its bound")
	}
	return duty, nil
}

func newProbeDuty(snapshot state.NodeDuty) probe.Duty {
	return probe.Duty{NetworkID: snapshot.NetworkID, EpochDigest: snapshot.Digest, NodeID: snapshot.NodeID,
		AssignmentDigest: snapshot.AssignmentDigest, EpochValidFrom: snapshot.EpochValidFrom,
		EpochValidUntil: snapshot.ValidUntil, RecordValidFrom: snapshot.RecordValidFrom,
		RecordValidUntil: snapshot.RecordValidUntil, Capacity: snapshot.ProbeCapacity}
}

func moveAndEmit(config runtimeConfig, machine *stateMachine, next lifecycleState, snapshot state.NodeDuty, reason string) error {
	if err := machine.move(next); err != nil {
		return err
	}
	return emitState(config, *machine, snapshot, reason)
}

func emitState(config runtimeConfig, machine stateMachine, snapshot state.NodeDuty, reason string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return config.Emit(ctx, Event{Schema: eventSchema, Kind: "lifecycle", State: machine.name(), At: config.now(),
		Epoch: snapshot.Epoch, Generation: snapshot.Generation, Assignment: snapshot.Assignment,
		CarrierProfile: selectedDutyCarrier(snapshot), AssignmentDigest: snapshot.AssignmentDigest, Reason: reason})
}

func resultFor(machine *stateMachine, snapshot state.NodeDuty, reason string) Result {
	return Result{State: machine.name(), Epoch: snapshot.Epoch, Assignment: snapshot.Assignment, CarrierProfile: selectedDutyCarrier(snapshot),
		AssignmentDigest: snapshot.AssignmentDigest, Reason: reason}
}

func selectedDutyCarrier(snapshot state.NodeDuty) string {
	if snapshot.Assignment == "rendezvous" {
		return snapshot.CarrierProfile
	}
	return ""
}
