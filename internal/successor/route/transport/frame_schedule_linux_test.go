//go:build linux

package transport

import (
	"bytes"
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

func TestFrameScheduleControlAlternatesWithRoundRobinData(t *testing.T) {
	physical := newLifecycleConn(true)
	s := newSession(context.Background(), physical, time.Now().Add(5*time.Second), 32<<20, nil, false, &queueBudget{maximum: 64 << 20}, nil)
	defer s.Close()
	first := lifecycleLane(t, s, 1)
	results := make(chan error, 5)
	go func() { _, err := first.Write([]byte("first")); results <- err }()
	select {
	case <-physical.writes:
	case <-time.After(time.Second):
		t.Fatal("initial physical frame absent")
	}
	for index, id := range []uint32{5, 3, 7, 9} {
		l := lifecycleLane(t, s, id)
		go func() {
			var err error
			if l.id < 7 {
				_, err = l.Write([]byte("data"))
			} else {
				err = s.write(l, ardp.Frame{Kind: ardp.KindCredit, Lane: l.id, Body: []byte{0, 0, 0, 1}}, false)
			}
			results <- err
		}()
		deadline := time.Now().Add(time.Second)
		for {
			s.mu.Lock()
			queued := len(s.output)
			s.mu.Unlock()
			if queued == index+1 {
				break
			}
			if !time.Now().Before(deadline) {
				t.Fatal("writer did not enter bounded queue")
			}
			runtime.Gosched()
		}
	}
	close(physical.writeGate)
	for range 5 {
		if err := lifecycleResult(t, results); err != nil {
			t.Fatal(err)
		}
	}
	for _, want := range []uint32{7, 3, 9, 5} {
		select {
		case raw := <-physical.writes:
			frame, err := ardp.ReadFrame(bytes.NewReader(raw))
			if err != nil || frame.Lane != want {
				t.Fatalf("want lane %d, got %+v %v", want, frame, err)
			}
		case <-time.After(time.Second):
			t.Fatal("queued frame starved")
		}
	}
	s.mu.Lock()
	outbound, control := s.outbound, s.controlQueued
	s.mu.Unlock()
	if outbound != 0 || control != 0 {
		t.Fatalf("output accounting leaked: %d %d", outbound, control)
	}
}
