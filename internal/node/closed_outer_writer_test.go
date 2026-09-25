package node

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
)

type closedOuterDeadlineObserved struct {
	net.Conn
	started chan struct{}
	once    sync.Once
}

func (connection *closedOuterDeadlineObserved) Write(raw []byte) (int, error) {
	connection.once.Do(func() { close(connection.started) })
	return connection.Conn.Write(raw)
}

func TestClosedOuterWriterUpdatesOnlyActiveChildDeadline(t *testing.T) {
	local, peer := net.Pipe()
	defer local.Close()
	defer peer.Close()
	observed := &closedOuterDeadlineObserved{Conn: local, started: make(chan struct{})}
	writer := &closedOuterWriter{connection: observed}
	done := make(chan error, 1)
	go func() {
		done <- writer.write(ardp.Frame{Kind: 6, Lane: 1, Body: []byte{1}}, func() time.Time { return time.Now().Add(time.Hour) }, false, false)
	}()
	<-observed.started
	if err := writer.update(3, time.Now()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
		t.Fatal("sibling changed in-flight write deadline")
	default:
	}
	if err := writer.update(1, time.Now()); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("interrupted frame succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("updated deadline did not interrupt physical write")
	}
}

func TestClosedOuterWriterReadsQueuedDeadlineAfterSerialization(t *testing.T) {
	local, peer := net.Pipe()
	defer local.Close()
	defer peer.Close()
	writer := &closedOuterWriter{connection: local}
	writer.writer.Lock()
	var mu sync.Mutex
	end := time.Now().Add(time.Hour)
	requested, done := make(chan struct{}), make(chan error, 1)
	go func() {
		close(requested)
		done <- writer.write(ardp.Frame{Kind: 6, Lane: 1, Body: []byte{1}}, func() time.Time { mu.Lock(); defer mu.Unlock(); return end }, false, false)
	}()
	<-requested
	mu.Lock()
	end = time.Now()
	mu.Unlock()
	writer.writer.Unlock()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled queued write succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("queued writer restored obsolete long deadline")
	}
}

func TestClosedOuterWriterBoundsActiveCreditForTerminalCleanup(t *testing.T) {
	local, peer := net.Pipe()
	defer local.Close()
	defer peer.Close()
	observed := &closedOuterDeadlineObserved{Conn: local, started: make(chan struct{})}
	writer := &closedOuterWriter{connection: observed}
	done := make(chan error, 1)
	go func() {
		done <- writer.write(ardp.Frame{Kind: 7, Lane: 1, Body: []byte{0, 0, 0, 1}},
			func() time.Time { return time.Now().Add(time.Hour) }, true, false)
	}()
	<-observed.started
	cleanupEnd := time.Now().Add(100 * time.Millisecond)
	if err := writer.update(1, cleanupEnd); err != nil {
		t.Fatal(err)
	}
	if err := writer.update(1, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("bounded active CREDIT succeeded without a reader")
		}
	case <-time.After(time.Second):
		t.Fatal("terminal cleanup did not bound active CREDIT")
	}
}

func TestClosedOuterWriterTerminalCleanupDoesNotExtendActiveCreditDeadline(t *testing.T) {
	local, peer := net.Pipe()
	defer local.Close()
	defer peer.Close()
	observed := &closedOuterDeadlineObserved{Conn: local, started: make(chan struct{})}
	writer := &closedOuterWriter{connection: observed}
	originalEnd := time.Now().Add(100 * time.Millisecond)
	done := make(chan error, 1)
	go func() {
		done <- writer.write(ardp.Frame{Kind: 7, Lane: 1, Body: []byte{0, 0, 0, 1}},
			func() time.Time { return originalEnd }, true, false)
	}()
	<-observed.started
	if err := writer.update(1, time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("active CREDIT exceeded its original deadline")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("terminal cleanup extended the active CREDIT deadline")
	}
}

