package durable

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

// Original reservation: original storage I/O must not hold the local
// commit comparison that Connection calls beneath its logical state lock.
func TestGenerationCommitDoesNotWaitForNativeFlush(t *testing.T) {
	credential := generationCredential(t, 1)
	value := credential.Delegation()
	root, err := Open(t.Context(), Config{Root: filepath.Join(t.TempDir(), "publication"), Target: value.Target, Network: value.Network})
	if err != nil {
		t.Fatal(err)
	}
	generation, err := root.Reserve(t.Context(), credential)
	if err != nil {
		_ = root.Close()
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	originalFlush := root.flush
	root.flush = func(path string) error { close(entered); <-release; return originalFlush(path) }
	advance := make(chan error, 1)
	go func() { advance <- generation.Advance(t.Context()) }()
	select {
	case <-entered:
	case err := <-advance:
		_ = generation.Close()
		_ = root.Close()
		t.Fatal("native advance failed before controlled flush", err)
	case <-t.Context().Done():
		close(release)
		<-advance
		_ = generation.Close()
		_ = root.Close()
		t.Fatal("controlled flush unavailable", t.Context().Err())
	}
	checked := make(chan error, 1)
	go func() { checked <- generation.CheckCommit(t.Context()) }()
	blocked := false
	var checkErr error
	select {
	case checkErr = <-checked:
	case <-time.After(100 * time.Millisecond):
		blocked = true
	}
	close(release)
	advanceErr := <-advance
	if blocked {
		checkErr = <-checked
	}
	root.flush = originalFlush
	if err := errors.Join(advanceErr, checkErr, generation.Close(), root.Close()); err != nil {
		t.Fatal(err)
	}
	if blocked {
		t.Fatal("local commit comparison waited for another owner's native filesystem flush")
	}
}

func TestGenerationCommitDeniesRetirementBeforeNativeFlushJoins(t *testing.T) {
	for _, wholeRoot := range []bool{false, true} {
		name := "reservation"
		if wholeRoot {
			name = "root"
		}
		t.Run(name, func(t *testing.T) {
			credential := generationCredential(t, 1)
			value := credential.Delegation()
			root, err := Open(t.Context(), Config{Root: filepath.Join(t.TempDir(), "publication"), Target: value.Target, Network: value.Network})
			if err != nil {
				t.Fatal(err)
			}
			generation, err := root.Reserve(t.Context(), credential)
			if err != nil {
				_ = root.Close()
				t.Fatal(err)
			}
			entered, release := make(chan struct{}), make(chan struct{})
			originalFlush := root.flush
			root.flush = func(path string) error { close(entered); <-release; return originalFlush(path) }
			advance := make(chan error, 1)
			go func() { advance <- generation.Advance(t.Context()) }()
			select {
			case <-entered:
			case err := <-advance:
				_ = generation.Close()
				_ = root.Close()
				t.Fatal("native advance failed before flush", err)
			}
			closed := make(chan error, 1)
			go func() {
				if wholeRoot {
					closed <- root.Close()
				} else {
					closed <- generation.Close()
				}
			}()
			denied := make(chan struct{})
			stop := make(chan struct{})
			joined := make(chan struct{})
			go func() {
				defer close(joined)
				tick := time.NewTicker(time.Millisecond)
				defer tick.Stop()
				for {
					if generation.CheckCommit(t.Context()) != nil {
						close(denied)
						return
					}
					select {
					case <-tick.C:
					case <-stop:
						return
					}
				}
			}()
			late := false
			select {
			case <-denied:
			case <-time.After(100 * time.Millisecond):
				late = true
			}
			earlyClosed := false
			select {
			case err := <-closed:
				earlyClosed = true
				t.Error("retirement returned before original native writer joined", err)
			default:
			}
			close(stop)
			close(release)
			<-advance // Work may retain root-close refusal; the original write joins.
			<-joined
			if wholeRoot {
				_ = generation.Close()
			}
			if !earlyClosed {
				<-closed
			}
			root.flush = originalFlush
			_ = generation.Close()
			_ = root.Close()
			if late || generation.CheckCommit(t.Context()) == nil {
				t.Fatal("native I/O delayed revoke or revived the retired reservation")
			}
		})
	}
}
