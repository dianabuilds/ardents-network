package channel

import (
	"errors"
	"net"
	"sync"
	"testing"
	"time"
)

// Hold an actual call to the owned physical deadline mechanism, separately
// from its already selected writer. This supplies no domain authority.
type heldPhysicalDeadline struct {
	*lifecycleConn
	armed   bool // lifecycleConn.mu guards the selected call.
	entered chan struct{}
	resume  chan struct{}
	failure error // Failure of the held original deadline call, after delegation.
}

func (conn *heldPhysicalDeadline) SetWriteDeadline(end time.Time) error {
	conn.mu.Lock()
	hold := conn.armed
	conn.armed = false
	conn.mu.Unlock()
	if hold {
		close(conn.entered)
		<-conn.resume
	}
	err := conn.lifecycleConn.SetWriteDeadline(end)
	if hold {
		return errors.Join(err, conn.failure)
	}
	return err
}

// Joining the writer alone cannot publish completion while a separately
// admitted physical deadline call still owns its original late result.
func TestSessionCloseJoinsDeadlineAfterOriginalWriterReturns(t *testing.T) {
	late := errors.New("original physical deadline failed after writer join")
	physical := &heldPhysicalDeadline{lifecycleConn: newLifecycleConn(true), failure: late,
		entered: make(chan struct{}), resume: make(chan struct{})}
	physical.writeIgnoresClose = true
	end := time.Now().Add(5 * time.Second)
	owner := Prepare(t.Context(), physical, end, 32<<20, nil, false, &Budget{maximum: 64 << 20}, Handlers{})
	lane := lifecycleLane(t, owner, 1)
	written, updated, closed := make(chan error, 1), make(chan error, 1), make(chan error, 1)
	var resumeOnce, outputOnce sync.Once
	resume := func() { resumeOnce.Do(func() { close(physical.resume) }) }
	releaseOutput := func() { outputOnce.Do(func() { close(physical.writeGate) }) }
	writerJoined, deadlineJoined, closeJoined := false, false, false
	t.Cleanup(func() {
		resume()
		releaseOutput()
		if !writerJoined {
			<-written
		}
		if !deadlineJoined {
			<-updated
		}
		if !closeJoined {
			<-closed
		}
		_ = lane.Close()
	})
	go func() { _, err := lane.Write([]byte("original output")); written <- err }()
	<-physical.writes
	physical.mu.Lock()
	physical.armed = true
	physical.mu.Unlock()
	go func() { updated <- lane.SetWriteDeadline(end) }()
	<-physical.entered
	go func() { closed <- owner.Close() }()
	waitFramingState(t, lane, func() bool { return owner.stopped })
	releaseOutput()
	<-written
	writerJoined = true
	select {
	case err := <-closed:
		closeJoined = true
		t.Fatal("Session returned before original deadline joined", err)
	case <-time.After(100 * time.Millisecond):
	}
	resume()
	err := <-updated
	deadlineJoined = true
	if !errors.Is(err, late) {
		t.Fatal("original deadline result lost", err)
	}
	err = <-closed
	closeJoined = true
	if !errors.Is(err, late) || !errors.Is(owner.PhysicalFailure(), late) {
		t.Fatal("Session omitted late original deadline failure", err, owner.PhysicalFailure())
	}
	if err := owner.Close(); !errors.Is(err, late) {
		t.Fatal("repeated Close changed original joined result", err)
	}
}

