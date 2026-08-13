package recoverysmoke

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/recovery"
)

type statsSampler struct {
	mu             sync.Mutex
	samples        []recovery.ResourceSample
	err            error
	cancel         context.CancelFunc
	done           chan struct{}
	stopOnce       sync.Once
	stoppedSamples []recovery.ResourceSample
	stoppedErr     error
}

func (observer dockerObserver) startStats(ctx context.Context, identities map[string]string, clock time.Time) *statsSampler {
	sampleCtx, cancel := context.WithCancel(ctx)
	result := &statsSampler{cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(result.done)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			sample, err := observer.hostResourceSample(sampleCtx, identities, clock)
			result.mu.Lock()
			if err != nil && sampleCtx.Err() == nil {
				result.err = err
			}
			if err == nil {
				result.samples = append(result.samples, sample)
			}
			result.mu.Unlock()
			select {
			case <-sampleCtx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return result
}

func (sampler *statsSampler) stop() ([]recovery.ResourceSample, error) {
	sampler.stopOnce.Do(func() {
		sampler.cancel()
		<-sampler.done
		sampler.mu.Lock()
		defer sampler.mu.Unlock()
		sampler.stoppedSamples = append([]recovery.ResourceSample(nil), sampler.samples...)
		sampler.stoppedErr = sampler.err
		if len(sampler.stoppedSamples) < 3 {
			sampler.stoppedErr = errors.Join(sampler.stoppedErr,
				errors.New("fewer than three one-second host resource samples"))
		}
	})
	return append([]recovery.ResourceSample(nil), sampler.stoppedSamples...), sampler.stoppedErr
}

func (observer dockerObserver) hostResourceSample(ctx context.Context, identities map[string]string,
	clock time.Time) (recovery.ResourceSample, error) {
	services := []string{"client-endpoint", "client-app", "client", "publisher-endpoint", "publisher-app", "publisher"}
	args := []string{"stats", "--no-stream", "--format", "{{json .}}"}
	for _, service := range services {
		args = append(args, identities[service])
	}
	raw, err := observer.docker(ctx, 30*time.Second, args...)
	if err != nil {
		return recovery.ResourceSample{}, err
	}
	var result recovery.ResourceSample
	result.AtNanos = time.Since(clock).Nanoseconds()
	for _, line := range splitLines(raw) {
		var value struct{ ID, MemUsage, CPUPerc, NetIO string }
		if json.Unmarshal(line, &value) != nil {
			return result, errors.New("docker stats row is malformed")
		}
		memory, received, sent, cpu, parseErr := parseDockerStats(value.MemUsage, value.NetIO, value.CPUPerc)
		if parseErr != nil {
			return result, parseErr
		}
		client, matched := false, false
		for _, service := range services {
			if strings.HasPrefix(identities[service], value.ID) || strings.HasPrefix(value.ID, identities[service][:12]) {
				client, matched = strings.HasPrefix(service, "client"), true
				break
			}
		}
		if !matched {
			return result, errors.New("docker stats process identity is unknown")
		}
		if client {
			result.ClientRSS += memory
			result.ClientCPUPercent += cpu
			result.ClientReceived += received
			result.ClientSent += sent
		} else {
			result.PublisherRSS += memory
			result.PublisherCPUPercent += cpu
			result.PublisherReceived += received
			result.PublisherSent += sent
		}
	}
	return result, nil
}

func parseDockerStats(memoryText, networkText, cpuText string) (uint64, uint64, uint64, float64, error) {
	memory, err := dockerSize(strings.TrimSpace(strings.Split(memoryText, "/")[0]))
	parts := strings.Split(networkText, "/")
	if err != nil || len(parts) != 2 {
		return 0, 0, 0, 0, errors.New("docker memory or network stats are malformed")
	}
	received, receiveErr := dockerSize(strings.TrimSpace(parts[0]))
	sent, sendErr := dockerSize(strings.TrimSpace(parts[1]))
	cpu, cpuErr := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(cpuText), "%"), 64)
	if receiveErr != nil || sendErr != nil || cpuErr != nil || cpu < 0 {
		return 0, 0, 0, 0, errors.New("docker CPU or traffic stats are malformed")
	}
	return memory, received, sent, cpu, nil
}

func dockerSize(value string) (uint64, error) {
	units := []struct {
		suffix string
		factor float64
	}{
		{"GiB", 1 << 30}, {"MiB", 1 << 20}, {"KiB", 1 << 10}, {"GB", 1e9}, {"MB", 1e6}, {"kB", 1e3}, {"B", 1},
	}
	for _, unit := range units {
		if strings.HasSuffix(value, unit.suffix) {
			number, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(value, unit.suffix)), 64)
			if err != nil || number < 0 {
				return 0, errors.New("docker size is invalid")
			}
			return uint64(number * unit.factor), nil
		}
	}
	return 0, errors.New("docker size unit is unknown")
}
