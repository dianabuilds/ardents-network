package recoverysmoke

import (
	"context"
	"errors"
	"testing"
)

func TestReplacementRouteProcessesUseHostAdapter(t *testing.T) {
	identities := map[string]string{"client": "process/client", "publisher": "process/publisher"}
	stub := &hostProcessStub{observations: map[string]processObservation{
		"client":    endpointProcessObservation("client", identities["client"], 1),
		"publisher": endpointProcessObservation("publisher", identities["publisher"], 2),
	}}
	result, err := observeReplacementRouteProcesses(context.Background(), stub, identities)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 2 || result["client"].Host.Identity != identities["client"] ||
		result["publisher"].Host.Identity != identities["publisher"] {
		t.Fatalf("Route process observations are incomplete: %+v", result)
	}
}

func TestReplacementRouteProcessFailurePreservesAdapterCause(t *testing.T) {
	sentinel := errors.New("adapter unavailable")
	_, err := observeReplacementRouteProcesses(context.Background(), &hostProcessStub{resolveErr: sentinel},
		map[string]string{"client": "process/client", "publisher": "process/publisher"})
	if !errors.Is(err, sentinel) {
		t.Fatalf("adapter cause was hidden: %v", err)
	}
}
