//go:build linux

package qualification

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestQualificationIntroductionPacerSpacesCompletedDeliveries(t *testing.T) {
	owner, err := NewMeasurements(4)
	if err != nil {
		t.Fatal(err)
	}
	release, err := owner.AcquireIntroductionOpening(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	acquired := make(chan time.Time, 1)
	failures := make(chan error, 1)
	go func() {
		nextRelease, err := owner.AcquireIntroductionOpening(t.Context())
		if err != nil {
			failures <- err
			return
		}
		acquired <- time.Now()
		nextRelease()
	}()
	select {
	case <-acquired:
		t.Fatal("second Publisher delivery entered before first finished")
	case err := <-failures:
		t.Fatal(err)
	case <-time.After(20 * time.Millisecond):
	}
	finished := time.Now()
	release()
	select {
	case at := <-acquired:
		if at.Sub(finished) < IntroductionSpacing-20*time.Millisecond {
			t.Fatalf("Publisher deliveries separated by %v, want at least %v", at.Sub(finished), IntroductionSpacing)
		}
	case err := <-failures:
		t.Fatal(err)
	case <-time.After(2 * time.Second):
		t.Fatal("second Publisher delivery did not enter after spacing")
	}
}

func TestQualificationIntroductionPacerCancelsWaitingDelivery(t *testing.T) {
	owner, err := NewMeasurements(4)
	if err != nil {
		t.Fatal(err)
	}
	release, err := owner.AcquireIntroductionOpening(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, cancel := context.WithCancel(t.Context())
	canceled := make(chan error, 1)
	go func() { _, err := owner.AcquireIntroductionOpening(ctx); canceled <- err }()
	cancel()
	select {
	case err := <-canceled:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("waiting delivery cancellation = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("waiting Publisher delivery did not cancel")
	}
}

func TestQualificationIntroductionSetupLeavesPublisherCapacity(t *testing.T) {
	owner, err := NewMeasurements(4)
	if err != nil {
		t.Fatal(err)
	}
	releases := make([]func(), 0, SetupLimit)
	for range SetupLimit {
		release, err := owner.AcquireIntroductionSetup(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		releases = append(releases, release)
	}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := owner.AcquireIntroductionSetup(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("full setup admission ignored cancellation: %v", err)
	}
	releases[0]()
	replacement, err := owner.AcquireIntroductionSetup(t.Context())
	if err != nil {
		t.Fatalf("released setup admission was not reusable: %v", err)
	}
	replacement()
	for _, release := range releases[1:] {
		release()
	}
}

func TestQualificationOwnerBarrierRequiresEveryParticipant(t *testing.T) {
	owner, err := NewMeasurements(2)
	if err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := owner.Finish(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatalf("incomplete owner barrier: %v", err)
	}
	select {
	case <-owner.ready:
		t.Fatal("barrier released without second participant")
	default:
	}
	if err := owner.Finish(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-owner.ready:
	default:
		t.Fatal("complete owner barrier did not release")
	}
	if err := owner.Finish(context.Background()); err == nil {
		t.Fatal("duplicate completion accepted")
	}
}

func TestQualificationOwnerCannotHideRetiredSibling(t *testing.T) {
	owner, _ := NewMeasurements(2)
	for _, group := range []string{"/system.slice/reader-a.service", "/system.slice/reader-b.service"} {
		if err := owner.addWorker(group); err != nil {
			t.Fatal(err)
		}
	}
	owner.retireWorker("/system.slice/reader-a.service")
	if _, err := owner.measure(); err == nil {
		t.Fatal("removed sibling was silently omitted")
	}
	if err := owner.addWorker("/system.slice/reader-c.service"); err == nil {
		t.Fatal("replacement reset owner measurement")
	}
}
