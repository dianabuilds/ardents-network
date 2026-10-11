package runtime

import (
	"errors"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/execution"
)

func TestAdministrationContextRefusesAbsentAndDetachedOperations(t *testing.T) {
	var original AdministrationContext
	if original.Context() != nil || original.Done() != nil {
		t.Fatal("absent context supplied an original lifetime")
	}
	if err, completed := original.Completion(); err == nil || completed {
		t.Fatal("absent context supplied successful completion")
	}
	for _, operation := range []*Operation{nil, {}, {publisher: true}} {
		if value, err := operation.AdministrationContext(); err == nil || value != original {
			t.Fatal("detached operation supplied Administration authority", err)
		}
		if original.Check(operation) == nil {
			t.Fatal("absent original context accepted an operation")
		}
	}
}

// Genuine local admission checks the read-only result projection. No worker
// launch or qualified Publisher operation is supplied by this fixture.
func TestAdministrationContextCompletionRetainsOriginalFailure(t *testing.T) {
	authority, err := execution.New(execution.Config{ID: [32]byte{1}, Grants: []execution.Grant{{Principal: [32]byte{2}, Surface: execution.Administration}}})
	if err != nil {
		t.Fatal(err)
	}
	supervisor, err := execution.NewSupervisor(authority)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = supervisor.Close() })
	capability, err := authority.Admit([32]byte{2}, execution.Administration)
	if err != nil {
		t.Fatal(err)
	}
	session, err := supervisor.Activate(t.Context(), capability, [32]byte{2}, execution.Administration)
	if err != nil {
		t.Fatal(err)
	}
	original := AdministrationContext{session: session}
	job, err := session.BeginJob()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { job.Retire(); _ = job.Finish(nil) })
	authority.Close()
	if err, completed := original.Completion(); err != nil || completed {
		t.Fatal("revocation supplied a terminal result", err, completed)
	}
	failure := errors.New("original physical cleanup unavailable")
	job.Retire()
	if job.Finish(failure) != failure || session.Close() != failure {
		t.Fatal("original cleanup failure changed")
	}
	<-original.Done()
	for range 2 {
		if err, completed := original.Completion(); err != failure || !completed {
			t.Fatal("read-only context erased original failure", err, completed)
		}
	}
}
