package release

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func publicVector(t *testing.T) Inputs {
	t.Helper()
	read := func(name string) []byte {
		b, e := os.ReadFile(filepath.Join("testdata", "r049-public-vector-v1", name))
		if e != nil {
			t.Fatal(e)
		}
		return b
	}
	return Inputs{RootBytes: read("root.json"), Files: map[string][]byte{metadataBaseURL + "timestamp.json": read("timestamp.json"), metadataBaseURL + "1.snapshot.json": read("1.snapshot.json"), metadataBaseURL + "1.targets.json": read("1.targets.json")}, Artifact: read("artifact.bin"), TargetPath: "ardents/windows-amd64/application", Local: LocalEnvironment{Platform: "windows-amd64", Architecture: "amd64", Environment: "h3-test", Network: "ardents-h3-test-1", RefTime: time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)}}
}

func TestUncertainPublicationRetiresOwnerAndReopenConfirmsHistory(t *testing.T) {
	root := filepath.Join(t.TempDir(), "history")
	v, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = v.Close() })
	ambiguous := errors.New("test: publication acknowledgement lost")
	// Failure-only control: all actual native flushes occur. Lose the final
	// acknowledgement only after the real metadata pointer was published.
	v.store.flush = func(path string) error {
		if err := syncDirectory(path); err != nil {
			return err
		}
		if path == root {
			pointer, err := os.ReadFile(filepath.Join(root, "current"))
			if err != nil {
				return err
			}
			state, err := os.ReadFile(filepath.Join(root, "generations", strings.TrimSpace(string(pointer)), "state.bin"))
			if err != nil {
				return err
			}
			if bytes.Contains(state, []byte("role=targets ")) {
				return ambiguous
			}
		}
		return nil
	}
	for range 2 {
		d := v.Evaluate(context.Background(), publicVector(t))
		if _, ok := d.Authorization(); ok || !errors.Is(d.Err(), ambiguous) {
			t.Fatalf("uncertain owner authorized a retry: %s %v", d.Outcome, d.Err())
		}
	}
	if err = v.Close(); !errors.Is(err, ambiguous) {
		t.Fatalf("Close lost publication uncertainty: %v", err)
	}
	v, err = Open(root)
	if err != nil {
		t.Fatal(err)
	}
	floors, err := v.CurrentFloors(context.Background())
	if err != nil || floors.TargetsVersion != 1 {
		t.Fatalf("reopen lost committed floors: %v %v", floors, err)
	}
	d := v.Evaluate(context.Background(), publicVector(t))
	if _, ok := d.Authorization(); !ok || d.Outcome != OutcomeNoUpdate {
		t.Fatalf("fresh verification after confirmed reopen refused: %s %v", d.Outcome, d.Err())
	}
}

// This mechanical control delays cancellation notification while the original
// observation reports loss after a real metadata commit. It isolates the
// original-caller guard, not filesystem or context scheduler qualification.
type committedCancellation struct {
	context.Context
	root string
}

type rootCommittedCancellation struct {
	context.Context
	root string
}

func (c rootCommittedCancellation) Err() error {
	if _, err := os.Stat(filepath.Join(c.root, "current")); err == nil {
		return context.Canceled
	}
	return nil
}

func TestOriginalCancellationBeforeMetadataCommitKeepsOnlyRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "history")
	v, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = v.Close() })
	d := v.Evaluate(rootCommittedCancellation{context.Background(), root}, publicVector(t))
	if _, ok := d.Authorization(); ok || !errors.Is(d.Err(), context.Canceled) {
		t.Fatal("lost original pre-commit cancellation", d.Err())
	}
	floors, err := v.CurrentFloors(context.Background())
	if err != nil || floors.RootVersion != 1 || floors.TargetsVersion != 0 || floors.SnapshotVersion != 0 || floors.TimestampVersion != 0 {
		t.Fatal("cancelled caller committed metadata floors", err)
	}
	if err = v.Close(); err != nil {
		t.Fatal(err)
	}
	v, err = Open(root)
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := v.CurrentFloors(context.Background())
	if err != nil || !floorSetEqual(floors, reopened) {
		t.Fatal("reopen changed cancelled history", err)
	}
}

