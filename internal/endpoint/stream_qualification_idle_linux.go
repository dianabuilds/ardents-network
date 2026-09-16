//go:build linux

package endpoint

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
	"github.com/dianabuilds/ardents-network/internal/resource"
)

const streamQualificationIdleWindow = 10 * time.Minute

// StreamQualificationIdleReport records the fixed short NET-32 observation.
// It is evidence for an explicitly labelled upper projection, not a claim that
// the candidate remained idle for an observed 24-hour period.
type StreamQualificationIdleReport struct {
	Started          time.Time
	Stopped          time.Time
	MeasuredDuration time.Duration
	Samples          uint32
	Failure          string
}

// RunStreamQualificationIdle observes an already provisioned ordinary User
// context without opening a Service Connection or launching an Application.
func RunStreamQualificationIdle(ctx context.Context, config StreamQualificationConfig) (report StreamQualificationIdleReport, outcome error) {
	return runStreamQualificationIdle(ctx, config, streamQualificationIdleWindow)
}

func runStreamQualificationIdle(ctx context.Context, config StreamQualificationConfig, window time.Duration) (report StreamQualificationIdleReport, outcome error) {
	defer func() {
		if outcome != nil {
			report.Failure = outcome.Error()
		}
	}()
	if ctx == nil || ctx.Err() != nil || config.Observe == nil || config.Participant.Observe == nil || config.HostingRoot == "" || config.Measurements == nil {
		return report, errors.New("NET-32 qualification configuration unavailable")
	}
	if config.Role != streamqualification.ReaderRole || config.Condition != streamqualification.NormalNetwork || config.Link != "" || window <= 0 || window > streamQualificationIdleWindow {
		return report, errors.New("NET-32 qualification shape invalid")
	}
	if err := config.Participant.validate(); err != nil {
		return report, err
	}
	host, err := resource.OpenHosting(config.HostingRoot)
	if err != nil {
		return report, err
	}
	defer func() { outcome = errors.Join(outcome, host.Close()) }()
	lifetime, cancel := context.WithTimeout(ctx, window+2*time.Minute)
	defer cancel()
	// The proportional NET-32 ceiling is reserved in either direction because
	// the provider may account tx, rx, or their sum. Termination remains separate.
	proportional := uint64(window)*1_000_000_000/uint64(24*time.Hour) + 1
	reservation, err := host.Reserve(lifetime, resource.HostingTraffic{Tx: proportional, Rx: proportional}, resource.HostingTraffic{Tx: 8 << 20, Rx: 8 << 20}, time.Now().Add(window+2*time.Minute))
	if err != nil {
		return report, err
	}
	defer func() {
		release, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		outcome = errors.Join(outcome, reservation.Release(release))
	}()
	sample := func(sampleCtx context.Context, elapsed time.Duration, fresh bool) (resource.HostingSample, error) {
		var hostSample resource.HostingSample
		var usage resource.Sample
		var err error
		if fresh {
			hostSample, usage, err = config.Measurements.sampleFresh(sampleCtx, host)
		} else {
			hostSample, usage, err = config.Measurements.sample(sampleCtx, host)
		}
		if err != nil {
			return resource.HostingSample{}, err
		}
		if err := config.Observe(sampleCtx, StreamQualificationEvent{Kind: "resource-sample", Elapsed: elapsed, Host: &hostSample, Usage: &usage}); err != nil {
			return resource.HostingSample{}, err
		}
		if hostSample.Observation.Drain {
			return resource.HostingSample{}, errors.New("NET-32 hosting allowance requires drain")
		}
		report.Samples++
		return hostSample, nil
	}
	outcome = withTextParticipant(lifetime, config.Participant, func(endpoint *endpoint) (operationErr error) {
		principal := config.Participant.ConnectionPrincipal
		capability, err := endpoint.Admit(principal, broker.Connection)
		if err != nil {
			return err
		}
		owner, err := endpoint.beginTextContext(lifetime, capability, principal, broker.Connection)
		if err != nil {
			return err
		}
		defer func() { operationErr = errors.Join(operationErr, owner.Close()) }()
		permission := config.Participant.ReaderPermission
		if err := owner.provisionTextPermission(lifetime, permission.RequestPath, permission.ResponsePath, permission.Maxima, func(reportCtx context.Context, digest [32]byte) error {
			return config.Participant.Observe(reportCtx, TextParticipantEvent{Kind: "permission-required", NetworkID: endpoint.network, Surface: string(broker.Connection), RequestDigest: digest})
		}); err != nil {
			return err
		}
		connection, err := owner.openTextConnection()
		if err != nil {
			return err
		}
		defer func() { operationErr = errors.Join(operationErr, connection.Close()) }()
		if err := config.Observe(lifetime, StreamQualificationEvent{Kind: "idle-ready"}); err != nil {
			return err
		}
		origin := time.Now()
		first, err := sample(lifetime, 0, true)
		if err != nil {
			return err
		}
		report.Started = first.At
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		timer := time.NewTimer(window)
		defer timer.Stop()
		for {
			select {
			case <-lifetime.Done():
				return lifetime.Err()
			case <-ticker.C:
				if _, err := sample(lifetime, time.Since(origin), false); err != nil {
					return err
				}
			case <-timer.C:
				last, err := sample(lifetime, time.Since(origin), true)
				if err != nil {
					return err
				}
				report.Stopped = last.At
				report.MeasuredDuration = report.Stopped.Sub(report.Started)
				return nil
			}
		}
	})
	return report, outcome
}
