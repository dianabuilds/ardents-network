package routeexperiment

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/preflight"
)

type resourceAggregate struct {
	role       string
	endpoint   bool
	peakRSS    uint64
	cpuTotal   float64
	cpuSamples int
	queueHigh  uint64
}

func runNativeCondition(ctx context.Context, identity preflight.RunLayout, manifest inputManifest, profile string) (conditionResult, error) {
	if _, _, _, _, err := identity.OwnedPaths(true, true); err != nil {
		return conditionResult{}, err
	}
	workloads := manifest.Workloads[profile]
	if len(workloads) != 26 {
		return conditionResult{}, errors.New("native condition workload schedule is incomplete")
	}
	result := conditionResult{Name: profile, CleanupPassed: true}
	resources := make(map[string]*resourceAggregate)
	for _, workload := range workloads {
		attempt, attemptErr := runNativeAttempt(ctx, identity, manifest, profile, workload)
		if attemptErr != nil && attempt.summary.Status == "" {
			return result, attemptErr
		}
		result.CleanupPassed = result.CleanupPassed && attempt.summary.Checks["cleanup_complete"]
		if workload.Kind == "setup" {
			result.Setups = append(result.Setups, setupSample{
				Passed:  attemptErr == nil && attempt.summary.Status == "passed",
				Elapsed: time.Duration(attempt.summary.SetupMilliseconds) * time.Millisecond,
			})
			continue
		}
		source := attempt.user
		if workload.Direction == directionDownload {
			source = attempt.service
		}
		elapsedSeconds := float64(source.StreamElapsedMilliseconds) / 1000
		bps := float64(0)
		if elapsedSeconds > 0 {
			bps = float64(source.ApplicationBytes*8) / elapsedSeconds
		}
		result.Streams = append(result.Streams, streamSample{
			Direction: workload.Direction, Verified: attemptErr == nil && source.ApplicationBytesVerified,
			BitsPerSecond: bps, LinkWireBytes: attempt.linkBytes,
		})
		aggregateAttemptResources(resources, attempt)
	}
	for _, aggregate := range resources {
		meanCPU := float64(0)
		if aggregate.cpuSamples > 0 {
			meanCPU = aggregate.cpuTotal / float64(aggregate.cpuSamples)
		}
		result.Resources = append(result.Resources, resourceSample{
			Role: aggregate.role, Endpoint: aggregate.endpoint, PeakRSSBytes: aggregate.peakRSS,
			MeanCPUCores: meanCPU, QueueHighBytes: aggregate.queueHigh,
		})
	}
	return result, nil
}

func aggregateAttemptResources(resources map[string]*resourceAggregate, attempt nativeAttemptEvidence) {
	for _, sample := range attempt.resources {
		aggregate := resources[sample.Role]
		if aggregate == nil {
			aggregate = &resourceAggregate{role: sample.Role, endpoint: sample.Role == "user" || sample.Role == "service"}
			resources[sample.Role] = aggregate
		}
		aggregate.peakRSS = max(aggregate.peakRSS, sample.RSSBytes)
		aggregate.cpuTotal += sample.CPUCores
		aggregate.cpuSamples++
	}
	for role, queue := range attempt.queues {
		aggregate := resources[role]
		if aggregate == nil {
			aggregate = &resourceAggregate{role: role, endpoint: role == "user" || role == "service"}
			resources[role] = aggregate
		}
		aggregate.queueHigh = max(aggregate.queueHigh, queue)
	}
}
