//go:build linux

package publication

import (
	"errors"
	"os"
	"testing"
)

// A non-root Linux process must observe the actual filesystem denial. This
// exercises the deployed platform's removal boundary, not an injected callback.
func TestUnpublishRetryFinishesFailedGenerationRemoval(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Fatal("generation removal failure test requires an unprivileged Linux process")
	}
	fixture := newPublicationFixture(t)
	root := t.TempDir()
	owner, err := Open(fixture.config(root))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := owner.Close(); err != nil {
			t.Error(err)
		}
	}()
	if _, err := owner.Publish(t.Context(), fixture.input(t, 1)); err != nil {
		t.Fatal(err)
	}
	generationDirectory := generationPath(root, publicationGeneration(1))
	if err := os.Chmod(generationDirectory, 0500); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chmod(generationDirectory, 0700); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Error(err)
		}
	}()
	if err := owner.Unpublish(t.Context()); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("actual generation removal was not denied: %v", err)
	}
	if lease, err := owner.Acquire(t.Context()); err == nil {
		_ = lease.Close()
		t.Fatal("failed removal left publication acquirable")
	}
	if _, err := os.Stat(generationDirectory); err != nil {
		t.Fatalf("failed removal did not retain the generation for cleanup: %v", err)
	}
	if err := os.Chmod(generationDirectory, 0700); err != nil {
		t.Fatal(err)
	}
	// A repeated withdrawal is unavailable by contract, even when it finishes
	// the previous operation's cleanup. Inspect that cleanup through reopening.
	if err := owner.Unpublish(t.Context()); err == nil {
		t.Fatal("repeated withdrawal advertised a second live publication")
	}
	if _, err := os.Stat(generationDirectory); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("retry did not finish generation removal: %v", err)
	}
	if err := owner.Close(); err != nil {
		t.Fatalf("Close did not join recovered retirement: %v", err)
	}
	reopened, err := Open(fixture.config(root))
	if err != nil {
		t.Fatalf("reopening after cleanup: %v", err)
	}
	defer func() {
		if err := reopened.Close(); err != nil {
			t.Error(err)
		}
	}()
	floor, err := reopened.Floor()
	if err != nil || floor != 1 {
		t.Fatalf("withdrawal lost durable generation floor: %d %v", floor, err)
	}
	if lease, err := reopened.Acquire(t.Context()); err == nil {
		_ = lease.Close()
		t.Fatal("reopening resurrected the retired publication")
	}
	if _, err := reopened.Publish(t.Context(), fixture.input(t, 1)); err == nil {
		t.Fatal("reopening accepted the retired generation")
	}
}
