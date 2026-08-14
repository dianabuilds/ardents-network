package recoverysmoke

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/recovery"
)

type trafficObservers struct {
	ids         [2]string
	routes      [2]string
	projections [2]recovery.ObserverProcess
}

type trafficBaseline struct {
	client, publisher uint64
	terminalNanos     int64
	finalTraffic      recovery.ResourceSample
	routes            [2]string
	observers         [2]recovery.ObserverProcess
}

func (observer dockerObserver) startTrafficObservers(ctx context.Context,
	identities map[string]string) (trafficObservers, error) {
	var result trafficObservers
	for index, role := range []string{"client", "publisher"} {
		result.routes[index] = identities[role]
		identity, projection, err := observer.startTrafficObserver(ctx, identities[role])
		if err != nil {
			removeErr := result.remove(context.Background(), observer)
			return trafficObservers{}, fmt.Errorf("start %s Route traffic observer: %w", role,
				errors.Join(err, removeErr))
		}
		result.ids[index], result.projections[index] = identity, projection
	}
	return result, nil
}

func (observer dockerObserver) startTrafficObserver(ctx context.Context,
	route string) (string, recovery.ObserverProcess, error) {
	raw, err := observer.docker(ctx, 10*time.Second, "create", "--network", "container:"+route,
		"--ipc", "private", "--read-only", "--cap-drop", "ALL", "--security-opt", "no-new-privileges",
		"--user", "65532:65532", "--pids-limit", "16", "--memory", "32m", "--cpus", "0.25",
		"--label", "com.docker.compose.project="+observer.project, observer.imageID,
		"/usr/local/bin/ardents-qualify", "carrier-fault", "traffic-wait")
	identity := strings.TrimSpace(string(raw))
	if err != nil || !validContainerID(identity) {
		return "", recovery.ObserverProcess{}, errors.Join(err, errors.New("traffic observer identity is invalid"))
	}
	projection, inspectErr := observer.inspectReplacementObserver(ctx, identity)
	if inspectErr != nil {
		_, removeErr := observer.docker(context.Background(), 10*time.Second, "rm", "-f", identity)
		return "", recovery.ObserverProcess{}, errors.Join(inspectErr, removeErr)
	}
	if err := observer.startTrafficObserverProcess(ctx, identity); err != nil {
		_, removeErr := observer.docker(context.Background(), 10*time.Second, "rm", "-f", identity)
		return "", recovery.ObserverProcess{}, errors.Join(err, removeErr)
	}
	return identity, projection, nil
}

func (observer dockerObserver) startTrafficObserverProcess(ctx context.Context, identity string) error {
	return retryObserverStart(ctx, func(attemptCtx context.Context) error {
		_, err := observer.docker(attemptCtx, time.Second, "start", identity)
		return err
	}, func(attemptCtx context.Context) (bool, error) {
		raw, err := observer.docker(attemptCtx, time.Second, "inspect", "--format",
			"{{.Id}} {{.State.Running}}", identity)
		if err != nil {
			return false, err
		}
		value := strings.TrimSpace(string(raw))
		if value == identity+" true" {
			return true, nil
		}
		if value == identity+" false" {
			return false, nil
		}
		return false, errors.New("traffic observer running inspection is invalid")
	})
}

