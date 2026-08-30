package publication

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPublishAfterReadinessAdvancesFloorBeforeExposure(t *testing.T) {
	t.Parallel()
	fixture := newPublicationFixture(t)
	owner, err := Open(fixture.config(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()

	input := fixture.input(t, 1)
	input.Acknowledgement = nil
	readinessStarted := make(chan struct{})
	releaseReadiness := make(chan struct{})
	published := make(chan struct {
		current Current
		err     error
	}, 1)
	go func() {
		current, publishErr := owner.PublishAfterReadiness(t.Context(), input, func(context.Context) ([]byte, error) {
			close(readinessStarted)
			<-releaseReadiness
			return []byte("canonical-registration-and-ready"), nil
		})
		published <- struct {
			current Current
			err     error
		}{current: current, err: publishErr}
	}()

	<-readinessStarted
	floor, err := owner.Floor()
	if err != nil || floor != 1 {
		t.Fatalf("Floor during readiness = %d, %v", floor, err)
	}
	if lease, acquireErr := owner.Acquire(t.Context()); acquireErr == nil {
		_ = lease.Close()
		t.Fatal("publication was exposed before readiness")
	}

	close(releaseReadiness)
	result := <-published
	if result.err != nil || result.current.Credential.Generation != 1 {
		t.Fatalf("PublishAfterReadiness = %+v, %v", result.current, result.err)
	}
	lease, err := owner.Acquire(t.Context())
	if err != nil || lease.Current().Digest != result.current.Digest {
		t.Fatalf("Acquire after readiness = %+v, %v", lease, err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestOpenRecoversUnexposedReadyGenerationAsUnavailable(t *testing.T) {
	t.Parallel()
	fixture := newPublicationFixture(t)
	root := t.TempDir()
	owner, err := Open(fixture.config(root))
	if err != nil {
		t.Fatal(err)
	}
	input := fixture.input(t, 1)
	record, _, err := encodePublication(input.Credential, input.Acknowledgement, input.InstanceSigner)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeFloor(owner.root.path, 1); err != nil {
		t.Fatal(err)
	}
	if err := writeGeneration(owner.root.path, 1, record); err != nil {
		t.Fatal(err)
	}
	if err := owner.root.lease.release(); err != nil {
		t.Fatal(err)
	}
	owner.root.closed = true

	reopened, err := Open(fixture.config(root))
	if err != nil {
		t.Fatalf("Open interrupted unexposed generation: %v", err)
	}
	defer reopened.Close()
	if _, err := reopened.Acquire(t.Context()); err == nil {
		t.Fatal("interrupted generation became live after restart")
	}
	if _, err := reopened.Publish(t.Context(), fixture.input(t, 1)); err == nil {
		t.Fatal("interrupted generation was reusable after restart")
	}
	if _, err := os.Stat(filepath.Join(root, "generations", publicationGeneration(1))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("interrupted unexposed generation was retained: %v", err)
	}
}
