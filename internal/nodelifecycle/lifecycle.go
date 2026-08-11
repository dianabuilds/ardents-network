package nodelifecycle

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/networkstate"
)

// Run owns one Node duty and returns only after terminal cleanup.
func Run(ctx context.Context, input Config) (Result, error) {
	config, err := resolveConfig(input)
	if err != nil {
		return Result{}, err
	}
	machine := stateMachine{current: stateAbsent}
	if err := emitState(config, machine, networkstate.Snapshot{}, "process started"); err != nil {
		return Result{State: stateNames[stateFailed], Reason: err.Error()}, err
	}
	ticker := time.NewTicker(config.PollInterval)
	defer ticker.Stop()
	for {
		snapshot, currentErr := config.Current()
		if currentErr != nil {
			return fail(config, &machine, nil, "persistent Network State is unavailable", currentErr)
		}
		admission := assessAdmission(config, snapshot)
		switch admission.kind {
		case admissionFailed:
			return fail(config, &machine, nil, admission.reason, errors.New(admission.reason))
		case admissionReady:
			return runDuty(ctx, config, &machine, snapshot)
		case admissionPrepared:
			if machine.current == stateAbsent {
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

func runDuty(ctx context.Context, config runtimeConfig, machine *stateMachine, snapshot networkstate.Snapshot) (Result, error) {
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
	current, err := config.Current()
	if err != nil {
		return fail(config, machine, nil, "persistent Network State is unavailable", err)
	}
	if assessAdmission(config, current).kind != admissionReady || !sameDuty(snapshot, current) {
		return fail(config, machine, nil, "assignment changed during quarantine", errors.New("assignment changed during quarantine"))
	}
	server, err := startProbeServer(config, current)
	if err != nil {
		return fail(config, machine, nil, "role-probe listener failed", err)
	}
	if err := moveAndEmit(config, machine, stateReady, current, ""); err != nil {
		return fail(config, machine, server, "external evidence channel failed", err)
	}
	ticker := time.NewTicker(config.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return withdraw(config, machine, server, current, "explicit shutdown")
		case terminalErr := <-server.terminal:
			if terminalErr == nil {
				terminalErr = errors.New("role-probe listener stopped while Node was READY")
			}
			return fail(config, machine, server, "role-probe listener stopped", terminalErr)
		case <-ticker.C:
			updated, readErr := config.Current()
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

func withdraw(config runtimeConfig, machine *stateMachine, server *probeServer, snapshot networkstate.Snapshot, reason string) (Result, error) {
	server.stopAdmission()
	if err := moveAndEmit(config, machine, stateDraining, snapshot, reason); err != nil {
		return fail(config, machine, server, "external evidence channel failed", err)
	}
	server.drain(context.Background())
	if err := moveAndEmit(config, machine, stateWithdrawn, snapshot, reason); err != nil {
		return fail(config, machine, nil, "external evidence channel failed", err)
	}
	return resultFor(machine, snapshot, reason), nil
}

func fail(config runtimeConfig, machine *stateMachine, server *probeServer, reason string, cause error) (Result, error) {
	if server != nil {
		server.stopAdmission()
	}
	if moveErr := machine.move(stateFailed); moveErr == nil {
		_ = emitState(config, *machine, networkstate.Snapshot{}, reason)
	}
	if server != nil {
		server.drain(context.Background())
	}
	return Result{State: stateNames[stateFailed], Reason: reason}, cause
}

func terminalWithoutDuty(config runtimeConfig, machine *stateMachine, snapshot networkstate.Snapshot, cause error) (Result, error) {
	if machine.current != statePrepared {
		return fail(config, machine, nil, "shutdown before assignment admission", cause)
	}
	if err := machine.move(stateFailed); err != nil {
		return Result{}, err
	}
	_ = emitState(config, *machine, snapshot, "shutdown before assignment admission")
	return resultFor(machine, snapshot, "shutdown before assignment admission"), cause
}

func sameDuty(first, second networkstate.Snapshot) bool {
	return first.Digest == second.Digest && first.NodeID == second.NodeID &&
		first.AssignmentDigest == second.AssignmentDigest
}

func moveAndEmit(config runtimeConfig, machine *stateMachine, next lifecycleState, snapshot networkstate.Snapshot, reason string) error {
	if err := machine.move(next); err != nil {
		return err
	}
	return emitState(config, *machine, snapshot, reason)
}

func emitState(config runtimeConfig, machine stateMachine, snapshot networkstate.Snapshot, reason string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return config.Emit(ctx, Event{Schema: eventSchema, Kind: "lifecycle", State: machine.name(), At: config.now(),
		Epoch: snapshot.Epoch, Generation: snapshot.Generation, Assignment: snapshot.Assignment,
		AssignmentDigest: snapshot.AssignmentDigest, Reason: reason})
}

func resultFor(machine *stateMachine, snapshot networkstate.Snapshot, reason string) Result {
	return Result{State: machine.name(), Epoch: snapshot.Epoch, Assignment: snapshot.Assignment,
		AssignmentDigest: snapshot.AssignmentDigest, Reason: reason}
}
