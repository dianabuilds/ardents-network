package recoverysmoke

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func (observer dockerObserver) runStressDirect(ctx context.Context, direction, phase string,
	hostScope hostScopeEvidence) (result directBaseline, returnErr error) {
	if err := observer.resetRecoveryTopology(ctx, time.Minute); err != nil {
		return result, err
	}
	observer.direction, observer.startGate = direction, true
	observer.directClientFlow, observer.directPublisherFlow, observer.directSeedName = "send", "receive", "client-seed.hex"
	if direction == "publisher-to-client" {
		observer.directClientFlow, observer.directPublisherFlow, observer.directSeedName = "receive", "send", "publisher-seed.hex"
	}
	seed, err := recoveryDirectionSeed(observer.generation, direction)
	if err != nil {
		return result, err
	}
	gateRoot := filepath.Join(observer.input.FixtureRoot, "gate")
	if err := resetDirectGates(gateRoot); err != nil {
		return result, err
	}
	if _, err := observer.compose(ctx, time.Minute, "--profile", "s43-direct", "up", "-d",
		"direct-client-anchor", "direct-publisher-anchor"); err != nil {
		return result, err
	}
	for _, service := range []string{"direct-client-anchor", "direct-publisher-anchor"} {
		if err := observer.waitReady(ctx, service); err != nil {
			return result, err
		}
	}
	anchorIDs, err := observer.serviceIDs(ctx, []string{"direct-client-anchor", "direct-publisher-anchor"})
	if err != nil {
		return result, err
	}
	targets := map[string]string{"client": anchorIDs["direct-client-anchor"],
		"publisher": anchorIDs["direct-publisher-anchor"]}
	clock := time.Now()
	shapers, err := prepareStressShapers(observer, phase, true, targets, clock)
	if err != nil {
		return result, err
	}
	observer = shapers.observer
	if err := shapers.start(ctx); err != nil {
		return result, err
	}
	defer func() {
		if !shapers.values[0].Removed {
			_, finishErr := shapers.finish(context.Background())
			returnErr = errors.Join(returnErr, finishErr)
		}
	}()
	if _, err := observer.compose(ctx, time.Minute, "--profile", "s43-direct", "up", "-d", "direct-publisher"); err != nil {
		return result, err
	}
	if err := observer.waitReady(ctx, "direct-publisher"); err != nil {
		return result, err
	}
	if _, err := observer.compose(ctx, time.Minute, "--profile", "s43-direct", "up", "-d", "direct-client"); err != nil {
		return result, err
	}
	if err := waitDirectStart(ctx, gateRoot); err != nil {
		return result, err
	}
	identities, err := observer.serviceIDs(ctx, []string{"direct-client", "direct-publisher"})
	if err != nil {
		return result, err
	}
	processes, err := observeDirectProcesses(ctx, observer, hostScope, clock, identities)
	if err != nil {
		return result, err
	}
	started, err := releaseDirectStart(gateRoot)
	if err != nil {
		return result, err
	}
	receiver := "direct-publisher"
	if direction == "publisher-to-client" {
		receiver = "direct-client"
	}
	progress, completed, err := observer.measureProgress(ctx, receiver, started, time.Minute)
	if err != nil {
		return result, err
	}
	shaperValues, err := shapers.finish(ctx)
	if err != nil {
		return result, err
	}
	for _, service := range []string{"direct-client", "direct-publisher"} {
		if err := observer.waitContainerFor(ctx, identities[service], true, 4*time.Minute); err != nil {
			return result, err
		}
	}
	terminalAt := time.Now()
	application, err := observer.directTerminal(ctx, receiver)
	if err != nil || application.Terminal != "success" || application.ReceivedBytes != 256<<20 {
		return result, errors.Join(err, errors.New("S4.3 direct baseline did not terminate cleanly"))
	}
	result = directBaseline{Direction: direction, Seed: seed, ExpectedDigest: workloadDigest(seed, 256<<20),
		ObservedDigest: application.ReceivedDigest, Bytes: 256 << 20,
		MeasurementDelivered: progress[len(progress)-1].Delivered,
		ActiveStartedAtNanos: started.Sub(clock).Nanoseconds(), ActiveEndedAtNanos: completed.Sub(clock).Nanoseconds(),
		TerminalNanos: terminalAt.Sub(clock).Nanoseconds(), Progress: progress,
		Processes: processes, Shapers: shaperValues}
	if err := observer.resetRecoveryTopology(ctx, time.Minute); err != nil {
		return result, err
	}
	return result, nil
}

