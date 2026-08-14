package recoverysmoke

import (
	"context"
	"errors"
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
			return trafficObservers{}, errors.Join(err, removeErr)
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
		"/usr/local/bin/ardents-qualify", "carrier-fault", "wait")
	identity := strings.TrimSpace(string(raw))
	if err != nil || !validContainerID(identity) {
		return "", recovery.ObserverProcess{}, errors.Join(err, errors.New("traffic observer identity is invalid"))
	}
	projection, inspectErr := observer.inspectReplacementObserver(ctx, identity)
	if inspectErr != nil {
		_, removeErr := observer.docker(context.Background(), 10*time.Second, "rm", "-f", identity)
		return "", recovery.ObserverProcess{}, errors.Join(inspectErr, removeErr)
	}
	if _, err := observer.docker(ctx, 10*time.Second, "start", identity); err != nil {
		_, removeErr := observer.docker(context.Background(), 10*time.Second, "rm", "-f", identity)
		return "", recovery.ObserverProcess{}, errors.Join(err, removeErr)
	}
	return identity, projection, nil
}

func (value *trafficObservers) snapshotAndRemove(ctx context.Context, observer dockerObserver,
	clock time.Time) (recovery.ResourceSample, error) {
	identities := map[string]string{"client-traffic": value.ids[0], "publisher-traffic": value.ids[1]}
	services := []string{"client-traffic", "publisher-traffic"}
	args := []string{"stats", "--no-stream", "--format", "{{json .}}", value.ids[0], value.ids[1]}
	raw, err := observer.docker(ctx, 10*time.Second, args...)
	var sample recovery.ResourceSample
	seen := map[string]bool{}
	for _, line := range splitLines(raw) {
		service, rowErr := addResourceRow(line, identities, services, &sample)
		if rowErr == nil && service == "" {
			continue
		}
		if rowErr != nil || seen[service] {
			err = errors.Join(err, rowErr, errors.New("final traffic observation is malformed"))
			break
		}
		seen[service] = true
	}
	sample.AtNanos = time.Since(clock).Nanoseconds()
	if len(seen) != len(services) {
		err = errors.Join(err, errors.New("final traffic observation is incomplete"))
	}
	removeErr := value.remove(context.Background(), observer)
	return sample, errors.Join(err, removeErr)
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
