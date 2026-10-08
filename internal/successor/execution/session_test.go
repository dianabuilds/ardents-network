package execution

import (
	"context"
	"errors"
	"testing"
)

func executionFixture(t *testing.T, surface Surface) (*Authority, *Supervisor, *Session) {
	t.Helper()
	authority, err := New(Config{ID: [32]byte{1}, Grants: []Grant{{Principal: [32]byte{2}, Surface: surface}}})
	if err != nil {
		t.Fatal(err)
	}
	supervisor, err := NewSupervisor(authority)
	if err != nil {
		t.Fatal(err)
	}
	capability, err := authority.Admit([32]byte{2}, surface)
	if err != nil {
		t.Fatal(err)
	}
	session, err := supervisor.Activate(t.Context(), capability, [32]byte{2}, surface)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = supervisor.Close() })
	return authority, supervisor, session
}

func TestJobOneClaimAndExactLastCompletion(t *testing.T) {
	_, _, session := executionFixture(t, Connection)
	first, err := session.BeginJob()
	if err != nil {
		t.Fatal(err)
	}
	if err := first.ClaimLifetime(); err != nil {
		t.Fatal(err)
	}
	if first.ClaimLifetime() == nil {
		t.Fatal("lifetime claimed twice")
	}
	if first.Finish(nil) == nil {
		t.Fatal("live Job published cleanup")
	}
	first.Retire()
	if err := first.Finish(nil); err != nil || !first.CompletedCurrent() {
		t.Fatalf("joined preparation unavailable: %v", err)
	}
	second, err := session.BeginJob()
	if err != nil {
		t.Fatal(err)
	}
	if second.Nonce() == first.Nonce() {
		t.Fatal("replacement reused nonce")
	}
	second.Retire()
	if err := second.Finish(nil); err != nil {
		t.Fatal(err)
	}
	if first.CompletedCurrent() || !second.CompletedCurrent() {
		t.Fatal("replacement inherited old result")
	}
	if first.Check() == nil || first.ClaimLifetime() == nil {
		t.Fatal("joined provenance granted live permission")
	}
}

func TestJobOperationRequiresLifetimeAndIsNeverReclaimed(t *testing.T) {
	_, _, session := executionFixture(t, Connection)
	job, err := session.BeginJob()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { job.Retire(); _ = job.Finish(nil) })
	if job.ClaimOperation() == nil {
		t.Fatal("unclaimed invocation authorized operation")
	}
	if err := job.ClaimLifetime(); err != nil {
		t.Fatal(err)
	}
	if err := job.ClaimOperation(); err != nil {
		t.Fatal(err)
	}
	if job.ClaimOperation() == nil {
		t.Fatal("operation claimed twice")
	}
	job.Retire()
	if job.ClaimOperation() == nil {
		t.Fatal("retired operation was reclaimed")
	}
	if err := job.Finish(nil); err != nil {
		t.Fatal(err)
	}
	if job.ClaimOperation() == nil {
		t.Fatal("joined operation was reclaimed")
	}
}

func TestRevokeRetainsCleanupCapacityUntilOriginalJobJoins(t *testing.T) {
	authority, supervisor, session := executionFixture(t, Connection)
	job, err := session.BeginJob()
	if err != nil {
		t.Fatal(err)
	}
	if err := authority.Revoke([32]byte{2}, Connection); err != nil {
		t.Fatal(err)
	}
	if session.Check() == nil || job.Check() == nil || job.Context().Err() == nil {
		t.Fatal("revoke retained effects")
	}
	supervisor.mu.Lock()
	_, retained := supervisor.sessions[session]
	supervisor.mu.Unlock()
	if !retained || authority.Active() != 0 {
		t.Fatal("admission and cleanup budgets were collapsed")
	}
	job.Retire()
	if err := job.Finish(nil); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	supervisor.mu.Lock()
	_, retained = supervisor.sessions[session]
	supervisor.mu.Unlock()
	if retained {
		t.Fatal("joined original slot was not returned")
	}
}

