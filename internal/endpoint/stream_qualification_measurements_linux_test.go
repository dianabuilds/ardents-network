//go:build linux

package endpoint

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestQualificationPublisherIncompleteErrorReportsProgressAndRegistrationReason(t *testing.T) {
	err := qualificationPublisherIncompleteError(193)
	message := err.Error()
	for _, want := range []string{"193/256", "producer ended"} {
		if !strings.Contains(message, want) {
			t.Fatalf("error %q does not contain %q", message, want)
		}
	}
}

func TestQualificationIntroductionPacerSpacesCompletedDeliveries(t *testing.T) {
	owner, err := NewStreamQualificationMeasurements(4)
	if err != nil {
		t.Fatal(err)
	}
	release, err := owner.acquireIntroductionOpening(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	acquired := make(chan time.Time, 1)
	failures := make(chan error, 1)
	go func() {
		nextRelease, err := owner.acquireIntroductionOpening(t.Context())
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
		if at.Sub(finished) < streamQualificationIntroductionSpacing-20*time.Millisecond {
			t.Fatalf("Publisher deliveries separated by %v, want at least %v", at.Sub(finished), streamQualificationIntroductionSpacing)
		}
	case err := <-failures:
		t.Fatal(err)
	case <-time.After(2 * time.Second):
		t.Fatal("second Publisher delivery did not enter after spacing")
	}
}

func TestQualificationIntroductionPacerCancelsWaitingDelivery(t *testing.T) {
	owner, err := NewStreamQualificationMeasurements(4)
	if err != nil {
		t.Fatal(err)
	}
	release, err := owner.acquireIntroductionOpening(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, cancel := context.WithCancel(t.Context())
	canceled := make(chan error, 1)
	go func() { _, err := owner.acquireIntroductionOpening(ctx); canceled <- err }()
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
	owner, err := NewStreamQualificationMeasurements(4)
	if err != nil {
		t.Fatal(err)
	}
	releases := make([]func(), 0, streamQualificationSetupLimit)
	for range streamQualificationSetupLimit {
		release, err := owner.acquireIntroductionSetup(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		releases = append(releases, release)
	}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := owner.acquireIntroductionSetup(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("full setup admission ignored cancellation: %v", err)
	}
	releases[0]()
	replacement, err := owner.acquireIntroductionSetup(t.Context())
	if err != nil {
		t.Fatalf("released setup admission was not reusable: %v", err)
	}
	replacement()
	for _, release := range releases[1:] {
		release()
	}
}

func TestQualificationOwnerBarrierRequiresEveryParticipant(t *testing.T) {
	owner, err := NewStreamQualificationMeasurements(2)
	if err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := owner.finish(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatalf("incomplete owner barrier: %v", err)
	}
	select {
	case <-owner.ready:
		t.Fatal("barrier released without second participant")
	default:
	}
	if err := owner.finish(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-owner.ready:
	default:
		t.Fatal("complete owner barrier did not release")
	}
	if err := owner.finish(context.Background()); err == nil {
		t.Fatal("duplicate completion accepted")
	}
}

func TestQualificationOwnerCannotHideRetiredSibling(t *testing.T) {
	owner, _ := NewStreamQualificationMeasurements(2)
	for _, group := range []string{"/system.slice/reader-a.service", "/system.slice/reader-b.service"} {
		if err := owner.add(group); err != nil {
			t.Fatal(err)
		}
	}
	owner.retire("/system.slice/reader-a.service")
	if _, err := owner.measure(); err == nil {
		t.Fatal("removed sibling was silently omitted")
	}
	if err := owner.add("/system.slice/reader-c.service"); err == nil {
		t.Fatal("replacement reset owner measurement")
	}
}

func TestQualificationUbuntu249KeepsParentExitInvariant(t *testing.T) {
	service := textManagerProperties{"RemainAfterExit": {Type: "b", Data: []byte("false")}}
	if !textEndpointStopsWithMainVersion(service, 249) {
		t.Fatal("v249 cannot represent its normal parent lifetime")
	}
	for _, version := range []uint16{0, 248, 250, 255} {
		if textEndpointStopsWithMainVersion(service, version) {
			t.Fatalf("accepted missing properties on %d", version)
		}
	}
	service["ExitType"] = textManagerValue{Type: "s", Data: []byte("\"cgroup\"")}
	if textEndpointStopsWithMainVersion(service, 249) {
		t.Fatal("unexpected cgroup lifetime accepted")
	}
	delete(service, "ExitType")
	service["RemainAfterExit"] = textManagerValue{Type: "b", Data: []byte("true")}
	if textEndpointStopsWithMainVersion(service, 249) {
		t.Fatal("retained parent accepted")
	}
}
