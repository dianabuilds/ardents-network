//go:build linux

package transport

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/successor/route/carrier"
)

// ReceiverConfig binds one actual new receiving forwarding duty. Admit must
// call receiving.Owner.Accept with its genuine Hosting capacity callback.
type ReceiverConfig struct {
	Authority   Authority
	Certificate tls.Certificate
	Admit       Admit
}

// Receiver owns its listener, in-flight handshakes, admitted parents and all
// forwarding descendants. Admission/spend roots are closed by composition only
// after Close returns its retained joined result.
type Receiver struct {
	config        ReceiverConfig
	listener      carrier.ClosedSharedCarrierListener
	ctx           context.Context
	cancel        context.CancelFunc
	mu            sync.Mutex
	connections   map[net.Conn]struct{}
	workers       sync.WaitGroup
	done          chan struct{}
	once          sync.Once
	listenerOnce  sync.Once
	listenerErr   error
	queues        *queueBudget
	pool          nodePool
	err, closeErr error
}

func Listen(ctx context.Context, config ReceiverConfig) (*Receiver, error) {
	if ctx == nil || config.Admit == nil {
		return nil, errors.New("route receiving composition absent")
	}
	m, err := config.Authority.member()
	if err != nil {
		return nil, err
	}
	key, ok := config.Certificate.PrivateKey.(ed25519.PrivateKey)
	if !ok || len(key) != ed25519.PrivateKeySize || string(key.Public().(ed25519.PublicKey)) != string(m.PublicKey[:]) ||
		(m.Subrole != 1 && m.Subrole != 2) || (m.RoleDomain != 1 && m.RoleDomain != 3 && m.RoleDomain != 4) {
		return nil, errors.New("route receiving duty or private key differs")
	}
	child, cancel := context.WithCancel(ctx)
	r := &Receiver{config: config, ctx: child, cancel: cancel, connections: make(map[net.Conn]struct{}), done: make(chan struct{}), queues: &queueBudget{maximum: 64 << 20}}
	r.listener, err = carrier.ListenClosedSharedCarrier(carrier.CarrierProfile(m.CarrierProfile), m.Endpoint, config.Certificate, func(key [32]byte) bool {
		_, err := r.peer(key)
		return err == nil
	}, 16)
	if err != nil {
		cancel()
		return nil, err
	}
	go r.run()
	return r, nil
}

func (r *Receiver) peer(key [32]byte) (network.Member, error) {
	m, err := r.config.Authority.member()
	if err != nil {
		return network.Member{}, err
	}
	v, err := r.config.Authority.Current()
	if err != nil {
		return network.Member{}, err
	}
	peer, err := v.MemberByKey(key, time.Now())
	if err != nil || conflicting(m, peer) || peer.RoleDomain != m.RoleDomain || peer.Subrole != 1 {
		return network.Member{}, errors.Join(errors.New("route adjacent Node peer unavailable"), err)
	}
	return peer, nil
}

func (r *Receiver) run() {
	defer close(r.done)
	watchDone := make(chan struct{})
	go func() {
		defer close(watchDone)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-r.ctx.Done():
				r.interrupt()
				return
			case <-ticker.C:
				if _, err := r.config.Authority.member(); err != nil {
					r.record(err)
					r.cancel()
				}
			}
		}
	}()
	defer func() { r.cancel(); r.interrupt(); r.workers.Wait(); r.record(r.pool.Close()); <-watchDone }()
	for {
		accepted, err := r.listener.Accept(r.ctx, 10*time.Second)
		if err != nil {
			if carrier.IsClosedSharedPeerFailure(err) {
				continue
			}
			if r.ctx.Err() == nil {
				r.record(err)
			}
			return
		}
		accepted.Connection = &retiredConn{Conn: accepted.Connection}
		r.mu.Lock()
		if r.ctx.Err() != nil || len(r.connections) >= 1024 {
			r.mu.Unlock()
			r.record(accepted.Connection.Close())
			continue
		}
		r.connections[accepted.Connection] = struct{}{}
		r.workers.Add(1)
		r.mu.Unlock()
		go func() {
			defer r.workers.Done()
			conn := accepted.Connection
			defer func() { r.record(conn.Close()); r.mu.Lock(); delete(r.connections, conn); r.mu.Unlock() }()
			if accepted.Kind == carrier.ClosedSharedDirect {
				m, err := r.config.Authority.member()
				if err == nil && m.Subrole == 1 {
					_ = r.serveRole(r.ctx, conn, nil, nil)
				}
			} else {
				_ = r.serveOuter(r.ctx, accepted)
			}
		}()
	}
}

