package recoverysmoke

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestDockerCleanupObservationUsesExactOwnedScope(t *testing.T) {
	var calls [][]string
	command := func(_ context.Context, limit time.Duration, arguments ...string) ([]byte, error) {
		if limit != time.Minute {
			t.Fatalf("cleanup command limit = %s", limit)
		}
		calls = append(calls, append([]string(nil), arguments...))
		return nil, nil
	}
	scope := hostScopeEvidence{Adapter: "docker-compose-v1", Commitment: [32]byte{7}}
	result, err := collectDockerCleanup(context.Background(), command, "campaign", scope, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"container", "ls", "-a", "-q", "--filter", "label=com.docker.compose.project=campaign"},
		{"network", "ls", "-q", "--filter", "label=com.docker.compose.project=campaign"},
		{"volume", "ls", "-q", "--filter", "label=com.docker.compose.project=campaign"},
	}
	if !reflect.DeepEqual(calls, want) || result.adapter != scope.Adapter || result.scope != scope.Commitment ||
		result.owned != 0 || result.observedAt <= 0 || result.commitment == [32]byte{} {
		t.Fatalf("cleanup observation is incomplete: calls=%v result=%+v", calls, result)
	}
}

func TestDockerCleanupObservationCountsStoppedContainers(t *testing.T) {
	command := func(_ context.Context, _ time.Duration, arguments ...string) ([]byte, error) {
		if len(arguments) >= 3 && arguments[0] == "container" && arguments[2] == "-a" {
			return []byte("stopped-container\n"), nil
		}
		return nil, nil
	}
	_, err := collectDockerCleanup(context.Background(), command, "campaign",
		hostScopeEvidence{Adapter: "docker-compose-v1", Commitment: [32]byte{1}}, time.Now())
	if err == nil {
		t.Fatal("stopped owned container was omitted from cleanup observation")
	}
}

func TestDockerCleanupObservationFailuresAreFailClosed(t *testing.T) {
	sentinel := errors.New("runtime unavailable")
	for name, command := range map[string]cleanupDockerCommand{
		"query error": func(context.Context, time.Duration, ...string) ([]byte, error) {
			return nil, sentinel
		},
		"owned resource": func(context.Context, time.Duration, ...string) ([]byte, error) {
			return []byte("owned-resource\n"), nil
		},
		"oversized output": func(context.Context, time.Duration, ...string) ([]byte, error) {
			return []byte(strings.Repeat("x", cleanupListMaximum+1)), nil
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := collectDockerCleanup(context.Background(), command, "campaign",
				hostScopeEvidence{Adapter: "docker-compose-v1", Commitment: [32]byte{1}}, time.Now())
			if err == nil {
				t.Fatal("cleanup observation unexpectedly succeeded")
			}
			if name == "query error" && !errors.Is(err, sentinel) {
				t.Fatalf("runtime cause was hidden: %v", err)
			}
		})
	}
}

func TestCleanupCommandCaptureIsBounded(t *testing.T) {
	t.Setenv("ARDENTS_CLEANUP_OUTPUT_HELPER", "overflow")
	observer := dockerObserver{}
	_, err := observer.commandBounded(context.Background(), time.Minute, 32,
		os.Args[0], "-test.run=TestCleanupOutputHelper")
	if err == nil || !strings.Contains(err.Error(), "output exceeded 32 bytes") {
		t.Fatalf("oversized command output error = %v", err)
	}
}

func TestCleanupCommandPreservesExitFailure(t *testing.T) {
	t.Setenv("ARDENTS_CLEANUP_OUTPUT_HELPER", "failure")
	observer := dockerObserver{}
	_, err := observer.commandBounded(context.Background(), time.Minute, 32,
		os.Args[0], "-test.run=TestCleanupOutputHelper")
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 7 {
		t.Fatalf("bounded command exit error = %v", err)
	}
}

func TestCleanupOutputHelper(t *testing.T) {
	switch os.Getenv("ARDENTS_CLEANUP_OUTPUT_HELPER") {
	case "overflow":
		_, _ = os.Stdout.WriteString(strings.Repeat("x", 128))
	case "failure":
		_, _ = os.Stderr.WriteString("bounded diagnostic")
		os.Exit(7)
	default:
		return
	}
	os.Exit(0)
}
