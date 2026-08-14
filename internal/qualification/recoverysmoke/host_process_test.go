package recoverysmoke

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

type hostProcessStub struct {
	observations                   map[string]processObservation
	fault                          processFaultReceipt
	state                          processStateObservation
	resolveErr, faultErr, stateErr error
	selector                       processSelector
	faultRef, stateRef             processEvidenceRef
	faultSpec                      processFaultSpec
	wanted                         processState
	limit                          time.Duration
	selectors                      []processSelector
}

func (stub *hostProcessStub) ResolveProcess(_ context.Context,
	selector processSelector) (processObservation, error) {
	stub.selector = selector
	stub.selectors = append(stub.selectors, selector)
	return stub.observations[selector.AdapterKey], stub.resolveErr
}

func (stub *hostProcessStub) InjectProcessFault(_ context.Context, ref processEvidenceRef,
	spec processFaultSpec) (processFaultReceipt, error) {
	stub.faultRef, stub.faultSpec = ref, spec
	return stub.fault, stub.faultErr
}

func (stub *hostProcessStub) AwaitProcessState(_ context.Context, ref processEvidenceRef,
	wanted processState, limit time.Duration) (processStateObservation, error) {
	stub.stateRef, stub.wanted, stub.limit = ref, wanted, limit
	return stub.state, stub.stateErr
}

func TestRouteGenerationUsesHostProcessObservations(t *testing.T) {
	fixture := prepared{}
	selection := selectedRoute{}
	observations := map[string]processObservation{}
	for index, roleName := range replacementRoles {
		candidate := route.Position{Role: roleName, Endpoint: "127.0.0.1:4604"}
		candidate.NodeID[0], candidate.PublicKey[0] = byte(index+1), byte(index+11)
		fixture.candidates = append(fixture.candidates, candidate)
		selection[roleName] = candidate
		observations[roleName] = processObservation{Ref: processRef{Adapter: "native-test",
			Scope: [32]byte{1}, Executable: [32]byte{2}, Tree: [32]byte{3},
			Identity: "process/" + roleName, Incarnation: "boot-7/start-" + roleName},
			AdapterProjection: []byte(`{"kind":"native-test"}`),
			OSProcessID:       uint32(index + 41), Running: true, ObservedAtNanos: int64(index + 1)}
	}
	stub := &hostProcessStub{observations: observations}
	generation, err := observeRouteGeneration(context.Background(), stub,
		fixture, 2, selection)
	if err != nil {
		t.Fatal(err)
	}
	for index, roleName := range replacementRoles {
		process := generation.Processes[roleName]
		if process.ContainerID != "process/"+roleName ||
			process.Incarnation != "boot-7/start-"+roleName || process.PID != uint32(index+41) ||
			process.Host.Adapter != "native-test" || process.Host.Scope == [32]byte{} ||
			process.Host.Identity != process.ContainerID || process.Host.Incarnation != process.Incarnation ||
			process.Host.Commitment == [32]byte{} {
			t.Fatalf("%s process observation was not preserved: %+v", roleName, process)
		}
	}
	if stub.selector.LogicalRole != "responder" || stub.selector.AdapterKey != "responder" {
		t.Fatalf("last exact selector was not preserved: %+v", stub.selector)
	}
}

func TestCandidateFaultUsesHostProcessObservations(t *testing.T) {
	ref := processEvidenceRef{Adapter: "native-test", Scope: [32]byte{7},
		Executable: [32]byte{8}, Tree: [32]byte{9},
		Identity: "process/rendezvous", Incarnation: "boot-7/start-rendezvous"}
	ref.Commitment = processRefCommitment(ref)
	process := candidateProcess{ContainerID: ref.Identity, Incarnation: ref.Incarnation, PID: 41,
		ObservedAtNanos: 1, Host: ref}
	adapter := &hostProcessStub{fault: processFaultReceipt{Ref: ref, Kind: processStop, State: processStopped,
		InvocationStartedNanos: 1, InvocationCompletedNanos: 3, ObservedAtNanos: 2},
		state: processStateObservation{Ref: ref, State: processStopped, ObservedAtNanos: 4}}
	fault, err := stopCandidate(context.Background(), adapter, process)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := candidateUnavailable(context.Background(), adapter, process, 1)
	if err != nil {
		t.Fatal(err)
	}
	if fault.Commitment == [32]byte{} || receipt.ContainerID != ref.Identity || receipt.Running ||
		receipt.State.Commitment == [32]byte{} {
		t.Fatalf("stopped process receipt is invalid: %+v", receipt)
	}
	if adapter.faultRef != ref || adapter.faultSpec.Kind != processStop || adapter.stateRef != ref ||
		adapter.wanted != processStopped || adapter.limit != 10*time.Second {
		t.Fatalf("host process authority changed: %+v", adapter)
	}
}

func TestHostProcessFailuresAreFailClosed(t *testing.T) {
	ref := processEvidenceRef{Adapter: "native-test", Scope: [32]byte{1}, Executable: [32]byte{2},
		Tree: [32]byte{3}, Identity: "process/rendezvous", Incarnation: "boot/start"}
	ref.Commitment = processRefCommitment(ref)
	process := candidateProcess{ObservedAtNanos: 1, Host: ref}
	sentinel := errors.New("adapter unavailable")
	tests := []struct {
		name     string
		stub     *hostProcessStub
		invoke   func(*hostProcessStub) error
		contains string
	}{
		{"fault adapter error", &hostProcessStub{faultErr: sentinel}, func(stub *hostProcessStub) error {
			_, err := stopCandidate(context.Background(), stub, process)
			return err
		}, "stop exact Route candidate process"},
		{"fault mismatched ref", &hostProcessStub{fault: processFaultReceipt{Ref: processEvidenceRef{},
			Kind: processStop, State: processStopped, InvocationStartedNanos: 1,
			InvocationCompletedNanos: 2, ObservedAtNanos: 2}}, func(stub *hostProcessStub) error {
			_, err := stopCandidate(context.Background(), stub, process)
			return err
		}, "fault receipt is inconsistent"},
		{"state adapter error", &hostProcessStub{stateErr: sentinel}, func(stub *hostProcessStub) error {
			_, err := candidateUnavailable(context.Background(), stub, process, 1)
			return err
		}, "observe failed Route candidate state"},
		{"state mismatched ref", &hostProcessStub{state: processStateObservation{Ref: processEvidenceRef{},
			State: processStopped, ObservedAtNanos: 3}}, func(stub *hostProcessStub) error {
			_, err := candidateUnavailable(context.Background(), stub, process, 1)
			return err
		}, "became available again"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.invoke(test.stub)
			if err == nil || !strings.Contains(err.Error(), test.contains) {
				t.Fatalf("expected %q failure, got %v", test.contains, err)
			}
			if test.stub.faultErr != nil || test.stub.stateErr != nil {
				if !errors.Is(err, sentinel) {
					t.Fatalf("adapter cause was hidden: %v", err)
				}
			}
		})
	}
}