func (r *Receiver) record(err error) {
	if err != nil {
		r.mu.Lock()
		r.err = errors.Join(r.err, err)
		r.mu.Unlock()
	}
}
func (r *Receiver) interrupt() {
	// Listener close must interrupt even a handshake waiting for bytes. Owned
	// accepted sockets are separately interrupted; each worker joins below.
	_ = r.closeListener()
	r.pool.interrupt()
	r.mu.Lock()
	for conn := range r.connections {
		_ = conn.SetDeadline(time.Now())
		_ = conn.Close()
	}
	r.mu.Unlock()
}

// Done signals joined listener retirement. Close retains the terminal result.
func (r *Receiver) Done() <-chan struct{} { return r.done }
func (r *Receiver) Close() error {
	if r == nil {
		return nil
	}
	r.once.Do(func() {
		r.cancel()
		r.interrupt()
		<-r.done
		r.mu.Lock()
		r.closeErr = errors.Join(r.err, r.closeListener())
		r.mu.Unlock()
	})
	return r.closeErr
}

func (r *Receiver) serveOuter(ctx context.Context, accepted carrier.ClosedSharedCarrier) (result error) {
	releaseControl, err := r.queues.channel()
	if err != nil {
		return err
	}
	defer releaseControl()
	conn := accepted.Connection
	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	h, err := readHello(conn)
	if err != nil {
		return err
	}
	if _, err := r.config.Authority.hello(h, true); err != nil {
		return err
	}
	peer, err := r.peer(accepted.NodeKey)
	if err != nil {
		return err
	}
	if err := acceptChannel(conn); err != nil {
		return err
	}
	if err := conn.SetDeadline(h.Deadline); err != nil {
		return err
	}
	check := func() error {
		if _, err := r.config.Authority.hello(h, true); err != nil {
			return err
		}
		current, err := r.peer(accepted.NodeKey)
		if err != nil || current.RecordDigest != peer.RecordDigest || current.DutyGeneration != peer.DutyGeneration {
			return errors.Join(errors.New("route Node peer changed"), err)
		}
		return nil
	}
	s := newSession(ctx, conn, h.Deadline, 32<<20, check, true, r.queues, func(ctx context.Context, l *lane, body []byte) error {
		if len(body) != 50 || body[49] != 0 {
			return errors.New("route Node OPEN restriction unavailable")
		}
		opened, err := decodeOpen(body[:49])
		if err != nil {
			return err
		}
		if opened.RecipientNodeID != h.RecipientNodeID || opened.RecipientDutyGeneration != h.RecipientDutyGeneration || opened.Purpose != 7 || opened.Deadline.After(h.Deadline) {
			return errors.New("route Node child binding differs")
		}
		if err := l.bound(opened.Deadline); err != nil {
			return err
		}
		m, err := r.config.Authority.member()
		if err != nil || m.Subrole != 2 {
			return errors.Join(errors.New("route Interior child unavailable"), err)
		}
		secured, err := carrier.AcceptClosedRoleTLS(ctx, l, r.config.Certificate, minDeadline(opened.Deadline, time.Now().Add(10*time.Second)))
		if err != nil {
			return err
		}
		if err := l.beginRole(); err != nil {
			return err
		}
		return r.serveRole(ctx, &retiredConn{Conn: secured}, l, &opened)
	})
	<-s.readerDone
	return s.Close()
}

