package duty

import (
	"testing"
	"time"
)

// The reader overlaps a real producer lease and must observe the generation
// committed before that lease is released, never an invented no-conflict view.
func TestReadConflictWaitsForConcurrentProducerCommit(t *testing.T) {
	root := localRoleFixtureRoot(t)
	writer, err := Open(Config{Root: root, Clock: time.Now, Create: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = writer.Close() })
	type result struct {
		conflict bool
		err      error
	}
	done := make(chan result, 1)
	started := make(chan struct{})
	go func() {
		close(started)
		conflict, err := ReadConflict(root, time.Now, [32]byte{1}, [32]byte{2})
		done <- result{conflict, err}
	}()
	<-started
	select {
	case result := <-done:
		t.Fatalf("reader failed during a live producer transaction: conflict=%v, err=%v", result.conflict, result.err)
	case <-time.After(50 * time.Millisecond):
	}
	if err := writer.Replace([32]byte{3}, []Duty{{Identity: [32]byte{1}, Family: [32]byte{2}, Class: "direct-source", State: "exposed", NotAfter: time.Now().Add(time.Hour)}}); err != nil {
		_ = writer.Close()
		<-done
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		<-done
		t.Fatal(err)
	}
	select {
	case result := <-done:
		if result.err != nil || !result.conflict {
			t.Fatalf("reader lost committed exposure: conflict=%v, err=%v", result.conflict, result.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("reader did not complete after producer release")
	}
}