func TestClosedOuterWriterTerminalPriorityYieldsToQueuedData(t *testing.T) {
	writer := &closedOuterWriter{dataDue: true}
	terminalOne := &closedOuterWriteRequest{terminal: true}
	terminalTwo := &closedOuterWriteRequest{terminal: true}
	dataOne := &closedOuterWriteRequest{}
	dataTwo := &closedOuterWriteRequest{}
	writer.terminals = []*closedOuterWriteRequest{terminalOne, terminalTwo}
	writer.data = []*closedOuterWriteRequest{dataOne, dataTwo}

	for index, want := range []*closedOuterWriteRequest{terminalOne, dataOne, terminalTwo, dataTwo} {
		if got := writer.nextLocked(); got != want {
			t.Fatalf("schedule %d = %p, want %p", index, got, want)
		}
	}
}

func TestClosedOuterChildCloseInterruptsItsPhysicalWriter(t *testing.T) {
	for _, ending := range []string{"local", "peer"} {
		t.Run(ending, func(t *testing.T) {
			now := time.Now().UTC().Truncate(time.Second)
			receiver := route.ClosedOuterReceiver{NetworkID: [32]byte{1}, StateGeneration: [32]byte{2}, StateDigest: [32]byte{3}, ProfileDigest: [32]byte{4},
				NodeID: [32]byte{5}, RecordDigest: [32]byte{6}, DutyGeneration: 7, RoleDomain: 1, Subrole: 2, Deadline: now.Add(time.Hour)}
			limits, err := route.NewClosedDutyLimits(time.Now)
			if err != nil {
				t.Fatal(err)
			}
			outer, err := route.NewClosedOuterHandshake(receiver, limits, time.Now)
			if err != nil {
				t.Fatal(err)
			}
			defer outer.Close()
			local, peer := net.Pipe()
			defer local.Close()
			defer peer.Close()
			observed := &closedOuterDeadlineObserved{Conn: local, started: make(chan struct{})}
			writer := &closedOuterWriter{connection: observed}
			bridge, err := route.NewClosedOuterBridge(outer, writer.update, writer.write)
			if err != nil {
				t.Fatal(err)
			}
			hello := ardp.Hello{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest,
				ProfileDigest: receiver.ProfileDigest, RecipientNodeID: receiver.NodeID, RecipientDutyGeneration: receiver.DutyGeneration,
				Purpose: ardp.PurposeForwarding, ChannelNonce: [32]byte{8}, Deadline: receiver.Deadline}
			body, err := ardp.EncodeHello(hello)
			if err != nil {
				t.Fatal(err)
			}
			accepted := make(chan error, 1)
			go func() { _, err := ardp.ReadFrame(peer); accepted <- err }()
			if _, err := bridge.Accept(ardp.Frame{Kind: 1, Body: body}); err != nil {
				t.Fatal(err)
			}
			if err := <-accepted; err != nil {
				t.Fatal(err)
			}
			// The first observed write was outer ACCEPT. Arm observation for BYTES.
			observed.once = sync.Once{}
			observed.started = make(chan struct{})
			body, err = route.EncodeClosedNodeOpen(route.ClosedOpen{NextNodeID: receiver.NodeID, NextDutyGeneration: receiver.DutyGeneration,
				Purpose: ardp.PurposeForwarding, Deadline: now.Add(30 * time.Minute)}, route.ClosedChildOrdinary)
			if err != nil {
				t.Fatal(err)
			}
			lane, err := bridge.Accept(ardp.Frame{Kind: 4, Lane: 1, Body: body})
			if err != nil {
				t.Fatal(err)
			}
			written := make(chan error, 1)
			go func() { _, err := lane.Write([]byte{1}); written <- err }()
			<-observed.started
			closed := make(chan error, 1)
			go func() {
				if ending == "local" {
					closed <- lane.Close()
					return
				}
				_, err := bridge.Accept(ardp.Frame{Kind: 9, Lane: 1, Body: []byte{0}})
				closed <- err
			}()
			select {
			case err := <-written:
				if err == nil {
					t.Error("closed child completed blocked payload")
				}
			case <-time.After(time.Second):
				t.Fatal("child Close did not interrupt physical write")
			}
			select {
			case <-closed: // A partial frame invalidates the physical Carrier.
			case <-time.After(time.Second):
				t.Fatal("child Close failed to join its writer")
			}
		})
	}
}