func TestCleanupFailureSynchronouslyClosesIdleSiblingAndRetainsFirstResult(t *testing.T) {
	authority, supervisor, session := executionFixture(t, Connection)
	capability, err := authority.Admit([32]byte{2}, Connection)
	if err != nil {
		t.Fatal(err)
	}
	idle, err := supervisor.Activate(t.Context(), capability, [32]byte{2}, Connection)
	if err != nil {
		t.Fatal(err)
	}
	job, err := session.BeginJob()
	if err != nil {
		t.Fatal(err)
	}
	failure := errors.New("original descendant join failed")
	job.Retire()
	if job.Finish(failure) != failure {
		t.Fatal("first result changed")
	}
	if idle.Check() == nil || idle.Context().Err() == nil {
		t.Fatal("cleanup failure left idle sibling accepting")
	}
	if _, err := authority.Admit([32]byte{2}, Connection); err == nil {
		t.Fatal("failed generation admitted work")
	}
	if job.Finish(nil) != failure || session.Close() != failure || supervisor.Close() != failure || supervisor.Close() != failure {
		t.Fatal("repeated completion erased first failure")
	}
	supervisor.mu.Lock()
	_, retained := supervisor.sessions[session]
	supervisor.mu.Unlock()
	if !retained {
		t.Fatal("failed cleanup returned physical capacity")
	}
}

func TestSupervisorRevokesAllBeforeJoiningBlockedJob(t *testing.T) {
	authority, supervisor, first := executionFixture(t, Connection)
	capability, err := authority.Admit([32]byte{2}, Connection)
	if err != nil {
		t.Fatal(err)
	}
	second, err := supervisor.Activate(t.Context(), capability, [32]byte{2}, Connection)
	if err != nil {
		t.Fatal(err)
	}
	job, err := first.BeginJob()
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	t.Cleanup(func() { job.Retire(); _ = job.Finish(nil) })
	go func() { done <- supervisor.Close() }()
	<-second.Context().Done()
	// One cancelled lease does not mean the generation-wide revoke loop has
	// finished. Observe its synchronous latch before checking the blocked Job;
	// Close must still retain that Job until its original physical join.
	supervisor.mu.Lock()
	jobCancelled := job.Context().Err() != nil
	supervisor.mu.Unlock()
	if second.Check() == nil || !jobCancelled {
		t.Fatal("sibling waited for another Job join before revoke")
	}
	select {
	case <-done:
		t.Fatal("supervisor returned before the original Job joined")
	default:
	}
	job.Retire()
	if err := job.Finish(nil); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestWrongSurfaceConsumesCapabilityWithoutCleanupReservation(t *testing.T) {
	authority, supervisor, _ := executionFixture(t, Connection)
	capability, err := authority.Admit([32]byte{2}, Connection)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := supervisor.Activate(context.Background(), capability, [32]byte{2}, Administration); err == nil {
		t.Fatal("wrong surface activated")
	}
	if _, err := supervisor.Activate(context.Background(), capability, [32]byte{2}, Connection); err == nil {
		t.Fatal("refused capability replayed")
	}
	supervisor.mu.Lock()
	count := len(supervisor.sessions)
	supervisor.mu.Unlock()
	if count != 1 {
		t.Fatal("refusal retained an invented cleanup slot")
	}
}

func TestEachSurfaceRetainsFullCleanupBudgetAfterAdmissionRelease(t *testing.T) {
	for _, surface := range []Surface{Connection, Administration} {
		t.Run(string(surface), func(t *testing.T) {
			authority, supervisor, first := executionFixture(t, surface)
			pending := []*Session{first}
			jobs := []*Job{}
			for index := 0; index < admissionCapacityFor(surface); index++ {
				var session *Session
				if index == 0 {
					session = first
				} else {
					capability, err := authority.Admit([32]byte{2}, surface)
					if err != nil {
						t.Fatal(err)
					}
					session, err = supervisor.Activate(t.Context(), capability, [32]byte{2}, surface)
					if err != nil {
						t.Fatal(err)
					}
					pending = append(pending, session)
				}
				job, err := session.BeginJob()
				if err != nil {
					t.Fatal(err)
				}
				jobs = append(jobs, job)
				t.Cleanup(func() { job.Retire(); _ = job.Finish(nil) })
				session.lease.Release()
			}
			if authority.Active() != 0 {
				t.Fatal("revoked leases still occupy authority admission")
			}
			capability, err := authority.Admit([32]byte{2}, surface)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := supervisor.Activate(t.Context(), capability, [32]byte{2}, surface); err == nil {
				t.Fatal("unjoined original cleanup slots were reused")
			}
			jobs[0].Retire()
			if err := jobs[0].Finish(nil); err != nil {
				t.Fatal(err)
			}
			if err := pending[0].Close(); err != nil {
				t.Fatal(err)
			}
			capability, err = authority.Admit([32]byte{2}, surface)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := supervisor.Activate(t.Context(), capability, [32]byte{2}, surface); err != nil {
				t.Fatalf("joined exact slot was not reusable: %v", err)
			}
		})
	}
}
