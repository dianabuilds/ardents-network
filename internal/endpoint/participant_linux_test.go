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
)

func TestTextParticipantCancellationBeforeQualificationExposesNoCommands(t *testing.T) {
	endpoint, principal := dutyContextEndpoint(t)
	release, err := endpoint.acquireLaunch(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	path := func(name string) string { return filepath.Join(root, name) }
	config := ClosedParticipantConfig{
		ConnectionPrincipal: principal, AdministrationPrincipal: principal,
		ApplicationAddress: path("reader.sock"), AdministrationAddress: path("publisher.sock"),
		ReaderPermission:    PermissionFiles{RequestPath: path("reader-request"), ResponsePath: path("reader-response"), Maxima: [3]uint32{1, 1, 1}},
		PublisherPermission: PermissionFiles{RequestPath: path("publisher-request"), ResponsePath: path("publisher-response"), Maxima: [3]uint32{1, 1, 1}},
		Observe: func(context.Context, ClosedParticipantEvent) error {
			t.Error("unqualified runtime exposed an event")
			return errors.New("unqualified runtime exposed an event")
		},
	}
	ctx, cancel := context.WithCancel(t.Context())
	completed := make(chan error, 1)
	var tasks sync.WaitGroup
	t.Cleanup(func() {
		cancel()
		if err := endpoint.closeDutyContexts(); err != nil {
			t.Error(err)
		}
		tasks.Wait()
	})
	tasks.Go(func() { completed <- endpoint.runInterfaces(ctx, config) })
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	var owner *dutyContext
	var job *jobIdentity
	for job == nil {
		endpoint.dutyMu.Lock()
		for candidate := range endpoint.dutyContexts {
			owner = candidate
			break
		}
		endpoint.dutyMu.Unlock()
		if owner != nil {
			owner.mu.Lock()
			job = owner.job
			owner.mu.Unlock()
		}
		if job != nil {
			break
		}
		select {
		case err := <-completed:
			t.Fatalf("runtime did not reach installed qualification: %v", err)
		case <-deadline.C:
			t.Fatal("runtime did not reserve qualification")
		case <-tick.C:
		}
	}
	cancel()
	select {
	case err := <-completed:
		if err == nil {
			t.Fatal("cancelled qualification succeeded")
		}
	case <-deadline.C:
		t.Fatal("runtime did not join cancellation")
	}
	tasks.Wait()
	endpoint.dutyMu.Lock()
	retained := len(endpoint.dutyContexts)
	endpoint.dutyMu.Unlock()
	owner.mu.Lock()
	clean := owner.closed && owner.job == nil && owner.verifiedJob == nil && !owner.tokens.PermissionLocked().Present() && job.finished && job.cleanupErr == nil
	owner.mu.Unlock()
	if retained != 0 || !clean {
		t.Fatal("runtime retained unqualified context or worker after cancellation")
	}
	for _, name := range []string{config.ApplicationAddress, config.AdministrationAddress, config.ReaderPermission.RequestPath, config.PublisherPermission.RequestPath} {
		if _, err := os.Lstat(name); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("runtime exposed %q before qualification: %v", name, err)
		}
	}
}
