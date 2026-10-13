package connection

import (
	"context"
	"errors"
	"sync"
	"testing"
	"testing/synctest"
)

// Local cancellation precedes the interrupted actor's return, even if the
// lifetime watcher has not yet acquired Stream state. Raw actor results and
// lower physical cleanup remain independently retained; this grants no success.
func TestCanceledStreamRetainsCauseBeforeLateRecordFailure(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		stream := &Stream{ctx: ctx, cancel: cancel}
		stream.cond = sync.NewCond(&stream.mu)
		records := &attachmentRecords{}
		stream.records = records
		late := errors.New("original interrupted record result")
		if canceled {
			cancel()
		}
		stream.failRecord(records, late)
		if canceled {
			if stream.failure != context.Canceled {
				t.Fatal("late record replaced original cancellation", stream.failure)
			}
		} else if stream.failure != late {
			t.Fatal("live original record failure was discarded", stream.failure)
		}
		stream.failRecord(records, errors.New("later actor failure"))
		if canceled && stream.failure != context.Canceled || !canceled && stream.failure != late {
			t.Fatal("later actor replaced retained first cause", stream.failure)
		}
		cancel()
	}
}

// These physical actor fixtures grant no Operation, Service or authenticated
// stream. They isolate joining and stale-generation refusal before any I/O.
func TestAttachmentRecordsJoinBothOriginalActors(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		records := &attachmentRecords{readerDone: make(chan struct{}), writerDone: make(chan struct{})}
		sibling := &attachmentRecords{readerDone: make(chan struct{}), writerDone: make(chan struct{})}
		joined := make(chan struct{})
		readerFailure, writerFailure := errors.New("original reader failure"), errors.New("late original writer failure")
		var result error
		go func() { result = records.join(); close(joined) }()
		records.readerResult = readerFailure
		close(records.readerDone)
		close(sibling.readerDone)
		close(sibling.writerDone)
		synctest.Wait()
		select {
		case <-joined:
			t.Fatal("sibling retirement substituted for original writer join")
		default:
		}
		records.writerResult = writerFailure
		close(records.writerDone)
		synctest.Wait()
		select {
		case <-joined:
			if !errors.Is(result, readerFailure) || !errors.Is(result, writerFailure) {
				t.Fatal("original actor join lost reader or late writer result", result)
			}
		default:
			t.Fatal("both original actors finished without releasing join")
		}
	})
}

func TestStaleAttachmentWriterCannotChangeCurrentLedgers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream := &Stream{ctx: ctx, cancel: cancel, send: &sendLedger{generation: 2, end: 7, next: 3}, terminal: &terminalExchange{generation: 2}}
	stream.cond = sync.NewCond(&stream.mu)
	current := &attachmentRecords{generation: 2}
	stream.records = current
	stale := &attachmentRecords{generation: 1, writerDone: make(chan struct{})}
	stream.writeRecords(stale)
	if !errors.Is(stream.failure, errIntegrity) || stream.send.end != 7 || stream.send.next != 3 || stream.terminal.generation != 2 || stream.records != current {
		t.Fatal("stale physical writer changed replacement state", stream.failure)
	}
	select {
	case <-stale.writerDone:
		if !errors.Is(stale.writerResult, errIntegrity) {
			t.Fatal("stale writer discarded its own refusal", stale.writerResult)
		}
	default:
		t.Fatal("stale writer did not finish its original actor")
	}
}

// A joined replacement has already sealed its previous actors. Their late
// return cannot turn the current logical stream into an integrity failure.
func TestRetiringAttachmentWriterCannotFailReplacement(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream := &Stream{ctx: ctx, cancel: cancel, send: &sendLedger{generation: 2, end: 7, next: 3}, terminal: &terminalExchange{generation: 2}}
	current := &attachmentRecords{generation: 2}
	stream.records = current
	previous := &attachmentRecords{generation: 1, retiring: true, writerDone: make(chan struct{})}
	stream.writeRecords(previous)
	if stream.failure != nil || ctx.Err() != nil || stream.send.end != 7 || stream.send.next != 3 || stream.records != current {
		t.Fatal("retiring original actor failed or changed its replacement", stream.failure)
	}
	select {
	case <-previous.writerDone:
		if !errors.Is(previous.writerResult, errIntegrity) {
			t.Fatal("retired writer discarded its own refusal", previous.writerResult)
		}
	default:
		t.Fatal("retiring writer did not join its original actor")
	}
}

// An incomplete original physical owner must refuse before transport I/O.
// Retirement suppresses its effect on a replacement, not its retained result.
func TestRetiringAttachmentReaderRetainsOriginalRefusal(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	current := &attachmentRecords{generation: 2}
	stream := &Stream{ctx: ctx, cancel: cancel, records: current}
	previous := &attachmentRecords{generation: 1, retiring: true, readerDone: make(chan struct{})}
	stream.readRecords(previous)
	<-previous.readerDone
	if !errors.Is(previous.readerResult, errIntegrity) {
		t.Fatal("retired reader discarded its own refusal", previous.readerResult)
	}
	if stream.failure != nil || ctx.Err() != nil || stream.records != current {
		t.Fatal("retired reader failed or replaced the current stream", stream.failure)
	}
}

// Waiting happens after the original state/Route barriers have been released.
// Capturing a notification before unlock must not lose an intervening effect;
// cancellation must release a waiter even without another byte/control effect.
func TestStreamChangeWaitRetainsWakeupAndCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		stream := &Stream{ctx: ctx, cancel: cancel}
		stream.mu.Lock()
		before := stream.changeLocked()
		stream.signalLocked()
		after := stream.changeLocked()
		stream.mu.Unlock()
		stream.waitChange(before)
		returned := make(chan struct{})
		go func() { stream.waitChange(after); close(returned) }()
		synctest.Wait()
		select {
		case <-returned:
			t.Fatal("new notification inherited the preceding effect")
		default:
		}
		cancel()
		synctest.Wait()
		select {
		case <-returned:
		default:
			t.Fatal("cancellation left the original change waiter blocked")
		}
	})
}
