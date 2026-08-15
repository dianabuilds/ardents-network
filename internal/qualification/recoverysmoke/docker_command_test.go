package recoverysmoke

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/serviceconn"
)

func TestStreamBoundsAreDirectionalForRecoveryCells(t *testing.T) {
	tests := map[string][4]uint32{
		"":                    {1024, 1024, 1024, 1024},
		"client-to-publisher": {1024, 0, 0, 1024},
		"publisher-to-client": {0, 1024, 1024, 0},
	}
	for direction, want := range tests {
		clientSend, clientReceive, publisherSend, publisherReceive := streamBounds(1024, direction)
		got := [4]uint32{clientSend, clientReceive, publisherSend, publisherReceive}
		if got != want {
			t.Fatalf("direction %q bounds = %v; want %v", direction, got, want)
		}
	}
}

func TestRecoveryOperationLifetimeBindsEndpointAndApplicationPlans(t *testing.T) {
	observer := (dockerObserver{}).forRecoveryOperation("client-to-publisher")
	environment := observer.streamEnvironment(1024)
	if !slices.Contains(environment, "ARDENTS_STREAM_LIFETIME="+recoveryOperationLifetime) {
		t.Fatalf("stream environment %v lacks the recovery operation lifetime", environment)
	}
	if observer.direction != "client-to-publisher" {
		t.Fatalf("operation direction = %q", observer.direction)
	}
	common := endpointGenerationPlan(time.Unix(1, 0), serviceconn.Credential{}, [32]byte{}, [32]byte{})
	if common["Deadline"] != "15s" || common["Lifetime"] != recoveryOperationLifetime {
		t.Fatalf("endpoint operation contract = %v", common)
	}
}

func TestRecoveryDownIncludesAllRecoveryProfiles(t *testing.T) {
	want := []string{"--profile", "*", "down", "-v", "--remove-orphans"}
	if got := recoveryDownArguments(); !slices.Equal(got, want) {
		t.Fatalf("recovery down arguments = %v; want %v", got, want)
	}
}

func TestRecoveryDownPreservesCleanupFailure(t *testing.T) {
	cause := errors.New("Docker unavailable")
	err := runRecoveryDown(func(arguments ...string) ([]byte, error) {
		if !slices.Equal(arguments, recoveryDownArguments()) {
			t.Fatalf("down arguments = %v", arguments)
		}
		return nil, cause
	})
	if !errors.Is(err, cause) || err.Error() != "reset recovery topology: Docker unavailable" {
		t.Fatalf("down error = %v; want contextualized %v", err, cause)
	}
}
