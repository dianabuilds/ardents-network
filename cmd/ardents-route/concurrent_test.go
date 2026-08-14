package main

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestCollectConcurrentClosesEveryOwnedTaskOnConstructionFailure(t *testing.T) {
	constructErr, closeErr := errors.New("construct failed"), errors.New("close failed")
	var calls, closed atomic.Int32
	next := func() (concurrentTask, bool, error) {
		switch calls.Add(1) {
		case 1:
			return concurrentTask{attachment: 1, close: func() error { closed.Add(1); return closeErr }}, true, nil
		case 2:
			return concurrentTask{attachment: 2, close: func() error { closed.Add(1); return nil }}, true, nil
		default:
			return concurrentTask{}, false, constructErr
		}
	}
	if _, err := collectConcurrent(next); !errors.Is(err, constructErr) || !errors.Is(err, closeErr) {
		t.Fatalf("construction and cleanup errors were not preserved: %v", err)
	}
	if closed.Load() != 2 {
		t.Fatalf("closed tasks=%d; want 2", closed.Load())
	}
}

func TestExecuteConcurrentSerializesEvidenceAndClosesMixedOutcomes(t *testing.T) {
	var active, encoded, closed atomic.Int32
	var concurrent atomic.Bool
	readyAttachments := make(chan uint32, 2)
	encode := func(value any) error {
		if active.Add(1) != 1 {
			concurrent.Store(true)
		}
		if evidence, ok := value.(route.Evidence); ok && evidence.Kind == "ready" {
			readyAttachments <- evidence.Attachment
		}
		time.Sleep(time.Millisecond)
		encoded.Add(1)
		active.Add(-1)
		return nil
	}
	run := func(_ context.Context, actor route.Actor, ready func(route.Evidence)) (route.Evidence, error) {
		ready(route.Evidence{Kind: "ready", Role: actor.Role})
		if actor.Role == "failed" {
			return route.Evidence{Role: actor.Role}, errors.New("bounded listener expired")
		}
		return route.Evidence{Role: actor.Role}, nil
	}
	tasks := []concurrentTask{
		{actor: route.Actor{Role: "failed"}, attachment: 1, close: func() error { closed.Add(1); return nil }},
		{actor: route.Actor{Role: "publisher"}, attachment: 2, close: func() error { closed.Add(1); return nil }},
	}
	if err := executeConcurrent(context.Background(), tasks, encode, run); err != nil {
		t.Fatal(err)
	}
	if concurrent.Load() || encoded.Load() != 4 || closed.Load() != 2 {
		t.Fatalf("concurrent=%v encoded=%d closed=%d", concurrent.Load(), encoded.Load(), closed.Load())
	}
	seen := map[uint32]bool{<-readyAttachments: true, <-readyAttachments: true}
	if !seen[1] || !seen[2] {
		t.Fatalf("ready Attachment ownership=%v", seen)
	}
}

func TestExecuteConcurrentPropagatesCloseAndEncodeFailures(t *testing.T) {
	closeErr, encodeErr := errors.New("close failed"), errors.New("encode failed")
	run := func(_ context.Context, _ route.Actor, ready func(route.Evidence)) (route.Evidence, error) {
		ready(route.Evidence{Kind: "ready"})
		return route.Evidence{}, nil
	}
	task := concurrentTask{attachment: 1, close: func() error { return closeErr }}
	if err := executeConcurrent(context.Background(), []concurrentTask{task}, func(any) error { return nil }, run); !errors.Is(err, closeErr) {
		t.Fatalf("close failure was hidden: %v", err)
	}
	task.close = func() error { return nil }
	if err := executeConcurrent(context.Background(), []concurrentTask{task}, func(any) error { return encodeErr }, run); !errors.Is(err, encodeErr) {
		t.Fatalf("encode failure was hidden: %v", err)
	}
}

func TestExecuteConcurrentRetainsEveryContextualizedTerminalError(t *testing.T) {
	closeOne, closeTwo := errors.New("close one"), errors.New("close two")
	readyErr, terminalErr := errors.New("ready encode"), errors.New("terminal encode")
	run := func(_ context.Context, actor route.Actor, ready func(route.Evidence)) (route.Evidence, error) {
		ready(route.Evidence{Kind: "ready", Role: actor.Role})
		return route.Evidence{Kind: "complete", Role: actor.Role}, nil
	}
	encode := func(value any) error {
		if evidence, ok := value.(route.Evidence); ok && evidence.Kind == "ready" {
			return readyErr
		}
		return terminalErr
	}
	tasks := []concurrentTask{
		{actor: route.Actor{Role: "publisher"}, attachment: 1, close: func() error { return closeOne }},
		{actor: route.Actor{Role: "publisher"}, attachment: 2, close: func() error { return closeTwo }},
	}
	err := executeConcurrent(context.Background(), tasks, encode, run)
	for _, wanted := range []error{closeOne, closeTwo, readyErr, terminalErr} {
		if !errors.Is(err, wanted) {
			t.Fatalf("concurrent terminal error %v was hidden: %v", wanted, err)
		}
	}
	if !strings.Contains(err.Error(), "Attachment 1") || !strings.Contains(err.Error(), "Attachment 2") {
		t.Fatalf("cleanup errors lack Attachment ownership: %v", err)
	}
}