func retryObserverStart(ctx context.Context, start func(context.Context) error,
	running func(context.Context) (bool, error)) error {
	retryCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var failures error
	for attempt := 1; ; attempt++ {
		startErr := start(retryCtx)
		isRunning, inspectErr := running(retryCtx)
		if isRunning && inspectErr == nil {
			return nil
		}
		if startErr != nil {
			failures = errors.Join(failures, fmt.Errorf("start traffic observer attempt %d: %w", attempt, startErr))
		}
		if inspectErr != nil {
			failures = errors.Join(failures, fmt.Errorf("inspect traffic observer attempt %d: %w", attempt, inspectErr))
		}
		if startErr == nil && inspectErr == nil {
			failures = errors.Join(failures,
				fmt.Errorf("traffic observer was not running after start attempt %d", attempt))
		}
		select {
		case <-retryCtx.Done():
			return errors.Join(fmt.Errorf("start traffic observer within retry bound: %w", retryCtx.Err()), failures)
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func (value *trafficObservers) snapshotAndRemove(ctx context.Context, observer dockerObserver,
	clock time.Time) (recovery.ResourceSample, error) {
	var sample recovery.ResourceSample
	client, clientErr := observer.observeTrafficCounter(ctx, value.ids[0])
	publisher, publisherErr := observer.observeTrafficCounter(ctx, value.ids[1])
	if clientErr == nil {
		sample.ClientReceived, sample.ClientSent = client.Received, client.Sent
	}
	if publisherErr == nil {
		sample.PublisherReceived, sample.PublisherSent = publisher.Received, publisher.Sent
	}
	sample.AtNanos = time.Since(clock).Nanoseconds()
	removeErr := value.remove(context.Background(), observer)
	return sample, errors.Join(clientErr, publisherErr, removeErr)
}

func (observer dockerObserver) observeTrafficCounter(ctx context.Context, identity string) (trafficCounterReceipt, error) {
	if _, err := observer.docker(ctx, 10*time.Second, "kill", "--signal", "USR1", identity); err != nil {
		return trafficCounterReceipt{}, err
	}
	retryCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	var lastErr error
	for {
		raw, err := observer.boundedTrafficLogs(retryCtx, identity)
		if err == nil {
			if value, parseErr := parseTrafficReceipt(raw); parseErr == nil {
				return value, nil
			} else {
				lastErr = parseErr
			}
		} else {
			lastErr = err
		}
		select {
		case <-retryCtx.Done():
			return trafficCounterReceipt{}, errors.Join(errors.New("terminal network traffic observation ended"),
				retryCtx.Err(), lastErr)
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func (observer dockerObserver) boundedTrafficLogs(ctx context.Context, identity string) ([]byte, error) {
	tail := &boundedTail{limit: 8 << 10}
	command := exec.CommandContext(ctx, "docker", "logs", "--tail", "4", identity)
	command.Dir, command.Stdout, command.Stderr = observer.input.SourceRoot, tail, tail
	if err := command.Run(); err != nil {
		digest := sha256.Sum256(tail.value())
		return nil, fmt.Errorf("read traffic observer logs (sha256=%x): %w", digest, err)
	}
	return tail.value(), nil
}

func parseTrafficReceipt(raw []byte) (trafficCounterReceipt, error) {
	var result trafficCounterReceipt
	readyCount, trafficCount := 0, 0
	for _, line := range splitLines(raw) {
		var value trafficCounterReceipt
		if err := json.Unmarshal(line, &value); err != nil {
			return trafficCounterReceipt{}, fmt.Errorf("decode traffic observer receipt: %w", err)
		}
		if value.Kind == "ready" {
			readyCount++
			continue
		}
		if value.Kind != "traffic" || value.Interfaces == 0 || value.Received == 0 || value.Sent == 0 {
			return trafficCounterReceipt{}, errors.New("terminal network traffic observation is invalid")
		}
		result, trafficCount = value, trafficCount+1
	}
	if readyCount != 1 || trafficCount != 1 {
		return trafficCounterReceipt{}, errors.New("exactly one ready and one terminal network traffic receipt are required")
	}
	return result, nil
}

func (value *trafficObservers) remove(ctx context.Context, observer dockerObserver) error {
	var result error
	for index, identity := range value.ids {
		if identity == "" {
			continue
		}
		_, removeErr := observer.docker(ctx, 10*time.Second, "rm", "-f", identity)
		present, presenceErr := observer.docker(ctx, 10*time.Second, "ps", "-a", "-q", "--no-trunc", "--filter", "id="+identity)
		if removeErr != nil || presenceErr != nil || strings.TrimSpace(string(present)) != "" {
			result = errors.Join(result, removeErr, presenceErr, errors.New("traffic observer removal is incomplete"))
			continue
		}
		value.projections[index].Removed = true
		value.ids[index] = ""
	}
	return result
}
