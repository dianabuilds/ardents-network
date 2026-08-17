//go:build live

package network_test

import (
	"context"
	"strings"
	"testing"
	"time"
)

func waitNamedContainer(t *testing.T, ctx context.Context, name string) {
	t.Helper()
	output, err := dockerOutput(ctx, "wait", name)
	if err != nil || strings.TrimSpace(string(output)) != "0" {
		logs, _ := dockerOutput(ctx, "logs", name)
		t.Fatalf("capacity container %s exit=%s err=%v\n%s", name, output, err, logs)
	}
}

func waitScaledComposeService(t *testing.T, ctx context.Context, compose composeCall, service string, count int) {
	t.Helper()
	output, err := compose(ctx, "ps", "-q", service)
	identities := strings.Fields(string(output))
	if err != nil || len(identities) != count {
		t.Fatalf("resolve %d scaled %s containers: %v\n%s", count, service, err, output)
	}
	for _, identity := range identities {
		waitNamedContainer(t, ctx, identity)
	}
}

func assertContainerDuration(t *testing.T, ctx context.Context, name string, started time.Time, maximum time.Duration) {
	t.Helper()
	output, err := dockerOutput(ctx, "inspect", "--format", "{{.State.FinishedAt}}", name)
	if err != nil {
		t.Fatalf("inspect capacity duration %s: %v\n%s", name, err, output)
	}
	finished, finishErr := time.Parse(time.RFC3339Nano, strings.TrimSpace(string(output)))
	if finishErr != nil || finished.Sub(started) > maximum {
		t.Fatalf("capacity response %s duration=%s parse=%v", name, finished.Sub(started), finishErr)
	}
}

func removeCapacityProjectObjects(t *testing.T, project string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	containers, _ := dockerOutput(ctx, "ps", "-aq", "--filter", "label=com.docker.compose.project="+project)
	if values := strings.Fields(string(containers)); len(values) > 0 {
		arguments := append([]string{"rm", "-f"}, values...)
		if output, err := dockerOutput(ctx, arguments...); err != nil {
			t.Errorf("remove capacity containers: %v\n%s", err, output)
		}
	}
	volumes, _ := dockerOutput(ctx, "volume", "ls", "-q", "--filter", "label=com.docker.compose.project="+project)
	if values := strings.Fields(string(volumes)); len(values) > 0 {
		arguments := append([]string{"volume", "rm", "-f"}, values...)
		if output, err := dockerOutput(ctx, arguments...); err != nil {
			t.Errorf("remove capacity volumes: %v\n%s", err, output)
		}
	}
}
