//go:build linux && document_worker_installed

package runtime

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/execution"
)

func TestInstalledExecutionRepeatedLaunch(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Fatal("invalid environment: original non-root Endpoint MainPID required")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 110*time.Second)
	defer cancel()
	for _, surface := range []execution.Surface{execution.Connection, execution.Administration} {
		t.Run(string(surface), func(t *testing.T) {
			principal := [32]byte{2}
			owner, err := New(execution.Config{ID: [32]byte{1}, Grants: []execution.Grant{{Principal: principal, Surface: surface}}})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := owner.Close(); err != nil {
					t.Error(err)
				}
			})
			capability, err := owner.authority.Admit(principal, surface)
			if err != nil {
				t.Fatal(err)
			}
			session, err := owner.supervisor.Activate(ctx, capability, principal, surface)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := session.Close(); err != nil {
					t.Error(err)
				}
			})

			var previous *Invocation
			nonces := make(map[[32]byte]bool)
			invocations := make(map[[16]byte]bool)
			const launches = 12
			for attempt := 1; attempt <= launches; attempt++ {
				if !t.Run(fmt.Sprintf("launch-%02d", attempt), func(t *testing.T) {
					job, err := session.BeginJob()
					if err != nil {
						t.Fatal(err)
					}
					if previous != nil && previous.CompletedCurrent() {
						t.Fatal("replacement kept the predecessor's completed-current provenance")
					}
					nonce := job.Nonce()
					if nonce == [32]byte{} || nonces[nonce] {
						t.Fatal("replacement reused a Job nonce")
					}
					nonces[nonce] = true
					invocation, err := owner.launchJob(job, surface)
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() {
						if err := invocation.Close(); err != nil {
							t.Error(err)
						}
					})
					if job.Check() != nil || invocation.lease.Context().Err() != nil || invocation.CompletedCurrent() {
						t.Fatal("installed worker did not retain its live Job and qualified lease")
					}
					instance := invocation.activation.Instance
					if instance.Invocation == [16]byte{} || invocations[instance.Invocation] {
						t.Fatal("replacement reused a systemd invocation")
					}
					invocations[instance.Invocation] = true
					observer := pinInstalledExecutionWorker(t, instance)
					if attempt == launches {
						if err := session.Close(); err != nil {
							t.Fatal(err)
						}
					}
					if err := invocation.Close(); err != nil {
						t.Fatal(err)
					}
					observer.requireJoined(t)
					if invocation.lease.Context().Err() == nil || invocation.CompletedCurrent() != (attempt < launches) {
						t.Fatal("joined cleanup retained live authority or the wrong completion provenance")
					}
					requireInstalledExecutionUnitCollected(t, ctx, instance.Name)
					previous = invocation
				}) {
					t.FailNow()
				}
			}
			if next, err := session.BeginJob(); next != nil || err == nil {
				t.Fatal("retired Session launched a new Job")
			}
		})
	}
}