func TestPhysicalDeadlineCannotHoldStateOrOverwriteWinningInterruption(t *testing.T) {
	for _, operation := range []string{"initial", "deadline", "interrupt", "bound", "control-bound"} {
		t.Run(operation, func(t *testing.T) {
			physical := &heldPhysicalDeadline{lifecycleConn: newLifecycleConn(true), armed: operation == "initial",
				entered: make(chan struct{}), resume: make(chan struct{})}
			physical.writeIgnoresClose = true
			late := errors.New("original selected writer failed late")
			physical.partial = late
			end := time.Now().Add(5 * time.Second)
			owner := Prepare(t.Context(), physical, end, 32<<20, nil, false, &Budget{maximum: 64 << 20}, Handlers{})
			lane := lifecycleLane(t, owner, 1)
			written, closed := make(chan error, 1), make(chan error, 1)
			var resumeOnce, outputOnce sync.Once
			resume := func() { resumeOnce.Do(func() { close(physical.resume) }) }
			releaseOutput := func() { outputOnce.Do(func() { close(physical.writeGate) }) }
			writerJoined, closeStarted, closeJoined := false, false, false
			var deadlineDone chan error
			t.Cleanup(func() {
				resume()
				releaseOutput()
				if !writerJoined {
					<-written
				}
				if deadlineDone != nil {
					<-deadlineDone
				}
				if closeStarted && !closeJoined {
					<-closed
				}
				_ = lane.Close()
				_ = owner.Close()
			})
			go func() { _, err := lane.Write([]byte("original payload")); written <- err }()
			if operation != "initial" {
				select {
				case <-physical.writes:
				case <-time.After(time.Second):
					t.Fatal("original physical writer did not start")
				}
				physical.mu.Lock()
				physical.armed = true
				physical.mu.Unlock()
				update := func() error { return lane.SetWriteDeadline(end) }
				switch operation {
				case "interrupt":
					update = lane.InterruptIO
				case "bound":
					update = func() error { return lane.Bound(end.Add(-time.Millisecond)) }
				case "control-bound":
					update = func() error { return lane.boundControlOutput(end.Add(-time.Millisecond)) }
				}
				deadlineDone = make(chan error, 1)
				go func() { deadlineDone <- update() }()
			}
			select {
			case <-physical.entered:
			case <-time.After(time.Second):
				t.Fatal("original deadline operation was not held")
			}
			sealDone := make(chan struct{})
			go func() { lane.Seal(); close(sealDone) }()
			select {
			case <-sealDone:
			case <-time.After(100 * time.Millisecond):
				t.Fatal("held foreign deadline blocked the original local seal")
			}
			closeStarted = true
			go func() { closed <- lane.Close() }()
			waitFramingState(t, lane, func() bool { return lane.localClosed && !lane.cleanupEnd.IsZero() })
			owner.mu.Lock()
			winning := owner.activeDeadline.end
			owner.mu.Unlock()
			if _, joined := lane.CleanupResult(); joined {
				t.Fatal("deadline or writer wait supplied early physical completion")
			}
			resume()
			if operation == "initial" {
				// Only the deadline effect entered physical I/O. The local seal
				// won before Write, so no late output or writer failure exists.
				err := <-written
				writerJoined = true
				if !errors.Is(err, net.ErrClosed) {
					t.Fatal("sealed payload was not refused before physical Write", err)
				}
				if err := <-closed; err != nil {
					t.Fatal("unstarted payload cleanup failed", err)
				}
				closeJoined = true
				select {
				case <-physical.writes:
					t.Fatal("sealed payload started Write after its deadline effect")
				default:
				}
				if !owner.Live() || owner.PhysicalFailure() != nil {
					t.Fatal("unstarted payload retired its healthy parent")
				}
				return
			} else {
				err := <-deadlineDone
				deadlineDone = nil
				if err != nil {
					t.Fatal("original deadline effect failed", err)
				}
			}
			physical.mu.Lock()
			applied := physical.writeEnd
			physical.mu.Unlock()
			if applied.After(winning) {
				t.Fatal("late original deadline restored a longer bound", applied, winning)
			}
			select {
			case <-closed:
				closeJoined = true
				t.Fatal("Close returned before its original writer joined")
			default:
			}
			releaseOutput()
			err := <-written
			writerJoined = true
			if !errors.Is(err, late) {
				t.Fatal("original late writer failure lost", err)
			}
			<-closed
			closeJoined = true
			if err := owner.Close(); !errors.Is(err, late) || !errors.Is(owner.PhysicalFailure(), late) {
				t.Fatal("joined original physical failure was erased", err, owner.PhysicalFailure())
			}
		})
	}
}
