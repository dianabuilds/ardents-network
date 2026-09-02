package contributor_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/contributor"
)

const (
	contributorChildApplyMode = "ARDENTS_CONTRIBUTOR_CHILD_APPLY"
	contributorChildRoot      = "ARDENTS_CONTRIBUTOR_CHILD_ROOT"
	contributorChildBundle    = "ARDENTS_CONTRIBUTOR_CHILD_BUNDLE"
	contributorChildPin       = "ARDENTS_CONTRIBUTOR_CHILD_PIN"
)

func TestConcurrentWithdrawDoesNotStopOrClaimCommittedSuccessor(t *testing.T) {
	if os.Getenv(contributorChildApplyMode) == "1" {
		testConcurrentWithdrawChildApply(t)
		return
	}
	hostRoot := t.TempDir()
	supervisor := &profileSupervisor{
		hostRoot:    hostRoot,
		stopEntered: make(chan struct{}),
		releaseStop: make(chan struct{}),
	}
	t.Cleanup(func() {
		select {
		case <-supervisor.releaseStop:
		default:
			close(supervisor.releaseStop)
		}
	})
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
	go func() {
		report, controlErr := withdrawer.Control(t.Context(), contributor.Withdraw, "")
		withdrawDone <- controlResult{report: report, err: controlErr}
	}()
	select {
	case <-supervisor.stopEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("withdraw did not reach the Supervisor stop boundary")
	}

	second, secondPin := writeContributorBundle(t, 2, deployment)
	childOutput, childErr := runConcurrentWithdrawChildApply(hostRoot, second, secondPin)
	if childErr != nil {
		t.Errorf("concurrent child apply failed to run: %v\n%s", childErr, childOutput)
	} else if !strings.Contains(childOutput, "contributor root is busy") {
		t.Errorf("concurrent child apply reached the Supervisor instead of returning contributor root is busy:\n%s", childOutput)
	}
	close(supervisor.releaseStop)

	var withdrawn controlResult
	select {
	case withdrawn = <-withdrawDone:
	case <-time.After(5 * time.Second):
		t.Fatal("paused withdraw did not return after release")
	}
	if withdrawn.err != nil {
		t.Fatalf("withdraw with a rejected concurrent successor: %v", withdrawn.err)
	}
	applied, err := successor.Apply(t.Context(), second, secondPin)
	if err != nil {
		t.Fatalf("successor apply after withdraw released the root: %v", err)
	}
	if applied.Generation != 2 || !applied.Active || applied.LifecycleState != "READY" {
		t.Fatalf("successor apply report = %+v", applied)
	}
	diagnosed, err := successor.Control(t.Context(), contributor.Diagnose, "")
	if err != nil {
		t.Errorf("diagnose after concurrent commands: %v", err)
	} else if diagnosed.Generation != 2 || !diagnosed.Active || diagnosed.LifecycleState != "READY" {
		t.Errorf("successor was changed by the earlier withdraw: %+v", diagnosed)
	}
	record := readPersistedContributorInstallation(t, hostRoot)
	if record.DeploymentID != deployment || record.Generation != 2 || record.ManifestDigest != secondPin {
		t.Errorf("installation record after concurrent commands = %+v, want generation two", record)
	}
}

func runConcurrentWithdrawChildApply(hostRoot, bundle, pin string) (string, error) {
	command := exec.Command(os.Args[0], "-test.run=^TestConcurrentWithdrawDoesNotStopOrClaimCommittedSuccessor$", "-test.v")
	command.Env = append(os.Environ(), contributorChildApplyMode+"=1", contributorChildRoot+"="+hostRoot,
		contributorChildBundle+"="+bundle, contributorChildPin+"="+pin)
	output, err := command.CombinedOutput()
	return string(output), err
}

func testConcurrentWithdrawChildApply(t *testing.T) {
	root, bundle, pin := os.Getenv(contributorChildRoot), os.Getenv(contributorChildBundle), os.Getenv(contributorChildPin)
	if root == "" || bundle == "" || pin == "" {
		t.Fatal("concurrent child apply is missing its required input")
	}
	profile, err := contributor.Open(contributor.Config{Root: root, Supervisor: rejectingChildSupervisor{}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := profile.Apply(t.Context(), bundle, pin); err == nil {
		t.Fatal("concurrent child apply unexpectedly completed")
	} else if _, writeErr := fmt.Fprint(os.Stdout, err.Error()); writeErr != nil {
		t.Fatal(writeErr)
	}
}

type rejectingChildSupervisor struct{}

func (rejectingChildSupervisor) Do(context.Context, contributor.SupervisorAction) (contributor.SupervisorState, error) {
	return contributor.SupervisorState{}, errors.New("concurrent child apply reached the Supervisor")
}

type persistedContributorInstallation struct {
	DeploymentID   string `json:"deployment_id"`
	Generation     uint64 `json:"generation"`
	ManifestDigest string `json:"manifest_digest"`
}

func readPersistedContributorInstallation(t *testing.T, hostRoot string) persistedContributorInstallation {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(hostRoot, "var", "lib", "private", "ardents-contributor", "installation.json"))
	if err != nil {
		t.Fatal(err)
	}
	var record persistedContributorInstallation
	if err := json.Unmarshal(raw, &record); err != nil {
		t.Fatal(err)
	}
	return record
}
