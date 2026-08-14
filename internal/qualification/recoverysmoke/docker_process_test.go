package recoverysmoke

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

type dockerProcessCall struct {
	limit     time.Duration
	arguments []string
}

const dockerTestImage = "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"

func dockerProcessJSON(t *testing.T, container string, pid uint32, running bool, service string) []byte {
	t.Helper()
	value := struct {
		ID, Started, Image, Path, Project, Service, PIDMode string
		Args                                                []string
		PID                                                 uint32
		Running                                             bool
	}{ID: container, PID: pid, Started: "2026-08-14T00:00:00Z", Running: running,
		Image: dockerTestImage, Path: "/usr/local/bin/ardents-route", Args: []string{"--plan", "/run/plan.json"},
		Project: "project", Service: service}
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestDockerProcessAdapterUsesExactAuthority(t *testing.T) {
	container := strings.Repeat("a", 64)
	var service string
	var calls []dockerProcessCall
	stopped := false
	adapter := newDockerProcessAdapter(dockerObserver{}, hostScopeEvidence{Adapter: "docker-compose-v1",
		AdapterProjection: "project", Commitment: [32]byte{1}, Image: sha256.Sum256([]byte(dockerTestImage))}, time.Now())
	adapter.serviceID = func(_ context.Context, value string) (string, error) { service = value; return container, nil }
	adapter.command = func(_ context.Context, limit time.Duration, arguments ...string) ([]byte, error) {
		calls = append(calls, dockerProcessCall{limit: limit, arguments: slices.Clone(arguments)})
		if arguments[0] == "stop" {
			stopped = true
			return nil, nil
		}
		if arguments[0] == "inspect" {
			return dockerProcessJSON(t, container, 41, !stopped, "rv-2"), nil
		}
		return nil, nil
	}
	observed, err := adapter.ResolveProcess(context.Background(), processSelector{LogicalRole: "rendezvous", AdapterKey: "rv-2"})
	if err != nil {
		t.Fatal(err)
	}
	ref := bindProcessRef(observed.Ref)
	if _, err := adapter.InjectProcessFault(context.Background(), ref, processFaultSpec{Kind: processStop}); err != nil {
		t.Fatal(err)
	}
	if service != "rv-2" || len(calls) != 4 || calls[1].arguments[0] != "inspect" ||
		calls[1].arguments[len(calls[1].arguments)-1] != container ||
		!slices.Equal(calls[2].arguments, []string{"stop", "-t", "0", container}) ||
		calls[3].arguments[0] != "inspect" || calls[3].arguments[len(calls[3].arguments)-1] != container {
		t.Fatalf("Docker authority was not exact: service=%q calls=%+v", service, calls)
	}
}

func TestDockerProcessAdapterRejectsMalformedInspectionAndCancellation(t *testing.T) {
	container := strings.Repeat("b", 64)
	sentinel := errors.New("inspect unavailable")
	tests := []struct {
		name       string
		raw        []byte
		commandErr error
		cancel     bool
		contains   string
	}{
		{"adapter error", nil, sentinel, false, "inspect unavailable"},
		{"malformed JSON", []byte(`{"id":`), nil, false, "inspection is invalid"},
		{"incomplete observation", dockerProcessJSON(t, container, 0, false, "rv"), nil, false, "incomplete or not live"},
		{"cancelled wait", dockerProcessJSON(t, container, 41, true, "rv"), nil, true, "context canceled"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			adapter := newDockerProcessAdapter(dockerObserver{}, hostScopeEvidence{Adapter: "docker-compose-v1",
				AdapterProjection: "project", Commitment: [32]byte{1},
				Image: sha256.Sum256([]byte(dockerTestImage))}, time.Now())
			adapter.serviceID = func(context.Context, string) (string, error) { return container, nil }
			adapter.command = func(ctx context.Context, _ time.Duration, arguments ...string) ([]byte, error) {
				if test.commandErr != nil {
					return nil, test.commandErr
				}
				return test.raw, nil
			}
			var err error
			if test.cancel {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				tree := dockerProcessProjection("ardents-qualification-process-tree-v1\x00",
					"project", "rv", "", container)
				executable := dockerProcessProjection("ardents-qualification-executable-v1\x00",
					dockerTestImage, "/usr/local/bin/ardents-route", "--plan", "/run/plan.json")
				ref := processEvidenceRef{Adapter: "docker-compose-v1", Scope: [32]byte{1}, Executable: executable,
					Tree: tree, Identity: container, Incarnation: container + "@2026-08-14T00:00:00Z"}
				ref.Commitment = processRefCommitment(ref)
				_, err = adapter.AwaitProcessState(ctx, ref, processStopped, time.Second)
			} else {
				_, err = adapter.ResolveProcess(context.Background(),
					processSelector{LogicalRole: "rendezvous", AdapterKey: "rv"})
			}
			if err == nil || !strings.Contains(err.Error(), test.contains) {
				t.Fatalf("expected %q failure, got %v", test.contains, err)
			}
			if test.commandErr != nil && !errors.Is(err, sentinel) {
				t.Fatalf("cause was hidden: %v", err)
			}
		})
	}
}

func TestDockerProcessAdapterRejectsAuthorityOutsideScope(t *testing.T) {
	container := strings.Repeat("c", 64)
	adapter := newDockerProcessAdapter(dockerObserver{}, hostScopeEvidence{Adapter: "docker-compose-v1",
		AdapterProjection: "project", Commitment: [32]byte{1},
		Image: sha256.Sum256([]byte(dockerTestImage))}, time.Now())
	called := false
	adapter.command = func(context.Context, time.Duration, ...string) ([]byte, error) {
		called = true
		return nil, nil
	}
	ref := processEvidenceRef{Adapter: "docker-compose-v1", Scope: [32]byte{9}, Executable: [32]byte{2},
		Tree: [32]byte{3}, Identity: container, Incarnation: container + "@2026-08-14T00:00:00Z"}
	ref.Commitment = processRefCommitment(ref)
	if _, err := adapter.InjectProcessFault(context.Background(), ref, processFaultSpec{Kind: processStop}); err == nil {
		t.Fatal("out-of-scope process fault was accepted")
	}
	if called {
		t.Fatal("out-of-scope fault reached Docker")
	}
}
