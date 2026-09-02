package contributor_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/contributor"
)

func TestConcurrentWithdrawDoesNotStopOrClaimCommittedSuccessor(t *testing.T) {
	hostRoot := t.TempDir()
	supervisor := &profileSupervisor{
		hostRoot:    hostRoot,
		stopEntered: make(chan struct{}),
		releaseStop: make(chan struct{}),
	}
	var releaseStop sync.Once
	releaseWithdraw := func() { releaseStop.Do(func() { close(supervisor.releaseStop) }) }
	withdrawer, err := contributor.Open(contributor.Config{Root: hostRoot, Supervisor: supervisor})
	if err != nil {
		t.Fatal(err)
	}
	successor, err := contributor.Open(contributor.Config{Root: hostRoot, Supervisor: supervisor})
	if err != nil {
		t.Fatal(err)
	}
	deployment := strings.Repeat("46", 32)
	first, firstPin := writeContributorBundle(t, 1, deployment)
	if _, err := withdrawer.Apply(t.Context(), first, firstPin); err != nil {
		t.Fatal(err)
	}

	type controlResult struct {
		report contributor.Report
		err    error
	}
	withdrawDone := make(chan controlResult, 1)
	withdrawJoined := false
	var withdrawn controlResult
	withdrawCtx, cancelWithdraw := context.WithTimeout(t.Context(), 5*time.Second)
	go func() {
		report, controlErr := withdrawer.Control(withdrawCtx, contributor.Withdraw, "")
		withdrawDone <- controlResult{report: report, err: controlErr}
	}()
	applyCtx, cancelApply := context.WithTimeout(t.Context(), 5*time.Second)
	defer func() {
		cancelApply()
		cancelWithdraw()
		releaseWithdraw()
		if !withdrawJoined {
			withdrawn = <-withdrawDone
			withdrawJoined = true
		}
	}()
	select {
	case <-supervisor.stopEntered:
	case <-time.After(5 * time.Second):
		t.Error("withdraw did not reach the Supervisor stop boundary")
		return
	}

	second, secondPin := writeContributorBundle(t, 2, deployment)
	concurrent, concurrentErr := successor.Apply(applyCtx, second, secondPin)
	busy := concurrentErr != nil && strings.Contains(concurrentErr.Error(), "contributor root is busy")
	if !busy {
		t.Errorf("concurrent successor apply = %+v, %v; want contributor root is busy", concurrent, concurrentErr)
	}
	if supervisor.stopCount() != 0 {
		t.Errorf("concurrent successor reached destructive Supervisor stop before withdraw released the root")
	}
	if !busy {
		return
	}
	releaseWithdraw()

	select {
	case withdrawn = <-withdrawDone:
		withdrawJoined = true
	case <-time.After(5 * time.Second):
		t.Error("paused withdraw did not return after release")
	}
	if !withdrawJoined || withdrawn.err != nil {
		if withdrawn.err != nil {
			t.Errorf("withdraw with a rejected concurrent successor: %v", withdrawn.err)
		}
		return
	}
	applied, err := successor.Apply(t.Context(), second, secondPin)
	if err != nil {
		t.Fatalf("successor apply after withdraw released the root: %v", err)
	}
	if applied.DeploymentID != deployment || applied.Generation != 2 || applied.ManifestDigest != secondPin || !applied.Active || applied.LifecycleState != "READY" {
		t.Fatalf("successor apply report = %+v", applied)
	}
	diagnosed, err := successor.Control(t.Context(), contributor.Diagnose, "")
	if err != nil {
		t.Errorf("diagnose after concurrent commands: %v", err)
	} else if diagnosed.DeploymentID != deployment || diagnosed.Generation != 2 || diagnosed.ManifestDigest != secondPin || !diagnosed.Active || diagnosed.LifecycleState != "READY" {
		t.Errorf("successor was changed by the earlier withdraw: %+v", diagnosed)
	}
}
