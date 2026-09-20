//go:build linux

package route

import (
	"crypto/ed25519"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

// nestedTerminalPhysicalBoundary stops one selected physical frame after its
// first byte. The peer still consumes that byte, so the frame is genuinely in
// flight rather than merely waiting for a test lock.
type nestedTerminalPhysicalBoundary struct {
	net.Conn
	mu                sync.Mutex
	armed             bool
	entered, release  chan struct{}
	resumed, complete chan struct{}
	holdNext          bool
	nextEntered       chan struct{}
	nextRelease       chan struct{}
}

func (connection *nestedTerminalPhysicalBoundary) holdNextCompletion() (<-chan struct{}, func()) {
	connection.mu.Lock()
	defer connection.mu.Unlock()
	connection.holdNext = true
	connection.nextEntered = make(chan struct{})
	connection.nextRelease = make(chan struct{})
	entered, release := connection.nextEntered, connection.nextRelease
	var once sync.Once
	return entered, func() { once.Do(func() { close(release) }) }
}

func (connection *nestedTerminalPhysicalBoundary) arm() (<-chan struct{}, <-chan struct{}, <-chan struct{}, func()) {
	connection.mu.Lock()
	defer connection.mu.Unlock()
	connection.armed = true
	connection.entered = make(chan struct{})
	connection.release = make(chan struct{})
	connection.resumed = make(chan struct{})
	connection.complete = make(chan struct{})
	entered, release, resumed, complete := connection.entered, connection.release, connection.resumed, connection.complete
	var once sync.Once
	return entered, resumed, complete, func() { once.Do(func() { close(release) }) }
}

func (connection *nestedTerminalPhysicalBoundary) Write(value []byte) (int, error) {
	connection.mu.Lock()
	if !connection.armed {
		if connection.holdNext {
			connection.holdNext = false
			entered, release := connection.nextEntered, connection.nextRelease
			connection.mu.Unlock()
			written, err := connection.Conn.Write(value)
			close(entered)
			<-release
			return written, err
		}
		connection.mu.Unlock()
		return connection.Conn.Write(value)
	}
	if len(value) == 0 {
		connection.mu.Unlock()
		return connection.Conn.Write(value)
	}
	connection.armed = false
	entered, release, resumed, complete := connection.entered, connection.release, connection.resumed, connection.complete
	connection.mu.Unlock()
	written, err := connection.Conn.Write(value[:1])
	close(entered)
	<-release
	close(resumed)
	if err != nil {
		close(complete)
		return written, err
	}
	count, err := connection.Conn.Write(value[1:])
	close(complete)
	return written + count, err
}

type nestedSourceLaneResult struct {
	lane *closedSourceLane
	err  error
}

func openNestedSourceLane(t *testing.T, owner *closedSourceChannels, peer net.Conn, end time.Time) (*closedSourceLane, ClosedLaneFrame) {
	t.Helper()
	result := make(chan nestedSourceLaneResult, 1)
	go func() {
		lane, err := owner.open(t.Context(), sourceIssuerOpen(end), end)
		result <- nestedSourceLaneResult{lane: lane, err: err}
	}()
	frame, err := ReadClosedLaneFrame(peer)
	if err != nil {
		t.Fatal(err)
	}
	opened := <-result
	if opened.err != nil {
		t.Fatal(opened.err)
	}
	if frame.Kind != closedFrameOpen || frame.Lane != opened.lane.id {
		t.Fatalf("source OPEN = kind %d lane %d, want lane %d", frame.Kind, frame.Lane, opened.lane.id)
	}
	return opened.lane, frame
}

func installNestedSourcePeerLane(t *testing.T, owner *closedSourceChannels, frame ClosedLaneFrame, end time.Time) *closedSourceLane {
	t.Helper()
	open, err := DecodeClosedOpen(frame.Body)
	if err != nil {
		t.Fatal(err)
	}
	lane := &closedSourceLane{owner: owner, id: frame.Lane, end: open.Deadline, readEnd: end, writeEnd: end,
		opened: true, active: true, credit: closedOuterLaneCredit, receiveCredit: closedOuterLaneCredit}
	owner.lanes[lane.id] = lane
	owner.last = lane.id
	return lane
}

// A terminal frame inside role TLS is ordinary encrypted BYTES to every lower
// forwarding layer. Its terminal scheduling class must survive those layers:
// finish the already active CREDIT, emit the terminal next, and leave an
// unrelated queued lane usable. The terminal bytes keep their ordinary data
// reservation; they do not borrow the finite control reserve.
func TestClosedNestedTLSTerminalPrecedesQueuedSiblingAfterActiveCredit(t *testing.T) {
	local, peer := net.Pipe()
	boundary := &nestedTerminalPhysicalBoundary{Conn: local}
	end := time.Now().UTC().Add(10 * time.Second).Truncate(time.Second)
	lower := newClosedSourceChannelOwner(boundary, end, boundary.Close)
	lower.start()
	remote := newClosedSourceChannelOwner(peer, end, peer.Close)

	primary, primaryOpen := openNestedSourceLane(t, lower, peer, end)
	sibling, siblingOpen := openNestedSourceLane(t, lower, peer, end)
	remotePrimary := installNestedSourcePeerLane(t, remote, primaryOpen, end)
	remoteSibling := installNestedSourcePeerLane(t, remote, siblingOpen, end)
	remote.start()
	lower.mu.Lock()
	primary.active, sibling.active = true, true
	lower.mu.Unlock()

	certificate := entryBindingCertificate(t, 199)
	serverKey := identifierFromKey(certificate.Leaf.PublicKey.(ed25519.PublicKey))
	serverResult := make(chan struct {
		connection net.Conn
		err        error
	}, 1)
	go func() {
		connection, err := AcceptClosedRoleTLS(t.Context(), remotePrimary, certificate, end)
		serverResult <- struct {
			connection net.Conn
			err        error
		}{connection: connection, err: err}
	}()
	clientTLS, err := OpenClosedRoleTLS(t.Context(), primary, serverKey, end)
	if err != nil {
		t.Fatal(err)
	}
	server := <-serverResult
	if server.err != nil {
		t.Fatal(server.err)
	}
	serverTLS := server.connection

	upper := newClosedSourceChannelOwner(clientTLS, end, clientTLS.Close)
	upper.start()
	upperLane, upperOpen := openNestedSourceLane(t, upper, serverTLS, end)

	var releaseOnce sync.Once
	entered, resumed, creditComplete, releasePhysical := boundary.arm()
	release := func() { releaseOnce.Do(releasePhysical) }
	t.Cleanup(func() {
		release()
		_ = upper.Close()
		_ = serverTLS.Close()
		_ = remote.Close()
		_ = lower.Close()
	})

	inboundDone := make(chan error, 1)
	go func() {
		inboundDone <- WriteClosedLaneFrame(serverTLS, ClosedLaneFrame{Kind: closedFrameBytes, Lane: upperLane.id, Body: []byte{1}})
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("real CREDIT did not reach the physical writer")
	}
	lower.mu.Lock()
	controlBefore := lower.controlsSize
	lower.mu.Unlock()

	siblingDone := make(chan error, 1)
	go func() {
		_, err := sibling.Write([]byte{42})
		siblingDone <- err
	}()
	waitSourceChannelState(t, lower, func() bool { return len(lower.data) == 1 })
	terminalDone := make(chan error, 1)
	go func() { terminalDone <- upperLane.Close() }()
	waitSourceChannelState(t, lower, func() bool { return len(lower.terminals)+len(lower.controls)+len(lower.data) == 2 })
	lower.mu.Lock()
	controlAfter := lower.controlsSize
	lower.mu.Unlock()
	if controlAfter != controlBefore {
		t.Fatalf("nested terminal changed control reservation from %d to %d", controlBefore, controlAfter)
	}

	events := make(chan string, 2)
	readErrors := make(chan error, 2)
	go func() {
		frame, err := ReadClosedLaneFrame(serverTLS)
		if err == nil && (frame.Kind != closedFrameClose || frame.Lane != upperOpen.Lane) {
			err = errors.New("nested terminal frame changed")
		}
		readErrors <- err
		events <- "terminal"
	}()
	go func() {
		var value [1]byte
		_, err := io.ReadFull(remoteSibling, value[:])
		if err == nil && value[0] != 42 {
			err = errors.New("sibling payload changed")
		}
		readErrors <- err
		events <- "sibling"
	}()

	nextEntered, releaseNextPhysical := boundary.holdNextCompletion()
	var releaseNextOnce sync.Once
	releaseNext := func() { releaseNextOnce.Do(releaseNextPhysical) }
	t.Cleanup(releaseNext)
	release()
	select {
	case <-resumed:
	case <-time.After(time.Second):
		t.Fatal("physical CREDIT boundary did not resume")
	}
	select {
	case <-creditComplete:
	case <-time.After(time.Second):
		t.Fatal("active CREDIT did not complete after release")
	}
	if err := <-inboundDone; err != nil {
		t.Fatalf("inbound TLS record failed: %v", err)
	}
	select {
	case <-nextEntered:
	case <-time.After(time.Second):
		t.Fatal("first queued frame did not reach the peer")
	}
	if first := <-events; first != "terminal" {
		t.Fatalf("first queued completion = %s, want nested terminal after active CREDIT", first)
	}
	releaseNext()
	if second := <-events; second != "sibling" {
		t.Fatalf("second queued completion = %s, want sibling", second)
	}
	for range 2 {
		if err := <-readErrors; err != nil {
			t.Fatal(err)
		}
	}
	if err := <-terminalDone; err != nil {
		t.Fatal(err)
	}
	if err := <-siblingDone; err != nil {
		t.Fatal(err)
	}
}
