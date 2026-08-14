package recoverysmoke

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestReplacementEndpointProcessesUseHostAdapter(t *testing.T) {
	identities, observations := map[string]string{}, map[string]processObservation{}
	for index, role := range replacementEndpointProcessRoles {
		identity := "process/" + role
		identities[role] = identity
		observations[role] = endpointProcessObservation(role, identity, int64(index+1))
	}
	stub := &hostProcessStub{observations: observations}
	result, err := observeReplacementEndpointProcesses(context.Background(), stub, identities)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 4 || len(stub.selectors) != 4 {
		t.Fatalf("observed=%d selectors=%d", len(result), len(stub.selectors))
	}
	for index, role := range replacementEndpointProcessRoles {
		if stub.selectors[index] != (processSelector{LogicalRole: role, AdapterKey: role}) ||
			result[role].Host.Identity != identities[role] || result[role].HostObservation == [32]byte{} {
			t.Fatalf("%s host observation is not exact: %+v", role, result[role])
		}
	}
}

func TestReplacementEndpointProcessFailuresAreFailClosed(t *testing.T) {
	sentinel := errors.New("adapter unavailable")
	valid := endpointProcessObservation("client-endpoint", "process/client-endpoint", 1)
	for name, stub := range map[string]*hostProcessStub{
		"adapter error": {resolveErr: sentinel},
		"incomplete":    {observations: map[string]processObservation{"client-endpoint": {}}},
		"identity changed": {observations: map[string]processObservation{
			"client-endpoint": endpointProcessObservation("client-endpoint", "process/other", 1)}},
		"missing later role": {observations: map[string]processObservation{"client-endpoint": valid}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := observeReplacementEndpointProcesses(context.Background(), stub, map[string]string{
				"client-endpoint": "process/client-endpoint", "publisher-endpoint": "process/publisher-endpoint",
				"client-app": "process/client-app", "publisher-app": "process/publisher-app",
			})
			if err == nil || !strings.Contains(err.Error(), "resolve") {
				t.Fatalf("expected contextual failure, got %v", err)
			}
			if name == "adapter error" && !errors.Is(err, sentinel) {
				t.Fatalf("adapter cause was hidden: %v", err)
			}
		})
	}
}

func endpointProcessObservation(role, identity string, observedAt int64) processObservation {
	return processObservation{Ref: processRef{Adapter: "native-test", Scope: [32]byte{1},
		Executable: [32]byte{2}, Tree: [32]byte{3}, Identity: identity, Incarnation: "boot/" + role},
		AdapterProjection: []byte(`{"kind":"native-test"}`), OSProcessID: 41,
		Running: true, ObservedAtNanos: observedAt}
}
