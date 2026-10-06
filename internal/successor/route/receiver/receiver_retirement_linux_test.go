//go:build linux

package receiver

import (
	"context"
	"encoding/binary"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	framing "github.com/dianabuilds/ardents-network/internal/successor/route/channel"
	issuertransport "github.com/dianabuilds/ardents-network/internal/successor/route/issuer"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	"net"
	"os"
	"sync"
	"testing"
	"time"
)

type retiredReceiverListener struct{}

// Closing a shared QUIC listener also destroys its UDP transport. This observer
// isolates Receiver's ordering without supplying any successful authority.
type receiverSocketOrderListener struct {
	connectionClosed <-chan struct{}
}

func (receiverSocketOrderListener) Accept(context.Context, time.Duration) (transport.ClosedSharedCarrier, error) {
	return transport.ClosedSharedCarrier{}, context.Canceled
}

func (l receiverSocketOrderListener) Close() error {
	select {
	case <-l.connectionClosed:
		return nil
	default:
		return errors.New("listener destroyed transport before accepted connection interruption")
	}
}

func TestReceiverInterruptClosesAcceptedConnectionsBeforeSharedTransport(t *testing.T) {
	physical := newLifecycleConn(false)
	ctx, cancel := context.WithCancel(t.Context())
	cancel() // new accepting work is already denied before physical interruption
	r := &Receiver{ctx: ctx, cancel: cancel, connections: map[net.Conn]struct{}{physical: {}}, listener: receiverSocketOrderListener{connectionClosed: physical.closed}}
	r.interrupt()
	if err := r.closeListener(); err != nil {
		t.Fatal(err)
	}
	r.interrupt()
	if err := r.closeListener(); err != nil {
		t.Fatal("repeated interruption replaced listener result", err)
	}
}

func (retiredReceiverListener) Accept(context.Context, time.Duration) (transport.ClosedSharedCarrier, error) {
	return transport.ClosedSharedCarrier{}, context.Canceled
}

type issuerWriteObservedConn struct {
	net.Conn
	started chan struct{}
	once    sync.Once
}

func (c *issuerWriteObservedConn) Write(raw []byte) (int, error) {
	c.once.Do(func() { close(c.started) })
	return c.Conn.Write(raw)
}

// A real net.Pipe writer cannot finish until its peer reads. This mechanical
// refusal control supplies neither authenticated authority nor successful
// signing/Admission/Hosting: malformed proof receives only Unavailable.
// Linux names the concrete Receiver owner, not a platform-specific pipe rule.
func TestIssuerBlockedRefusalOutputRetainsCancellationAndExclusiveWork(t *testing.T) {
	server, peer := net.Pipe()
	defer server.Close()
	defer peer.Close()
	if err := peer.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	physical := &issuerWriteObservedConn{Conn: server, started: make(chan struct{})}
	r := &Receiver{queues: framing.NewBudget(256 << 10), issuerWork: make(chan struct{}, 1)}
	r.config.Issue = func(context.Context, []byte, bool) ([]byte, error) {
		return admission.EncodeClosedTokenBatchResult(admission.ClosedTokenBatchResult{Status: admission.ClosedTokenUnavailable})
	}
	release, err := r.holdIssuer(false)
	if err != nil {
		t.Fatal(err)
	}
	var returnOnce sync.Once
	returnWork := func() { returnOnce.Do(release) }
	defer returnWork()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	finished := make(chan error, 1)
	go func() {
		finished <- issuertransport.Serve(ctx, physical, ardp.Hello{Purpose: ardp.PurposeIssuer, Deadline: time.Now().Add(5 * time.Second)}, 64<<10, ctx.Err, func(ctx context.Context, request []byte) ([]byte, error) { return r.config.Issue(ctx, request, false) })
	}()
	// Framing-only batch: no valid permission or holder signature.
	batch := make([]byte, 689+259)
	copy(batch, "ARDIBR01")
	binary.BigEndian.PutUint16(batch[623:625], 1)
	body, err := ardp.EncodeIssuerRequest([32]byte{1}, batch)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(body)
	if err := ardp.WriteFrame(peer, ardp.Frame{Kind: ardp.KindOperation, Body: body}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-physical.started:
	case err := <-finished:
		t.Fatal("refusal writer never started", err)
	case <-time.After(2 * time.Second):
		t.Fatal("writer did not reach physical pipe")
	}
	select {
	case err := <-finished:
		t.Fatal("unread output completed", err)
	default:
	}
	if extra, err := r.holdIssuer(false); err == nil {
		extra()
		t.Fatal("blocked output released exclusive work")
	}
	cancel()
	if err := lifecycleResult(t, finished); !errors.Is(err, context.Canceled) || !errors.Is(err, net.ErrClosed) && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatal("blocked physical refusal lost original cancellation or I/O cause", err)
	}
	returnWork() // writer is joined; the enclosing owner may now return work
	returnWork()
	if next, err := r.holdIssuer(false); err != nil {
		t.Fatal("joined issuer work did not return capacity", err)
	} else {
		next()
	}
}
func (retiredReceiverListener) Close() error { return nil }

// Mechanical terminal-owner oracle: no substitute successful authority or
// Admission. Both real receiving session paths call finishSession; genuine
// admitted operation and peer refusal are covered in command Carrier tests.
func TestReceiverRetainsJoinedWriterFailure(t *testing.T) {
	late := errors.New("late receiving physical write failed")
	physical := newLifecycleConn(true)
	physical.writeIgnoresClose, physical.partial = true, late
	r := &Receiver{}
	s, l, err := newJoinedSession(context.Background(), physical, time.Now().Add(5*time.Second), 32<<20, nil, framing.NewBudget(64<<20))
	if err != nil {
		t.Fatal(err)
	}
	written := make(chan error, 1)
	go func() { _, err := l.Write([]byte("started")); written <- err }()
	<-physical.writes
	joined := make(chan error, 1)
	go func() { joined <- r.finishSession(s) }()
	<-physical.closed
	select {
	case <-joined:
		t.Fatal("receiver completed before write joined")
	default:
	}
	close(physical.writeGate)
	if err := lifecycleResult(t, written); !errors.Is(err, late) {
		t.Fatal(err)
	}
	if err := lifecycleResult(t, joined); !errors.Is(err, late) {
		t.Fatal(err)
	}
	r.mu.Lock()
	retained := r.err
	r.mu.Unlock()
	if !errors.Is(retained, late) {
		t.Fatal("receiver discarded joined write failure", retained)
	}
	// The listener is already joined in this isolated terminal-owner oracle.
	r.ctx, r.cancel = context.WithCancel(context.Background())
	r.listener, r.done = retiredReceiverListener{}, make(chan struct{})
	close(r.done)
	for range 2 {
		if err := r.Close(); !errors.Is(err, late) {
			t.Fatal("receiver Close replaced joined failure", err)
		}
	}
}

func TestReceiverDoesNotRetainPeerSessionRefusal(t *testing.T) {
	r := &Receiver{}
	s := framing.New(context.Background(), newLifecycleConn(false), time.Now().Add(5*time.Second), 32<<20, nil, false, framing.NewBudget(64<<20), nil)
	refusal := errors.New("peer session refused")
	s.Retire(refusal)
	if err := r.finishSession(s); !errors.Is(err, refusal) {
		t.Fatal(err)
	}
	if r.err != nil {
		t.Fatal("peer refusal poisoned receiver", r.err)
	}
}