func (c committedCancellation) Err() error {
	pointer, err := os.ReadFile(filepath.Join(c.root, "current"))
	if err != nil {
		return nil
	}
	state, err := os.ReadFile(filepath.Join(c.root, "generations", strings.TrimSpace(string(pointer)), "state.bin"))
	if err == nil && bytes.Contains(state, []byte("role=targets ")) {
		return context.Canceled
	}
	return nil
}
func TestOriginalCancellationAfterCommitRetainsFloorsWithoutHandoff(t *testing.T) {
	root := filepath.Join(t.TempDir(), "history")
	v, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if v != nil {
			if err := v.Close(); err != nil {
				t.Error(err)
			}
		}
	})
	d := v.Evaluate(committedCancellation{context.Background(), root}, publicVector(t))
	if !errors.Is(d.Err(), context.Canceled) {
		t.Fatal("original post-commit cancellation ignored")
	}
	if _, ok := d.Authorization(); ok {
		t.Fatal("late caller got authorization")
	}
	f, err := v.CurrentFloors(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if f.TargetsVersion != 1 || f.RootVersion != 1 {
		t.Fatal("late cancellation refunded committed history")
	}
	if err = v.Close(); err != nil {
		t.Fatal(err)
	}
	v, err = Open(root)
	if err != nil {
		t.Fatal(err)
	}
	f, err = v.CurrentFloors(context.Background())
	if err != nil || f.TargetsVersion != 1 {
		t.Fatal("committed floors lost on reopen")
	}
	if err = v.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSignedVectorAuthorizationAndReopen(t *testing.T) {
	in := publicVector(t)
	root := filepath.Join(t.TempDir(), "history")
	v, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	d := v.Evaluate(context.Background(), in)
	if d.Outcome != OutcomeReleaseAccepted {
		_ = v.Close()
		t.Fatalf("actual signed vector refused: %s %v", d.Outcome, d.Err())
	}
	a, ok := d.Authorization()
	if !ok {
		t.Fatal("missing actual authorization")
	}
	d.Digest[0] ^= 255
	d.Floors.TargetsDigest[0] ^= 255
	d.Path = "changed"
	frozen, ok := a.AcceptedDecision()
	if !ok || frozen.Path != in.TargetPath || bytes.Equal(frozen.Digest, d.Digest) {
		t.Fatal("authorization aliases public projection")
	}
	if err = v.Close(); err != nil {
		t.Fatal(err)
	}
	v, err = Open(root)
	if err != nil {
		t.Fatal(err)
	}
	d = v.Evaluate(context.Background(), in)
	if d.Outcome != OutcomeNoUpdate {
		t.Fatalf("retained exact candidate: %s %v", d.Outcome, d.Err())
	}
	if _, ok = d.Authorization(); !ok {
		t.Fatal("no-update lacks fresh actual verification")
	}
	if err = v.Close(); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(filepath.Join(root, "current")); err != nil {
		t.Fatal(err)
	}
	if reset, err := Open(root); err == nil {
		_ = reset.Close()
		t.Fatal("lost retained pointer reset initial trust")
	}
}

func TestCloseJoinsAdmittedOperationBeforeLeaseReturn(t *testing.T) {
	root := filepath.Join(t.TempDir(), "history")
	v, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	work, leave, err := v.enter(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	closed := make(chan error, 1)
	go func() { closed <- v.Close() }()
	<-v.stop
	<-work.Done()
	if replacement, err := Open(root); err == nil {
		_ = replacement.Close()
		t.Fatal("lease returned before original work joined")
	}
	select {
	case err := <-closed:
		t.Fatalf("Close completed before leave: %v", err)
	default:
	}
	leave()
	if err = <-closed; err != nil {
		t.Fatal(err)
	}
	if err = v.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err = reopened.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestWaitingCallerCancellationAndZeroVerifier(t *testing.T) {
	var zero Verifier
	if _, err := zero.CurrentFloors(context.Background()); !errors.Is(err, ErrClosed) {
		t.Fatal("zero owner accepted")
	}
	v, err := Open(filepath.Join(t.TempDir(), "history"))
	if err != nil {
		t.Fatal(err)
	}
	_, leave, err := v.enter(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = v.CurrentFloors(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled waiter entered")
	}
	leave()
	if err = v.Close(); err != nil {
		t.Fatal(err)
	}
}
