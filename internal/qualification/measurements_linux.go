//go:build linux

package qualification

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/resource"
)

// Publisher admits at most four new Introduction openings in a rolling second.
// Qualification waits 300 ms after each remote delivery result before admitting
// another, so preparation and network jitter cannot bunch Publisher openings.
const (
	IntroductionSpacing = 300 * time.Millisecond
	SetupLimit          = 15
)

// Measurements owns the complete local runner process and
// every verified worker launched by that runner. Sharing it across participants
// prevents a Reader observation from silently omitting sibling worker processes.
// A completion barrier keeps every worker present until all participants finish.
// Retiring a worker invalidates later observations: its removed cgroup cannot
// provide a trustworthy final CPU counter. Completed measurement windows remain
// usable; overlapping windows must finish before any sibling is retired.
type Measurements struct {
	mu, sampleMu       sync.Mutex
	openingOnce        sync.Once
	setupOnce          sync.Once
	workers            map[string]bool
	retired            bool
	expected, finished int
	ready              chan struct{}
	nextOpening        time.Time
	openingSlot        chan struct{}
	setupSlots         chan struct{}
	sampledAt          time.Time
	hostSample         resource.HostingSample
	usageSample        resource.Sample
}

func (owner *Measurements) AcquireIntroductionSetup(ctx context.Context) (func(), error) {
	if owner == nil || ctx == nil {
		return nil, errors.New("qualification Introduction setup canceled")
	}
	if err := ctx.Err(); err != nil {
		return nil, errors.Join(errors.New("qualification Introduction setup canceled"), err)
	}
	owner.setupOnce.Do(func() { owner.setupSlots = make(chan struct{}, SetupLimit) })
	select {
	case owner.setupSlots <- struct{}{}:
	case <-ctx.Done():
		return nil, errors.Join(errors.New("qualification Introduction setup canceled"), ctx.Err())
	}
	var once sync.Once
	return func() { once.Do(func() { <-owner.setupSlots }) }, nil
}

// AcquireIntroductionOpening holds the shared qualification slot through the
// remote delivery result. Spacing begins at release, so variable preparation
// and network delay cannot bunch Publisher admissions into a rolling second.
func (owner *Measurements) AcquireIntroductionOpening(ctx context.Context) (func(), error) {
	if owner == nil || ctx == nil {
		return nil, errors.New("qualification Introduction pacing unavailable")
	}
	if err := ctx.Err(); err != nil {
		return nil, errors.Join(errors.New("qualification Introduction pacing canceled"), err)
	}
	owner.openingOnce.Do(func() { owner.openingSlot = make(chan struct{}, 1) })
	select {
	case owner.openingSlot <- struct{}{}:
	case <-ctx.Done():
		return nil, errors.Join(errors.New("qualification Introduction pacing canceled"), ctx.Err())
	}
	next := owner.nextOpening
	if wait := time.Until(next); wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			<-owner.openingSlot
			return nil, errors.Join(errors.New("qualification Introduction pacing canceled"), ctx.Err())
		}
	}
	if err := ctx.Err(); err != nil {
		<-owner.openingSlot
		return nil, errors.Join(errors.New("qualification Introduction pacing canceled"), err)
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			owner.nextOpening = time.Now().Add(IntroductionSpacing)
			<-owner.openingSlot
		})
	}, nil
}

func (owner *Measurements) Sample(ctx context.Context, host *resource.Hosting) (resource.HostingSample, resource.Sample, error) {
	owner.sampleMu.Lock()
	defer owner.sampleMu.Unlock()
	if owner.sampledAt.IsZero() || time.Since(owner.sampledAt) >= 900*time.Millisecond {
		if err := owner.sampleLocked(ctx, host); err != nil {
			return resource.HostingSample{}, resource.Sample{}, err
		}
	}
	hostSample := owner.hostSample
	hostSample.Interfaces = append([]resource.HostingInterfaceSample(nil), hostSample.Interfaces...)
	return hostSample, owner.usageSample, nil
}

func (owner *Measurements) SampleFresh(ctx context.Context, host *resource.Hosting) (resource.HostingSample, resource.Sample, error) {
	owner.sampleMu.Lock()
	defer owner.sampleMu.Unlock()
	if err := owner.sampleLocked(ctx, host); err != nil {
		return resource.HostingSample{}, resource.Sample{}, err
	}
	hostSample := owner.hostSample
	hostSample.Interfaces = append([]resource.HostingInterfaceSample(nil), hostSample.Interfaces...)
	return hostSample, owner.usageSample, nil
}

func (owner *Measurements) sampleLocked(ctx context.Context, host *resource.Hosting) error {
	hostSample, hostErr := host.Sample(ctx, 0)
	usageSample, usageErr := owner.measure()
	if err := errors.Join(hostErr, usageErr); err != nil {
		return err
	}
	owner.sampledAt, owner.hostSample, owner.usageSample = time.Now(), hostSample, usageSample
	return nil
}

// RegisterWorker adds one verified cgroup to the shared measurement window.
// The returned release retires that exact worker and invalidates later samples.
func (owner *Measurements) RegisterWorker(group string) (func(), error) {
	if owner == nil {
		return nil, errors.New("qualification owner worker registration invalid")
	}
	if err := owner.addWorker(group); err != nil {
		return nil, err
	}
	var once sync.Once
	return func() { once.Do(func() { owner.retireWorker(group) }) }, nil
}

func (owner *Measurements) addWorker(group string) error {
	owner.sampleMu.Lock()
	defer owner.sampleMu.Unlock()
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.retired || group == "" || owner.workers[group] {
		return errors.New("qualification owner worker registration invalid")
	}
	if owner.workers == nil {
		owner.workers = make(map[string]bool)
	}
	owner.workers[group] = true
	owner.sampledAt = time.Time{}
	return nil
}

func (owner *Measurements) retireWorker(group string) {
	owner.sampleMu.Lock()
	defer owner.sampleMu.Unlock()
	owner.mu.Lock()
	defer owner.mu.Unlock()
	delete(owner.workers, group)
	owner.retired = true
	owner.sampledAt = time.Time{}
}

func (owner *Measurements) measure() (resource.Sample, error) {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.retired {
		return resource.Sample{}, errors.New("qualification owner lost a worker before measurement finished")
	}
	groups := make([]string, 0, len(owner.workers))
	for group := range owner.workers {
		groups = append(groups, group)
	}
	return resource.MeasureOwnerCgroups(groups)
}

// NewMeasurements fixes the number of local participants
// before launch. Runtime callers cannot add participants to an active window.
func NewMeasurements(participants int) (*Measurements, error) {
	if participants < 1 || participants > 5 {
		return nil, errors.New("qualification participant count invalid")
	}
	return &Measurements{expected: participants, ready: make(chan struct{})}, nil
}

func (owner *Measurements) Finish(ctx context.Context) error {
	owner.mu.Lock()
	if owner.ready == nil || owner.finished >= owner.expected {
		owner.mu.Unlock()
		return errors.New("qualification measurement barrier invalid")
	}
	owner.finished++
	if owner.finished == owner.expected {
		close(owner.ready)
	}
	ready := owner.ready
	owner.mu.Unlock()
	select {
	case <-ready:
		return nil
	case <-ctx.Done():
		return errors.Join(errors.New("qualification owner completion barrier interrupted"), ctx.Err())
	}
}
