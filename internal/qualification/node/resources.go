package node

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
)

func (observer *nodeObserver) sampleNodeResources(ctx context.Context, scheduled time.Time) ([]nodeResourceSnapshot, error) {
	services := []string{"source1", "source2", "endpoint", "node1", "node2"}
	identities, err := observer.composeBounded(ctx, 4096, "ps", "--format", "{{.Service}}\t{{.ID}}")
	if err != nil {
		return nil, err
	}
	ids := make(map[string]string, len(services))
	for _, line := range strings.Split(string(identities), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && len(fields[1]) >= 12 && len(fields[1]) <= 64 {
			ids[fields[0]] = fields[1]
		}
	}
	arguments := []string{"inspect", "--format", "{{.Id}}\t{{.State.Pid}}"}
	for _, service := range services {
		if ids[service] != "" {
			arguments = append(arguments, ids[service])
		}
	}
	if len(arguments) == 3 {
		return nil, errors.New("node resource candidate set is empty")
	}
	inspected, err := observer.dockerBounded(ctx, 8192, 4096, arguments...)
	if err != nil {
		return nil, err
	}
	candidates := make([]nodeHostCandidate, 0, len(services))
	for _, service := range services {
		if ids[service] == "" {
			continue
		}
		candidate, found, candidateErr := nodeCandidateFromInspect(service, ids[service], string(inspected))
		if candidateErr != nil {
			return nil, candidateErr
		}
		if !found {
			continue
		}
		candidates = append(candidates, candidate)
	}
	if len(candidates) == 0 {
		return []nodeResourceSnapshot{}, nil
	}
	payload, err := json.Marshal(candidates)
	if err != nil {
		return nil, fmt.Errorf("encode node resource collector input: %w", err)
	}
	if len(payload) > 4096 {
		return nil, errors.New("node resource collector input exceeds its bound")
	}
	if len(observer.collectorID) < 12 {
		return nil, errors.New("node resource collector is unavailable")
	}
	raw, err := observer.dockerBounded(ctx, 384<<10, 4096, "exec", observer.collectorID,
		"/usr/local/bin/ardents-qualify", "sample-node", string(payload))
	if err != nil {
		return nil, err
	}
	var samples []nodeResourceSnapshot
	if err := json.Unmarshal(raw, &samples); err != nil {
		return nil, invalidNodeCampaign(fmt.Errorf("decode node host resource sample: %w", err))
	}
	if len(samples) > len(candidates) {
		return nil, invalidNodeCampaign(errors.New("node host resource sample count is invalid"))
	}
	for index := range samples {
		samples[index].TickDelayNanos = int64(samples[index].At.Sub(scheduled))
	}
	return samples, nil
}

func (observer *nodeObserver) captureInitialResources(ctx context.Context) error {
	samples, err := observer.sampleNodeResources(ctx, time.Now())
	if err != nil {
		return err
	}
	if len(samples) != 5 {
		return invalidNodeCampaign(errors.New("node initial resource oracle did not observe every candidate"))
	}
	seen := make(map[string]bool, 5)
	for _, sample := range samples {
		seen[sample.Service] = true
	}
	for _, service := range []string{"source1", "source2", "endpoint", "node1", "node2"} {
		if !seen[service] {
			return errors.New("node initial resource oracle missed " + service)
		}
	}
	return byteio.WriteJSON(filepath.Join(observer.input.EvidenceRoot, "resources-initial.json"), samples, 64<<10)
}
