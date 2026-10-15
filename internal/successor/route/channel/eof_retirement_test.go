package channel

import (
	"bytes"
	"errors"
	"io"
	"net"
	"os"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

// A real half-written canonical EOF meets a canonical peer CLOSE, or local
// Close, before its remaining bytes are consumed. Only the already selected
// terminal may finish under original cleanup; an unrelated sibling stays live.
// These pipe controls grant no TLS, Network or Admission authority.
func TestEOFAlreadyStartedJoinsRetirementWithoutPoisoningParent(t *testing.T) {
	for _, mode := range []string{"peer", "local", "refusal", "stalled", "late-failure"} {
		t.Run(mode, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				local, peer := net.Pipe()
				physical := &creditCloseConn{Conn: local, kind: ardp.KindEOF, emitted: make(chan struct{}), updated: make(chan struct{})}
				late := errors.New("original EOF physical write failed after peer CLOSE")
				if mode == "late-failure" {
					physical.writeFailure = late
				}
				end := time.Now().Add(5 * time.Second)
				s := New(t.Context(), physical, end, 1<<20, nil, false, NewBudget(4<<20), nil)
				defer func() { _ = peer.Close(); _ = s.Close() }()
				lane := lifecycleLane(t, s, 1)
				if mode == "stalled" {
					// Keep parent cancellation later than the original lane bound,
					// so this oracle observes the physical writer's own timeout.
					if err := lane.Bound(time.Now().Add(500 * time.Millisecond)); err != nil {
						t.Fatal(err)
					}
				}
				s.mu.Lock()
				lane.openEmitted = true
				s.mu.Unlock()
				written := make(chan error, 1)
				go func() { written <- lane.CloseWrite() }()
				var prefix [8]byte
				if _, err := io.ReadFull(peer, prefix[:]); err != nil {
					t.Fatal(err)
				}
				<-physical.emitted
				start := time.Now()
				closing := make(chan error, 1)
				if mode == "local" {
					go func() { closing <- lane.Close() }()
				} else {
					status := byte(0)
					if mode == "refusal" {
						status = 1
					}
					if err := ardp.WriteFrame(peer, ardp.Frame{Kind: ardp.KindClose, Lane: 1, Body: []byte{status}}); err != nil {
						t.Fatal(err)
					}
				}
				// Local Close waits for writeMu, which is not a durably blocked
				// synctest wait. The actual deadline effect supplies this barrier.
				<-physical.updated
				if mode == "stalled" {
					if err := <-written; !errors.Is(err, os.ErrDeadlineExceeded) {
						t.Fatal("stalled original EOF lost timeout", err)
					}
					if time.Since(start) > 500*time.Millisecond {
						t.Fatal("EOF exceeded earlier original bound")
					}
					if err := s.Close(); !errors.Is(err, os.ErrDeadlineExceeded) {
						t.Fatal("joined EOF timeout lost", err)
					}
					return
				}
				frame, err := ardp.ReadFrame(io.MultiReader(bytes.NewReader(prefix[:]), peer))
				if err != nil || frame.Kind != ardp.KindEOF || frame.Lane != 1 || len(frame.Body) != 0 {
					t.Fatal("selected EOF was interrupted", frame, err)
				}
				result := <-written
				if mode == "late-failure" {
					if !errors.Is(result, late) || !errors.Is(s.Close(), late) {
						t.Fatal("late original physical failure erased", result)
					}
					return
				}
				if result != nil {
					t.Fatal("joined original EOF failed", result)
				}
				if mode == "local" {
					terminal, err := ardp.ReadFrame(peer)
					if err != nil || terminal.Kind != ardp.KindClose || terminal.Lane != 1 {
						t.Fatal("local CLOSE did not follow EOF", terminal, err)
					}
					if err := <-closing; err != nil {
						t.Fatal(err)
					}
				}
				if mode == "refusal" {
					var one [1]byte
					if _, err := lane.Read(one[:]); err == nil || err == io.EOF {
						t.Fatal("peer refusal became clean EOF", err)
					}
				}
				if !s.Live() {
					t.Fatal("selected EOF retired unrelated parent")
				}
				sibling := lifecycleLane(t, s, 3)
				done := make(chan error, 1)
				go func() { _, err := sibling.Write([]byte{9}); done <- err }()
				frame, err = ardp.ReadFrame(peer)
				if err != nil || frame.Kind != ardp.KindBytes || frame.Lane != 3 || !bytes.Equal(frame.Body, []byte{9}) {
					t.Fatal("sibling lost useful work", frame, err)
				}
				if err := <-done; err != nil {
					t.Fatal(err)
				}
			})
		})
	}
}

type eofDeadlineGate struct {
	net.Conn
	once            sync.Once
	entered, resume chan struct{}
}

func (c *eofDeadlineGate) SetWriteDeadline(end time.Time) error {
	c.once.Do(func() { close(c.entered); <-c.resume })
	return c.Conn.SetWriteDeadline(end)
}

// A deadline operation can be selected without entering physical Write. A peer
// CLOSE decoded during that operation must prevent a new EOF, even though an
// already started EOF has the bounded completion right exercised above.
func TestEOFRefusesPhysicalStartAfterPeerCloseDuringDeadline(t *testing.T) {
	for _, status := range []byte{0, 1} {
		t.Run(string(rune('0'+status)), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				local, peer := net.Pipe()
				physical := &eofDeadlineGate{Conn: local, entered: make(chan struct{}), resume: make(chan struct{})}
				s := New(t.Context(), physical, time.Now().Add(5*time.Second), 1<<20, nil, false, NewBudget(4<<20), nil)
				defer func() { _ = peer.Close(); _ = s.Close() }()
				lane := lifecycleLane(t, s, 1)
				written := make(chan error, 1)
				go func() { written <- lane.CloseWrite() }()
				<-physical.entered
				s.mu.Lock()
				changed := lane.changed
				s.mu.Unlock()
				if err := ardp.WriteFrame(peer, ardp.Frame{Kind: ardp.KindClose, Lane: 1, Body: []byte{status}}); err != nil {
					t.Fatal(err)
				}
				<-changed // Only the actual decoder can signal this original lane here.
				close(physical.resume)
				type outcome struct {
					frame ardp.Frame
					err   error
				}
				output := make(chan outcome, 1)
				go func() { f, e := ardp.ReadFrame(peer); output <- outcome{f, e} }()
				select {
				case err := <-written:
					var refusal *frameRetirement
					if !errors.As(err, &refusal) || refusal.lane != lane || refusal.kind != ardp.KindEOF {
						t.Fatal("unemitted original EOF lost its refusal", err)
					}
				case result := <-output:
					t.Fatal("EOF entered physical output after peer CLOSE", result)
				}
				if !s.Live() {
					t.Fatal("unemitted EOF retired parent")
				}
				sibling := lifecycleLane(t, s, 3)
				done := make(chan error, 1)
				go func() { _, err := sibling.Write([]byte{7}); done <- err }()
				result := <-output
				if result.err != nil || result.frame.Kind != ardp.KindBytes || result.frame.Lane != 3 || !bytes.Equal(result.frame.Body, []byte{7}) {
					t.Fatal("new EOF or lost sibling", result)
				}
				if err := <-done; err != nil {
					t.Fatal(err)
				}
			})
		})
	}
}
