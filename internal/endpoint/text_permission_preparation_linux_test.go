//go:build linux

package endpoint

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
)

// The real launch is held before host activation. This proves preparation owns
// and joins qualification; it supplies no installed-worker success fixture.
func TestTextPermissionPreparationJoinsUnqualifiedLaunch(t *testing.T) {
	for _, surface := range []broker.Surface{broker.Connection, broker.Administration} {
		for _, retire := range []bool{false, true} {
			t.Run(string(surface)+map[bool]string{false: "/cancel", true: "/retire"}[retire], func(t *testing.T) {
				endpoint, principal := textContextEndpoint(t)
				owner := admittedTextContext(t, endpoint, principal, surface)
				release, err := endpoint.acquireTextLaunch(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				defer release()
				root := t.TempDir()
				if err := os.Chmod(root, 0700); err != nil {
					t.Fatal(err)
				}
				requestPath := filepath.Join(root, "request")
				caller, cancel := context.WithCancel(t.Context())
				completed := make(chan error, 1)
				var helpers sync.WaitGroup
				t.Cleanup(func() {
					cancel()
					if err := owner.Close(); err != nil {
						t.Error(err)
					}
					helpers.Wait()
				})
				helpers.Go(func() {
					completed <- owner.provisionTextPermission(caller, requestPath, filepath.Join(root, "response"), [3]uint32{1, 1, 1}, func(context.Context, [32]byte) error {
						return errors.New("unqualified context reported a permission request")
					})
				})
				deadline := time.NewTimer(5 * time.Second)
				defer deadline.Stop()
				tick := time.NewTicker(time.Millisecond)
				defer tick.Stop()
				var job *textJobIdentity
				for job == nil {
					owner.mu.Lock()
					job = owner.job
					owner.mu.Unlock()
					if job != nil {
						break
					}
					select {
					case err := <-completed:
						t.Fatalf("preparation did not reach installed launch: %v", err)
					case <-deadline.C:
						t.Fatal("preparation did not reserve its worker")
					case <-tick.C:
					}
				}
				if retire {
					if err := owner.Close(); err != nil {
						t.Fatal(err)
					}
				} else {
					cancel()
				}
				if err := <-completed; err == nil {
					t.Fatal("unqualified preparation succeeded")
				}
				helpers.Wait()
				owner.mu.Lock()
				retained, qualified, permission := owner.job, owner.verifiedJob, owner.permission
				finished, cleanupErr := job.finished, job.cleanupErr
				owner.mu.Unlock()
				if retained != nil || qualified != nil || permission != nil || !finished || cleanupErr != nil {
					t.Fatal("preparation returned without joining refused qualification")
				}
				if _, err := os.Lstat(requestPath); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("unqualified request exported: %v", err)
				}
			})
		}
	}
}
