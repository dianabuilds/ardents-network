//go:build linux

package qualification

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
	hostingbudget "github.com/dianabuilds/ardents-network/internal/hosting"
	"github.com/dianabuilds/ardents-network/internal/resource"
	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

// Event is one scenario observation. The qualification runner records these;
// the scenario owner emits them at the original positions.
type Event struct {
	Artifact *Artifact
	Elapsed  time.Duration
	Host     *hostingbudget.Sample
	Usage    *resource.Sample
	Kind     string
	Link     string
	Report   *streamqualification.Report
}

// Scenario is one fixed measured workload. Endpoint composes it from its
// trusted participant configuration and supplies the participant authority:
// ValidateParticipant inspects the composed participant, and WithSession binds
// launch authority to one authorized Session for the measured run.
type Scenario struct {
	Role                streamqualification.Role
	Profile             streamqualification.Profile
	Condition           streamqualification.NetworkCondition
	Seed                [32]byte
	ReaderIndex         int
	Link                string
	NetworkID           [32]byte
	HostingRoot         string
	Measurements        *Measurements
	Observe             func(context.Context, Event) error
	ValidateParticipant func() error
	WithSession         func(ctx context.Context, run *Run, use func(Session) error) error
}

// RunScenario exercises the real installed-worker and Service boundary for
// one fixed workload through the Endpoint-supplied authorized Session. Its
// only caller composes the participant in Endpoint; ordinary text commands
// cannot reach this operation.
func RunScenario(ctx context.Context, scenario Scenario) (report streamqualification.Report, outcome error) {
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
	if ctx == nil || ctx.Err() != nil || scenario.Observe == nil || scenario.Seed == [32]byte{} || scenario.HostingRoot == "" || scenario.Measurements == nil || scenario.ValidateParticipant == nil || scenario.WithSession == nil {
		return report, errors.New("qualification configuration unavailable")
	}
	run, err := NewRun(scenario.Role, scenario.Profile, scenario.Seed)
	if err != nil {
		return report, err
	}
	schedule, err := scenario.Profile.Definition(scenario.Role)
	if err != nil {
		return report, err
	}
	if _, err := scenario.Condition.CarrierRatioLimit(); err != nil {
		return report, err
	}
	if err := scenario.ValidateParticipant(); err != nil {
		return report, err
	}
	if scenario.Role == streamqualification.ReaderRole && (scenario.ReaderIndex < 0 || scenario.ReaderIndex >= 4) {
		return report, errors.New("qualification Reader index invalid")
	}
	var destination targetlink.Link
	if scenario.Role == streamqualification.ReaderRole {
		destination, err = targetlink.Decode(scenario.Link)
		if err != nil || destination.Network != scenario.NetworkID {
			return report, errors.New("qualification Target Link invalid")
		}
	} else if scenario.Link != "" {
		return report, errors.New("qualification Publisher does not accept a destination")
	}
	host, err := hostingbudget.Open(scenario.HostingRoot)
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
	reservation, err := host.Reserve(lifetime, hostingbudget.Traffic{Tx: work, Rx: work}, hostingbudget.Traffic{Tx: 8 << 20, Rx: 8 << 20}, time.Now().Add(20*time.Minute))
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
		sample, usage, err := scenario.Measurements.Sample(sampleCtx, host)
		if err != nil {
			return err
		}
		if err := scenario.Observe(sampleCtx, Event{Elapsed: time.Since(origin), Kind: "resource-sample", Host: &sample, Usage: &usage}); err != nil {
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
	outcome = scenario.WithSession(lifetime, run, func(session Session) error {
		if err := scenario.Observe(lifetime, Event{Elapsed: time.Since(origin), Kind: "verified-worker", Artifact: session.Artifact()}); err != nil {
			return err
		}
		observe := func(observeCtx context.Context, snapshot streamqualification.Report) error {
			snapshot.StartedElapsed = snapshot.Started.Sub(origin)
			return scenario.Observe(observeCtx, Event{Elapsed: time.Since(origin), Kind: "stream-progress", Report: &snapshot})
		}
		retireWorker, err := scenario.Measurements.RegisterWorker(session.Cgroup())
		if err != nil {
			return err
		}
		defer retireWorker()
		stopQualificationSampling := func() error {
			stopErr := stopSamples()
			finalCtx, finish := context.WithTimeout(context.Background(), time.Second)
			defer finish()
			sampleErr := sampleOwner(finalCtx)
			finish()
			barrierCtx, stopBarrier := context.WithTimeout(lifetime, 30*time.Second)
			defer stopBarrier()
			return errors.Join(stopErr, sampleErr, scenario.Measurements.Finish(barrierCtx))
		}
		if err := run.Configure(&report, scenario.Measurements.AcquireIntroductionOpening,
			scenario.Measurements.AcquireIntroductionSetup, stopQualificationSampling, observe); err != nil {
			return err
		}
		if err := session.ProvisionPermission(lifetime); err != nil {
			return err
		}
		if scenario.Role == streamqualification.ReaderRole {
			report, err = runReader(lifetime, session, destination, scenario.ReaderIndex)
			return err
		}
		publication, err := session.StartPublication(lifetime)
		if err != nil {
			return err
		}
		link, err := targetlink.Encode(publication.Link())
		if err != nil {
			return errors.Join(err, publication.Close())
		}
		if err := scenario.Observe(lifetime, Event{Kind: "publisher-ready", Link: link}); err != nil {
			return errors.Join(err, publication.Close())
		}
		select {
		case <-publication.Done():
			return publication.Err()
		case <-lifetime.Done():
			return errors.Join(lifetime.Err(), publication.Close())
		}
	})
	return report, outcome
}
