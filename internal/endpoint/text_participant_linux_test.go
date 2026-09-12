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
	endpoint, principal := textContextEndpoint(t)
	release, err := endpoint.acquireTextLaunch(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	path := func(name string) string { return filepath.Join(root, name) }
	config := TextParticipantConfig{
		ConnectionPrincipal: principal, AdministrationPrincipal: principal,
		ApplicationAddress: path("reader.sock"), AdministrationAddress: path("publisher.sock"),
		ReaderPermission:    TextPermissionFiles{RequestPath: path("reader-request"), ResponsePath: path("reader-response"), Maxima: [3]uint32{1, 1, 1}},
		PublisherPermission: TextPermissionFiles{RequestPath: path("publisher-request"), ResponsePath: path("publisher-response"), Maxima: [3]uint32{1, 1, 1}},
		Observe: func(context.Context, TextParticipantEvent) error {
			t.Error("unqualified runtime exposed an event")
			return errors.New("unqualified runtime exposed an event")
		},
	}
	ctx, cancel := context.WithCancel(t.Context())
	completed := make(chan error, 1)
	var tasks sync.WaitGroup
	t.Cleanup(func() {
		cancel()
		if err := endpoint.closeTextContexts(); err != nil {
			t.Error(err)
		}
		tasks.Wait()
	})
	tasks.Go(func() { completed <- endpoint.runTextInterfaces(ctx, config) })
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	var owner *textContext
	var job *textJobIdentity
	for job == nil {
		endpoint.textMu.Lock()
		for candidate := range endpoint.textContexts {
			owner = candidate
			break
		}
		endpoint.textMu.Unlock()
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
	endpoint.textMu.Lock()
	retained := len(endpoint.textContexts)
	endpoint.textMu.Unlock()
	owner.mu.Lock()
	clean := owner.closed && owner.job == nil && owner.verifiedJob == nil && owner.permission == nil && job.finished && job.cleanupErr == nil
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
