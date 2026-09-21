//go:build linux

package endpoint

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
	"github.com/dianabuilds/ardents-network/internal/resource"
	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

// StreamQualificationConfig belongs only to the qualification runner. It
// contains normal trusted participant configuration, never a worker selector,
// privilege receipt, network bypass or caller-provided worker identity.
type StreamQualificationConfig struct {
	Participant  TextParticipantConfig
	Role         streamqualification.Role
	Profile      streamqualification.Profile
	Condition    streamqualification.NetworkCondition
	Seed         [32]byte
	ReaderIndex  int
	Link         string
	HostingRoot  string
	Measurements *StreamQualificationMeasurements
	Observe      func(context.Context, StreamQualificationEvent) error
}

type StreamQualificationEvent struct {
	Artifact *StreamQualificationArtifact
	Elapsed  time.Duration
	Host     *resource.HostingSample
	Usage    *resource.Sample
	Kind     string
	Link     string
	Report   *streamqualification.Report
}

// RunStreamQualification exercises the real installed-worker and Service
// boundary for one fixed workload. Its only command caller is the separate
// qualification runner; ordinary text commands cannot reach this operation.
func RunStreamQualification(ctx context.Context, config StreamQualificationConfig) (report streamqualification.Report, outcome error) {
	origin := time.Now()
	defer func() {
		if !report.Started.IsZero() {
			report.StartedElapsed = report.Started.Sub(origin)
		}
		if !report.Stopped.IsZero() {
			report.StoppedElapsed = report.Stopped.Sub(origin)
		}
		if outcome != nil {
			report.Failure = outcome.Error()
		}
	}()
	if ctx == nil || ctx.Err() != nil || config.Observe == nil || config.Seed == [32]byte{} || config.HostingRoot == "" || config.Measurements == nil {
		return report, errors.New("qualification configuration unavailable")
	}
	qualification, err := newTextQualificationRun(config.Role, config.Profile, config.Seed)
	if err != nil {
		return report, err
	}
	schedule, err := config.Profile.Definition(config.Role)
	if err != nil {
		return report, err
	}
	if _, err := config.Condition.CarrierRatioLimit(); err != nil {
		return report, err
	}
	if err := config.Participant.validate(); err != nil {
		return report, err
	}
	if config.Role == streamqualification.ReaderRole && (config.ReaderIndex < 0 || config.ReaderIndex >= 4) {
		return report, errors.New("qualification Reader index invalid")
	}
	var destination targetlink.Link
	if config.Role == streamqualification.ReaderRole {
		destination, err = targetlink.Decode(config.Link)
		if err != nil || destination.Network != config.Participant.Network.NetworkID {
			return report, errors.New("qualification Target Link invalid")
		}
	} else if config.Link != "" {
		return report, errors.New("qualification Publisher does not accept a destination")
	}
	host, err := resource.OpenHosting(config.HostingRoot)
	if err != nil {
		return report, err
	}
	defer func() { outcome = errors.Join(outcome, host.Close()) }()
	lifetime, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()
	// Reserve hard Service maxima, Carrier overhead and finite termination for
	// this whole owner before any qualification or network effect. The shared
	// ledger charges all roles and processes against the same installed period.
	work := uint64(schedule.OpenConnections) * (64 << 20) * 2
	reservation, err := host.Reserve(lifetime, resource.HostingTraffic{Tx: work, Rx: work}, resource.HostingTraffic{Tx: 8 << 20, Rx: 8 << 20}, time.Now().Add(20*time.Minute))
	if err != nil {
		return report, err
	}
	defer func() {
		release, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		outcome = errors.Join(outcome, reservation.Release(release))
	}()
	monitorCtx, stopMonitoring := context.WithCancel(lifetime)
	monitor := make(chan error, 1)
	sampleOwner := func(sampleCtx context.Context) error {
		sample, usage, err := config.Measurements.sample(sampleCtx, host)
		if err != nil {
			return err
		}
		if err := config.Observe(sampleCtx, StreamQualificationEvent{Elapsed: time.Since(origin), Kind: "resource-sample", Host: &sample, Usage: &usage}); err != nil {
			return err
		}
		if sample.Observation.Drain {
			return errors.New("qualification hosting allowance requires drain")
		}
		return nil
	}
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-monitorCtx.Done():
				monitor <- nil
				return
			case <-ticker.C:
				if err := sampleOwner(monitorCtx); err != nil {
					if monitorCtx.Err() != nil {
						monitor <- nil
						return
					}
					monitor <- err
					cancel()
					return
				}
			}
		}
	}()
	var samplingOnce sync.Once
	var samplingErr error
	stopSamples := func() error {
		samplingOnce.Do(func() { stopMonitoring(); samplingErr = <-monitor })
		return samplingErr
	}
	defer func() { outcome = errors.Join(outcome, stopSamples()) }()
	// Establish a pre-work interface and owner baseline even when local setup is
	// faster than the one-second sampling cadence.
	if err := sampleOwner(lifetime); err != nil {
		return report, err
	}
	outcome = withTextParticipant(lifetime, config.Participant, func(endpoint *endpoint) (operationErr error) {
		principal, surface, permission := config.Participant.ConnectionPrincipal, broker.Connection, config.Participant.ReaderPermission
		if config.Role == streamqualification.PublisherRole {
			principal, surface, permission = config.Participant.AdministrationPrincipal, broker.Administration, config.Participant.PublisherPermission
		}
		capability, err := endpoint.Admit(principal, surface)
		if err != nil {
			return err
		}
		owner, err := endpoint.beginTextContext(lifetime, capability, principal, surface)
		if err != nil {
			return err
		}
		defer func() { operationErr = errors.Join(operationErr, owner.Close()) }()
		worker, err := owner.launchStreamQualificationWorker(lifetime, qualification)
		if err != nil {
			return err
		}
		defer func() { operationErr = errors.Join(operationErr, worker.Close()) }()
		if err := config.Observe(lifetime, StreamQualificationEvent{Elapsed: time.Since(origin), Kind: "verified-worker", Artifact: worker.lifetime.qualificationArtifact()}); err != nil {
			return err
		}
		observe := func(observeCtx context.Context, snapshot streamqualification.Report) error {
			snapshot.StartedElapsed = snapshot.Started.Sub(origin)
			return config.Observe(observeCtx, StreamQualificationEvent{Elapsed: time.Since(origin), Kind: "stream-progress", Report: &snapshot})
		}
		if err := config.Measurements.add(worker.lifetime.cgroup); err != nil {
			return err
		}
		defer config.Measurements.retire(worker.lifetime.cgroup)
		stopQualificationSampling := func() error {
			stopErr := stopSamples()
			finalCtx, finish := context.WithTimeout(context.Background(), time.Second)
			defer finish()
			sampleErr := sampleOwner(finalCtx)
			finish()
			barrierCtx, stopBarrier := context.WithTimeout(lifetime, 30*time.Second)
			defer stopBarrier()
			return errors.Join(stopErr, sampleErr, config.Measurements.finish(barrierCtx))
		}
		if err := qualification.configure(&report, config.Measurements.acquireIntroductionOpening,
			config.Measurements.acquireIntroductionSetup, stopQualificationSampling, observe); err != nil {
			return err
		}
		if err := owner.provisionTextPermission(lifetime, permission.RequestPath, permission.ResponsePath, permission.Maxima, func(reportCtx context.Context, digest [32]byte) error {
			return config.Participant.Observe(reportCtx, TextParticipantEvent{Kind: "permission-required", NetworkID: endpoint.network, Surface: string(surface), RequestDigest: digest})
		}); err != nil {
			return err
		}
		if config.Role == streamqualification.ReaderRole {
			report, err = worker.runQualificationReader(lifetime, destination, config.ReaderIndex)
			return err
		}
		run, err := worker.startPublication(lifetime)
		if err != nil {
			return err
		}
		link, err := targetlink.Encode(run.link)
		if err != nil {
			return errors.Join(err, run.Close())
		}
		if err := config.Observe(lifetime, StreamQualificationEvent{Kind: "publisher-ready", Link: link}); err != nil {
			return errors.Join(err, run.Close())
		}
		select {
		case <-run.done:
			return run.err
		case <-lifetime.Done():
			return errors.Join(lifetime.Err(), run.Close())
		}
	})
	return report, outcome
}
