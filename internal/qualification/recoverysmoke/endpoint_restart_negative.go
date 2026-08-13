package recoverysmoke

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/recovery"
)

func (observer dockerObserver) endpointRestartNegative(ctx context.Context) (recovery.Negative, error) {
	_, _ = observer.compose(ctx, time.Minute, "down", "-v", "--remove-orphans")
	observer.direction, observer.gateOffset = "client-to-publisher", 17*16_381
	observer.generation = filepath.Join(observer.input.FixtureRoot, "generations", "1")
	if err := configureRecoveryDirection(observer.generation, observer.direction); err != nil {
		return recovery.Negative{}, err
	}
	if _, err := observer.compose(ctx, time.Minute, "--profile", "setup", "run", "--no-deps", "--rm", "volume-init"); err != nil {
		return recovery.Negative{}, err
	}
	gate := filepath.Join(observer.input.FixtureRoot, "gate")
	for _, name := range []string{"client.ready", "client.release", "publisher.ready", "publisher.release"} {
		_ = os.Remove(filepath.Join(gate, name))
	}
	if err := observer.startRecoveryServices(ctx); err != nil {
		return recovery.Negative{}, err
	}
	if _, err := observer.waitGate(ctx, gate, "client", observer.gateOffset); err != nil {
		return recovery.Negative{}, err
	}
	if delivered, err := observer.waitProgress(ctx, "publisher-app", observer.gateOffset); err != nil || delivered != observer.gateOffset {
		return recovery.Negative{}, errors.Join(err, errors.New("endpoint restart cell did not reach its exact gate"))
	}
	started := time.Now()
	publisherID, err := observer.serviceID(ctx, "publisher-endpoint")
	if err != nil {
		return recovery.Negative{}, err
	}
	beforePID, err := observer.containerPID(ctx, publisherID)
	if err != nil {
		return recovery.Negative{}, err
	}
	if _, err := observer.compose(ctx, time.Minute, "restart", "publisher-endpoint"); err != nil {
		return recovery.Negative{}, err
	}
	afterPID, err := observer.containerPID(ctx, publisherID)
	if err != nil || afterPID == beforePID {
		return recovery.Negative{}, errors.Join(err, errors.New("endpoint process identity did not change on restart"))
	}
	if err := writeRelease(gate, "client"); err != nil {
		return recovery.Negative{}, err
	}
	clientID, err := observer.serviceID(ctx, "client-endpoint")
	if err != nil {
		return recovery.Negative{}, err
	}
	if err := observer.waitContainer(ctx, clientID, true); err != nil {
		return recovery.Negative{}, err
	}
	elapsed := time.Since(started)
	raw, err := observer.compose(ctx, time.Minute, "logs", "--no-color", "--no-log-prefix", "client-endpoint")
	if err != nil {
		return recovery.Negative{}, err
	}
	terminal, terminalErr := terminalEndpoint(raw)
	_, cleanupErr := observer.compose(ctx, time.Minute, "down", "-v", "--remove-orphans")
	if terminalErr != nil || cleanupErr != nil {
		return recovery.Negative{}, errors.Join(terminalErr, cleanupErr)
	}
	passed := terminal.Class != "" && terminal.Class != "clean service connection close" && elapsed <= 15*time.Second
	return recovery.Negative{TerminalCount: 1, Class: terminal.Class, WithinNanos: elapsed.Nanoseconds(), Passed: passed,
		ContainerID: publisherID, InjectedResource: "publisher-endpoint", BeforeProcess: beforePID, AfterProcess: afterPID}, nil
}

func writeRelease(root, role string) error {
	return os.WriteFile(filepath.Join(root, role+".release"), []byte("release\n"), 0o600)
}

func (observer dockerObserver) containerPID(ctx context.Context, container string) (string, error) {
	raw, err := observer.docker(ctx, 10*time.Second, "inspect", "--format", "{{.State.Pid}}", container)
	pid := strings.TrimSpace(string(raw))
	if err != nil || pid == "" || pid == "0" {
		return "", errors.Join(err, errors.New("container process identity is missing"))
	}
	return pid, nil
}
