package contributor_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/contributor"
)

func TestConcurrentDiagnoseCannotBypassWithdrawRootLease(t *testing.T) {
	hostRoot := t.TempDir()
	supervisor := &profileSupervisor{
		hostRoot:    hostRoot,
		stopEntered: make(chan struct{}),
		releaseStop: make(chan struct{}),
	}
	bundle, pin := writeContributorBundle(t, 1, strings.Repeat("46", 32))
	installRetainedContributorFixture(t, hostRoot, bundle, pin, supervisor)
	withdrawer, err := contributor.Open(contributor.Config{Root: hostRoot, Supervisor: supervisor})
	if err != nil {
		t.Fatal(err)
	}
	observer, err := contributor.Open(contributor.Config{Root: hostRoot, Supervisor: supervisor})
	if err != nil {
		t.Fatal(err)
	}

	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(supervisor.releaseStop) }) }
	type result struct {
		report contributor.Report
		err    error
	}
	done := make(chan result, 1)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer func() { cancel(); release() }()
	go func() {
		report, controlErr := withdrawer.Control(ctx, contributor.Withdraw, "")
		done <- result{report, controlErr}
	}()
	select {
	case <-supervisor.stopEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("withdraw did not reach Supervisor stop boundary")
	}
	if report, err := observer.Control(t.Context(), contributor.Diagnose, ""); err == nil || !strings.Contains(err.Error(), "contributor root is busy") {
		t.Fatalf("concurrent diagnose = %+v, %v; want busy root", report, err)
	}
	if supervisor.stopCount() != 0 {
		t.Fatal("concurrent Diagnose reached Supervisor while Withdraw owns the root")
	}
	release()
	select {
	case withdrawn := <-done:
		if withdrawn.err != nil || withdrawn.report.Active || withdrawn.report.Enabled || withdrawn.report.LifecycleState != "WITHDRAWN" {
			t.Fatalf("withdraw = %+v, %v", withdrawn.report, withdrawn.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("withdraw did not finish after Supervisor release")
	}
	diagnosed, err := observer.Control(t.Context(), contributor.Diagnose, "")
	if err != nil || diagnosed.Active || diagnosed.Enabled || diagnosed.LifecycleState != "WITHDRAWN" {
		t.Fatalf("diagnose after withdraw = %+v, %v", diagnosed, err)
	}
	if supervisor.startCount() != 0 {
		t.Fatal("concurrent retained operations started Contributor")
	}
}
