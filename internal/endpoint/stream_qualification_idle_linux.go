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
	// Started and Stopped are Hosting wall timestamps for log correlation only.
	Started time.Time
	Stopped time.Time
	// MeasuredDuration conservatively bounds the actual counter interval with
	// one host's monotonic clock; it excludes the boundary sampling calls.
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
	work, termination := streamQualificationIdleTraffic(window)
	reservation, err := host.Reserve(lifetime, work, termination, time.Now().Add(window+2*time.Minute))
	if err != nil {
		return report, err
	}
	defer func() {
		release, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		outcome = errors.Join(outcome, reservation.Release(release))
	}()
	outcome = withParticipant(lifetime, config.Participant, func(endpoint *endpoint) (operationErr error) {
		principal := config.Participant.ConnectionPrincipal
		capability, err := endpoint.Admit(principal, broker.Connection)
		if err != nil {
			return err
		}
		owner, err := endpoint.beginDutyContext(lifetime, capability, principal, broker.Connection)
		if err != nil {
			return err
		}
		defer func() { operationErr = errors.Join(operationErr, owner.Close()) }()
		permission := config.Participant.ReaderPermission
		if err := owner.provisionPermission(lifetime, permission.RequestPath, permission.ResponsePath, permission.Maxima, func(reportCtx context.Context, digest [32]byte) error {
			return config.Participant.Observe(reportCtx, ClosedParticipantEvent{Kind: "permission-required", NetworkID: endpoint.network, Surface: string(broker.Connection), RequestDigest: digest})
		}); err != nil {
			return err
		}
		connection, err := owner.openConnection()
		if err != nil {
			return err
		}
		defer func() { operationErr = errors.Join(operationErr, connection.Close()) }()
		if err := config.Observe(lifetime, StreamQualificationEvent{Kind: "idle-ready"}); err != nil {
			return err
		}
		return observeStreamQualificationIdle(lifetime, window, &report,
			func(sampleCtx context.Context, fresh bool) (resource.HostingSample, resource.Sample, error) {
				if fresh {
					return config.Measurements.SampleFresh(sampleCtx, host)
				}
				return config.Measurements.Sample(sampleCtx, host)
			}, config.Observe, time.Now)
	})
	return report, outcome
}

// streamQualificationIdleTraffic receives the validated positive observation
// window. Cancel the 1e9 scale against the day's nanoseconds before dividing:
// this retains floor(window*1e9/day)+1, including its extra byte at exact
// multiples, without overflow or loss of subsecond duration precision.
func streamQualificationIdleTraffic(window time.Duration) (work, termination resource.HostingTraffic) {
	proportional := uint64(window)/uint64(24*time.Hour/time.Second) + 1
	return resource.HostingTraffic{Tx: proportional, Rx: proportional}, resource.HostingTraffic{Tx: 8 << 20, Rx: 8 << 20}
}

// observeStreamQualificationIdle brackets fresh counter reads conservatively:
// the first completion is after every first interface read, and the last start
// is before every final interface read. Neither provisioning nor the final
// observation callback can enlarge the denominator. The wall timestamps remain
// the Hosting continuity owner's observations and grant no elapsed-time proof.
func observeStreamQualificationIdle(ctx context.Context, window time.Duration, report *StreamQualificationIdleReport,
	read func(context.Context, bool) (resource.HostingSample, resource.Sample, error),
	observe func(context.Context, StreamQualificationEvent) error, now func() time.Time) error {
	origin := now()
	var firstFinished time.Time
	sample := func(fresh bool, first bool) (resource.HostingSample, time.Time, error) {
		began := now()
		hostSample, usage, err := read(ctx, fresh)
		finished := now()
		if err != nil {
			return resource.HostingSample{}, began, err
		}
		if first {
			firstFinished = finished
		}
		if err := observe(ctx, StreamQualificationEvent{Kind: "resource-sample", Elapsed: began.Sub(origin), Host: &hostSample, Usage: &usage}); err != nil {
			return resource.HostingSample{}, began, err
		}
		if hostSample.Observation.Drain {
			return resource.HostingSample{}, began, errors.New("NET-32 hosting allowance requires drain")
		}
		report.Samples++
		return hostSample, began, nil
	}
	first, _, err := sample(true, true)
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
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if _, _, err := sample(false, false); err != nil {
				return err
			}
		case <-timer.C:
			last, lastBegan, err := sample(true, false)
			if err != nil {
				return err
			}
			report.Stopped = last.At
			report.MeasuredDuration = lastBegan.Sub(firstFinished)
			if report.MeasuredDuration <= 0 {
				return errors.New("NET-32 monotonic counter interval unavailable")
			}
			return nil
		}
	}
}