func (r *Receiver) serveRole(ctx context.Context, conn net.Conn, outer *lane, opened *ardpHello) (result error) {
	releaseControl, err := r.queues.channel()
	if err != nil {
		return err
	}
	defer releaseControl()
	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	capacity := &admissionRetirement{}
	grant, h, err := receiveChannel(ctx, conn, r.config.Authority, r.config.Admit, opened, capacity)
	var s *session
	defer func() {
		// Interrupt nested physical work, join every borrower, then return the
		// finite reservation. A retained token spend is never refunded here.
		if s != nil {
			result = errors.Join(result, s.Close())
		} else {
			result = errors.Join(result, conn.Close())
		}
		releaseErr := errors.Join(capacity.finish(), grant.Release())
		r.record(releaseErr)
		result = errors.Join(result, releaseErr)
	}()
	if err != nil {
		return err
	}
	if opened != nil && (h.RecipientNodeID != opened.RecipientNodeID || h.RecipientDutyGeneration != opened.RecipientDutyGeneration || uint8(h.Purpose) != opened.Purpose || h.Deadline.After(opened.Deadline)) {
		return errors.New("route inner HELLO differs from OPEN")
	}
	if err := conn.SetDeadline(h.Deadline); err != nil {
		return err
	}
	if outer != nil {
		if err := outer.admit(h.Deadline); err != nil {
			return err
		}
	}
	if err := acceptChannel(conn); err != nil {
		return err
	}
	check := func() error { _, err := r.config.Authority.hello(h, false); return err }
	if err := check(); err != nil {
		return err
	}
	s = newSession(ctx, conn, h.Deadline, grant.Allowance().Bytes()-admissionWireBytes, check, false, r.queues, func(ctx context.Context, l *lane, body []byte) error {
		return r.forward(ctx, l, h, body)
	})
	<-s.readerDone
	return nil
}

// ardpHello uses the same exact OPEN fields without inventing a wire identity.
type ardpHello struct {
	RecipientNodeID         [32]byte
	RecipientDutyGeneration uint64
	Purpose                 uint8
	Deadline                time.Time
}

func decodeOpen(body []byte) (ardpHello, error) {
	if len(body) != 49 {
		return ardpHello{}, errors.New("route OPEN length invalid")
	}
	o := ardpHello{}
	copy(o.RecipientNodeID[:], body[:32])
	o.RecipientDutyGeneration = binary.BigEndian.Uint64(body[32:40])
	o.Purpose = body[40]
	o.Deadline = time.Unix(int64(binary.BigEndian.Uint64(body[41:])), 0).UTC()
	if o.RecipientNodeID == [32]byte{} || o.RecipientDutyGeneration == 0 || o.Purpose != 7 || !time.Now().Before(o.Deadline) {
		return ardpHello{}, errors.New("route OPEN facts invalid")
	}
	return o, nil
}
func encodeOpen(o ardpHello, node bool) []byte {
	body := append([]byte(nil), o.RecipientNodeID[:]...)
	body = binary.BigEndian.AppendUint64(body, o.RecipientDutyGeneration)
	body = append(body, o.Purpose)
	body = binary.BigEndian.AppendUint64(body, uint64(o.Deadline.Unix()))
	if node {
		body = append(body, 0)
	}
	return body
}
func minDeadline(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}

