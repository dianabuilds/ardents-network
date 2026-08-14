package recoverysmoke

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type processSelector struct {
	LogicalRole, AdapterKey string
}

type processRef struct {
	Adapter, Identity, Incarnation string
	Scope, Executable, Tree        [32]byte
}

type processObservation struct {
	Ref               processRef
	AdapterProjection []byte
	OSProcessID       uint32
	Running           bool
	ObservedAtNanos   int64
}

type processState string

const processStopped processState = "stopped"

type processFaultKind string

const processStop processFaultKind = "stop"

type processFaultSpec struct {
	Kind processFaultKind
}

type processFaultReceipt struct {
	Ref                                              processEvidenceRef
	Kind                                             processFaultKind
	State                                            processState
	InvocationStartedNanos, InvocationCompletedNanos int64
	ObservedAtNanos                                  int64
}

type processStateObservation struct {
	Ref             processEvidenceRef
	State           processState
	ObservedAtNanos int64
}

type hostProcessAdapter interface {
	ResolveProcess(context.Context, processSelector) (processObservation, error)
	InjectProcessFault(context.Context, processEvidenceRef, processFaultSpec) (processFaultReceipt, error)
	AwaitProcessState(context.Context, processEvidenceRef, processState, time.Duration) (processStateObservation, error)
}

func observeRouteGeneration(ctx context.Context, observer hostProcessAdapter, fixture prepared,
	generation uint64, selection selectedRoute) (routeGeneration, error) {
	result := routeGeneration{Generation: generation,
		Processes: make(map[string]candidateProcess, len(replacementRoles))}
	for _, roleName := range replacementRoles {
		service, err := candidateService(fixture.candidates, selection[roleName])
		if err != nil {
			return routeGeneration{}, err
		}
		observed, err := observer.ResolveProcess(ctx,
			processSelector{LogicalRole: roleName, AdapterKey: service})
		if err != nil {
			return routeGeneration{}, fmt.Errorf("resolve %s Route process: %w", roleName, err)
		}
		if err := validateProcessObservation(observed); err != nil {
			return routeGeneration{}, fmt.Errorf("resolve %s Route process: %w", roleName, err)
		}
		candidate := selection[roleName]
		host := bindProcessRef(observed.Ref)
		process := candidateProcess{Service: service,
			ContainerID: observed.Ref.Identity, Incarnation: observed.Ref.Incarnation,
			PID: observed.OSProcessID, ObservedAtNanos: observed.ObservedAtNanos,
			NodeID: candidate.NodeID, PublicKey: candidate.PublicKey, Host: host,
			AdapterProjection: string(observed.AdapterProjection)}
		process.HostObservation = processObservationCommitment(host, []byte(process.AdapterProjection),
			process.PID, true, process.ObservedAtNanos)
		result.Processes[roleName] = process
	}
	return result, nil
}

func stopCandidate(ctx context.Context, observer hostProcessAdapter,
	process candidateProcess) (processFaultEvidence, error) {
	receipt, err := observer.InjectProcessFault(ctx, process.Host, processFaultSpec{Kind: processStop})
	if err != nil {
		return processFaultEvidence{}, fmt.Errorf("stop exact Route candidate process: %w", err)
	}
	if receipt.Ref != process.Host || receipt.Kind != processStop || receipt.State != processStopped ||
		receipt.InvocationStartedNanos <= 0 || receipt.InvocationCompletedNanos < receipt.InvocationStartedNanos ||
		process.ObservedAtNanos <= 0 || receipt.InvocationStartedNanos < process.ObservedAtNanos ||
		receipt.ObservedAtNanos < receipt.InvocationStartedNanos ||
		receipt.ObservedAtNanos > receipt.InvocationCompletedNanos {
		return processFaultEvidence{}, errors.New("route candidate process fault receipt is inconsistent")
	}
	return freezeProcessFault(receipt), nil
}

func candidateUnavailable(ctx context.Context, observer hostProcessAdapter,
	process candidateProcess, hostStartedAt int64) (failedResourceReceipt, error) {
	observed, err := observer.AwaitProcessState(ctx, process.Host, processStopped, 10*time.Second)
	if err != nil {
		return failedResourceReceipt{}, fmt.Errorf("observe failed Route candidate state: %w", err)
	}
	if hostStartedAt <= 0 || observed.Ref != process.Host || observed.State != processStopped ||
		observed.ObservedAtNanos <= hostStartedAt || observed.ObservedAtNanos < process.ObservedAtNanos {
		return failedResourceReceipt{}, errors.New("failed route candidate became available again")
	}
	state := freezeProcessState(observed)
	return failedResourceReceipt{ContainerID: process.Host.Identity,
		ObservedAtNanos: observed.ObservedAtNanos - hostStartedAt, State: state}, nil
}

func validateProcessObservation(value processObservation) error {
	if value.Ref.Adapter == "" || value.Ref.Scope == [32]byte{} || value.Ref.Executable == [32]byte{} ||
		value.Ref.Tree == [32]byte{} || value.Ref.Identity == "" || value.Ref.Incarnation == "" ||
		len(value.AdapterProjection) == 0 || value.OSProcessID == 0 || !value.Running || value.ObservedAtNanos <= 0 {
		return errors.New("host process observation is incomplete or not live")
	}
	return nil
}

func sameProcessIncarnation(left, right candidateProcess) bool {
	left.ObservedAtNanos, right.ObservedAtNanos = 0, 0
	left.HostObservation, right.HostObservation = [32]byte{}, [32]byte{}
	return left == right
}