func resetDirectGates(root string) error {
	var paths []string
	for _, role := range []string{"direct-listen", "direct-connect"} {
		ready := filepath.Join(root, role+".start.ready")
		paths = append(paths, ready, ready+".pending", filepath.Join(root, role+".start.release"))
	}
	return removeHostGateState(paths)
}

func waitDirectStart(ctx context.Context, root string) error {
	var result error
	for _, role := range []string{"direct-listen", "direct-connect"} {
		_, err := (&dockerObserver{}).waitGateFile(ctx, filepath.Join(root, role+".start.ready"), 0, time.Minute)
		result = errors.Join(result, err)
	}
	return result
}

func releaseDirectStart(root string) (time.Time, error) {
	for _, role := range []string{"direct-listen", "direct-connect"} {
		if err := os.WriteFile(filepath.Join(root, role+".start.release"), []byte("release\n"), 0o600); err != nil {
			return time.Time{}, err
		}
	}
	return time.Now(), nil
}

func (observer dockerObserver) measureProgress(ctx context.Context, service string, started time.Time,
	duration time.Duration) ([]progressSample, time.Time, error) {
	values := []progressSample{{}}
	deadline := started.Add(duration)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, time.Time{}, ctx.Err()
		case <-time.After(time.Second):
		}
		delivered, err := observer.currentStressProgress(ctx, service)
		if err != nil {
			return nil, time.Time{}, err
		}
		values = append(values, progressSample{AtNanos: time.Since(started).Nanoseconds(), Delivered: delivered})
	}
	return values, time.Now(), nil
}

func (observer dockerObserver) currentStressProgress(ctx context.Context, service string) (uint32, error) {
	raw, err := observer.compose(ctx, 10*time.Second, "logs", "--no-color", "--no-log-prefix", service)
	if err != nil {
		return 0, err
	}
	var highest uint32
	for _, line := range splitLines(raw) {
		var value struct {
			Schema        string `json:"schema"`
			ReceivedBytes uint32 `json:"received_bytes"`
		}
		if json.Unmarshal(line, &value) == nil &&
			(value.Schema == "ardents-h3-stream-progress-v1" || value.Schema == "ardents-h3-s43-direct-progress-v1") {
			highest = max(highest, value.ReceivedBytes)
		}
	}
	return highest, nil
}

func observeDirectProcesses(ctx context.Context, observer dockerObserver, scope hostScopeEvidence, clock time.Time,
	identities map[string]string) (map[string]processObservationEvidence, error) {
	adapter := newDockerProcessAdapter(observer, scope, clock)
	result := make(map[string]processObservationEvidence, 2)
	for _, role := range []string{"direct-client", "direct-publisher"} {
		value, err := observeProcessEvidence(ctx, adapter, role, role)
		if err != nil || value.Host.Identity != identities[role] {
			return nil, errors.Join(err, fmt.Errorf("observe %s process", role))
		}
		result[role] = value
	}
	return result, nil
}

func (observer dockerObserver) directTerminal(ctx context.Context, service string) (applicationEvidence, error) {
	raw, err := observer.compose(ctx, time.Minute, "logs", "--no-color", "--no-log-prefix", service)
	if err != nil {
		return applicationEvidence{}, err
	}
	var terminal struct {
		Schema      string
		Observation applicationEvidence
	}
	for _, line := range splitLines(raw) {
		if json.Unmarshal(line, &terminal) == nil && terminal.Schema == "ardents-h3-s43-direct-application-v1" &&
			terminal.Observation.Terminal != "" {
			return terminal.Observation, nil
		}
	}
	return applicationEvidence{}, errors.New("S4.3 direct terminal evidence is missing")
}