func (r *Receiver) forward(ctx context.Context, source *lane, parent ardp.Hello, body []byte) (result error) {
	local, err := r.config.Authority.hello(parent, false)
	if err != nil {
		return err
	}
	opened, err := decodeOpen(body)
	if err != nil {
		return err
	}
	if local.Subrole != 1 || opened.Deadline.After(parent.Deadline) {
		return errors.New("route prefix next-hop unavailable")
	}
	if err := source.bound(opened.Deadline); err != nil {
		return err
	}
	v, err := r.config.Authority.Current()
	if err != nil {
		return err
	}
	next, err := v.Member(opened.RecipientNodeID, time.Now())
	if err != nil || next.Subrole != 2 || next.RoleDomain != local.RoleDomain || next.DutyGeneration != opened.RecipientDutyGeneration || conflicting(local, next) {
		return errors.Join(errors.New("route next Interior unavailable"), err)
	}
	duty, err := v.RetainDuty(next.NodeID, time.Now())
	if err != nil {
		return err
	}
	a := Authority{Current: r.config.Authority.Current, Duty: duty, Profile: v.Profile().ProfileBinding}
	if _, err := a.member(); err != nil {
		return err
	}
	pooled, err := r.pool.acquire(ctx, a, func(ctx context.Context) (*session, func(), error) { return r.openNode(ctx, a) })
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, pooled.release()) }()
	downstream := pooled.session
	target, err := downstream.openLane(ctx, encodeOpen(opened, true))
	if err != nil {
		return err
	}
	defer target.finish()
	// Cancellation retires only this child, while healthy sibling borrowers
	// retain the same authenticated Carrier and their physical reservation.
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(interrupted); _ = target.Close(); _ = source.Close() })
	defer func() {
		if !stop() {
			<-interrupted
		}
	}()
	results := make(chan error, 2)
	go func() {
		_, err := io.Copy(target, source)
		if err == nil {
			err = target.CloseWrite()
		}
		results <- err
	}()
	go func() {
		_, err := io.Copy(source, target)
		if err == nil {
			err = source.CloseWrite()
		}
		results <- err
	}()
	first := <-results
	if first != nil {
		_ = target.Close()
		_ = source.Close()
	}
	second := <-results
	return errors.Join(first, second, target.Close(), source.Close())
}

func (r *Receiver) openNode(ctx context.Context, a Authority) (_ *session, release func(), result error) {
	next, err := a.member()
	if err != nil {
		return nil, nil, err
	}
	local, err := r.config.Authority.member()
	if err != nil {
		return nil, nil, err
	}
	end := minDeadline(minDeadline(next.NotAfter(), local.NotAfter()), a.Profile.NotAfter)
	control, err := r.queues.channel()
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if result != nil {
			control()
		}
	}()
	physical, err := carrier.OpenClosedNodeCarrier(ctx, carrier.ClosedNodeCarrierRequest{CarrierProfile: carrier.CarrierProfile(next.CarrierProfile), Endpoint: next.Endpoint, Certificate: r.config.Certificate, ExpectedPeerKey: next.PublicKey, Deadline: minDeadline(end, time.Now().Add(10*time.Second))})
	if err != nil {
		return nil, nil, err
	}
	conn, ok := physical.(net.Conn)
	if !ok {
		return nil, nil, errors.Join(errors.New("route Node Carrier missing connection contract"), physical.Close())
	}
	conn = &retiredConn{Conn: conn}
	interrupted := make(chan struct{})
	sealed := false
	stop := context.AfterFunc(ctx, func() { defer close(interrupted); _ = conn.SetDeadline(time.Now()); _ = conn.Close() })
	defer func() {
		if !sealed && !stop() {
			<-interrupted
		}
	}()
	defer func() {
		if result != nil {
			result = errors.Join(result, conn.Close())
		}
	}()
	h, err := freshHello(a, end)
	if err != nil {
		return nil, nil, err
	}
	if err := conn.SetDeadline(minDeadline(h.Deadline, time.Now().Add(10*time.Second))); err != nil {
		return nil, nil, err
	}
	if err := sendHello(conn, h); err != nil {
		return nil, nil, err
	}
	if err := acceptedChannel(conn); err != nil {
		return nil, nil, err
	}
	if _, err := a.member(); err != nil {
		return nil, nil, err
	}
	if err := conn.SetDeadline(h.Deadline); err != nil {
		return nil, nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	// Seal setup cancellation before publishing into the shared pool.
	if !stop() {
		<-interrupted
		return nil, nil, ctx.Err()
	}
	sealed = true
	check := func() error {
		if _, err := r.config.Authority.member(); err != nil {
			return err
		}
		_, err := a.member()
		return err
	}
	if err := check(); err != nil {
		return nil, nil, err
	}
	return newSession(r.ctx, conn, h.Deadline, 32<<20, check, false, r.queues, nil), control, nil
}
