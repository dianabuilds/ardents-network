package recoverysmoke

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/recovery"
)

func (observer dockerObserver) endpointRestartNegative(ctx context.Context) (recovery.Negative, error) {
	if err := observer.resetRecoveryTopology(ctx, time.Minute); err != nil {
		return recovery.Negative{}, err
	}
	observer.direction, observer.gateOffset = "client-to-publisher", 17*16_381
	observer.generation = filepath.Join(observer.input.FixtureRoot, "generations", "1")
	if err := setRouteAttachments(observer.input.FixtureRoot, 1); err != nil {
		return recovery.Negative{}, err
	}
	if err := configureRecoveryDirection(observer.generation, observer.direction); err != nil {
		return recovery.Negative{}, err
	}
	if _, err := observer.compose(ctx, time.Minute, "--profile", "setup", "run", "--no-deps", "--rm", "volume-init"); err != nil {
		return recovery.Negative{}, err
	}
	gate := filepath.Join(observer.input.FixtureRoot, "gate")
	if err := resetRecoveryGates(gate); err != nil {
		return recovery.Negative{}, err
	}
	if err := observer.startRecoveryServices(ctx); err != nil {
		return recovery.Negative{}, err
	}
	gateWait, err := pacedGateWait(0, observer.gateOffset, observer.input.ChunkDelay)
	if err != nil {
		return recovery.Negative{}, err
	}
	if _, err := observer.waitGate(ctx, gate, "client", observer.gateOffset, gateWait); err != nil {
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
	beforeProcess, err := observer.containerProcessIdentity(ctx, publisherID)
	if err != nil {
		return recovery.Negative{}, err
	}
	if _, err := observer.compose(ctx, time.Minute, endpointRestartArguments()...); err != nil {
		return recovery.Negative{}, err
	}
	afterProcess, err := observer.containerProcessIdentity(ctx, publisherID)
	if err != nil || afterProcess == beforeProcess {
		return recovery.Negative{}, errors.Join(err, errors.New("endpoint process identity did not change on restart"))
	}
	if err := writeRelease(gate, "client"); err != nil {
		return recovery.Negative{}, err
	}
	clientID, err := observer.serviceID(ctx, "client-endpoint")
	if err != nil {
		return recovery.Negative{}, err
	}
	if err := observer.waitContainer(ctx, clientID, false); err != nil {
		return recovery.Negative{}, err
	}
	elapsed := time.Since(started)
	raw, err := observer.compose(ctx, time.Minute, "logs", "--no-color", "--no-log-prefix", "client-endpoint")
	if err != nil {
		return recovery.Negative{}, err
	}
	terminal, terminalErr := terminalEndpoint(raw)
	cleanupErr := observer.resetRecoveryTopology(ctx, time.Minute)
	if terminalErr != nil || cleanupErr != nil {
		return recovery.Negative{}, errors.Join(terminalErr, cleanupErr)
	}
	passed := terminal.Class != "" && terminal.Class != "clean service connection close" && elapsed <= 15*time.Second
	return recovery.Negative{TerminalCount: 1, Class: terminal.Class, WithinNanos: elapsed.Nanoseconds(), Passed: passed,
		ContainerID: publisherID, InjectedResource: "publisher-endpoint",
		BeforeProcess: beforeProcess, AfterProcess: afterProcess}, nil
}

func endpointRestartArguments() []string {
	return []string{"restart", "--timeout", "1", "publisher-endpoint"}
}

func writeRelease(root, role string) error {
	return os.WriteFile(filepath.Join(root, role+".release"), []byte("release\n"), 0o600)
}

func writeSequentialRelease(root, role string, offset uint32) error {
	name := role + ".release." + strconv.FormatUint(uint64(offset), 10)
	return os.WriteFile(filepath.Join(root, name), []byte("release\n"), 0o600)
}

func (observer dockerObserver) containerProcessIdentity(ctx context.Context, container string) (string, error) {
	raw, err := observer.docker(ctx, 10*time.Second, "inspect", "--format", "{{.Id}} {{.State.StartedAt}}", container)
	if err != nil {
		return "", errors.Join(err, errors.New("container process identity inspection failed"))
	}
	return parseProcessIdentity(container, raw)
}

func parseProcessIdentity(container string, raw []byte) (string, error) {
	fields := strings.Fields(string(raw))
	if len(fields) != 2 || !validContainerID(container) || fields[0] != container {
		return "", errors.New("container process identity is missing")
	}
	started, err := time.Parse(time.RFC3339Nano, fields[1])
	if err != nil || started.IsZero() {
		return "", errors.Join(err, errors.New("container process start identity is invalid"))
	}
	return container + "@" + started.UTC().Format(time.RFC3339Nano), nil
}
