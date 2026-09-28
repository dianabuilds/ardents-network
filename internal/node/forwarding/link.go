package forwarding

import (
	"context"
	"encoding/binary"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/carrier"
)

// link owns exactly one admitted child and its selected next
// Carrier. It is deliberately not a route cache: every link begins with a
// fresh outer HELLO/OPEN and closes with its child.
type forwardLink struct {
	session     *session
	remoteLane  uint32
	localLane   uint32
	reverse     *frameQueue
	lease       *carrier.ClosedCarrierLease
	write       func(ardp.Frame) error
	forward     sync.Mutex
	forwardDone chan struct{}
	deadline    time.Time
	once        sync.Once
	channel     *route.ClosedForwardingChannel
	done        chan struct{}
	stopped     chan struct{}
	abort       func()
	stopErr     error
	forwarding  bool
	stopping    bool
	forwardErr  error
}

type openResult struct {
	lane uint32
	link *forwardLink
	err  error
}

func nonCancellationError(err error) error {
	if err == nil || errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		joined, ok := err.(interface{ Unwrap() []error })
		if !ok {
			return nil
		}
		var result error
		for _, item := range joined.Unwrap() {
			if retained := nonCancellationError(item); retained != nil {
				result = errors.Join(result, retained)
			}
		}
		return result
	}
	return err
}

// openings owns bounded child OPEN work while the parent keeps
// reading its accounted control queue. Its caller collects results before it
// uses a child and cancels then joins every unfinished opener at teardown.
type openings struct {
	ctx     context.Context
	cancel  context.CancelFunc
	results chan openResult
	pending map[uint32]context.CancelFunc
	wake    func()
}

func newOpenings(parent context.Context, wake ...func()) *openings {
	ctx, cancel := context.WithCancel(parent)
	var notify func()
	if len(wake) != 0 {
		notify = wake[0]
	}
	return &openings{ctx: ctx, cancel: cancel, results: make(chan openResult, 256), pending: make(map[uint32]context.CancelFunc), wake: notify}
}

func (openings *openings) start(server *forwardServer, event route.ClosedForwardingEvent, channel *route.ClosedForwardingChannel, write func(ardp.Frame) error, abort func()) {
	child, cancel := context.WithCancel(openings.ctx)
	openings.pending[event.Lane] = cancel
	go func() {
		link, err := server.openForwardingLink(child, event.Open, event.Restriction, event.Lane, channel, write, abort)
		if err != nil && child.Err() == nil {
			abort()
		}
		openings.results <- openResult{lane: event.Lane, link: link, err: err}
		if openings.wake != nil {
			openings.wake()
		}
	}()
}

func (openings *openings) collect(links map[uint32]*forwardLink) error {
	if openings == nil {
		return nil
	}
	for {
		select {
		case result := <-openings.results:
			delete(openings.pending, result.lane)
			if result.err != nil {
				var closeErr error
				if result.link != nil {
					closeErr = result.link.close()
				}
				return errors.Join(result.err, closeErr)
			}
			links[result.lane] = result.link
		default:
			return nil
		}
	}
}

func (openings *openings) close(links map[uint32]*forwardLink) error {
	if openings == nil {
		return nil
	}
	openings.cancel()
	var result error
	for _, cancel := range openings.pending {
		cancel()
	}
	for len(openings.pending) != 0 {
		opened := <-openings.results
		delete(openings.pending, opened.lane)
		if opened.link != nil {
			result = errors.Join(result, opened.link.close())
		}
		if retained := nonCancellationError(opened.err); retained != nil {
			result = errors.Join(result, retained)
		}
	}
	clear(openings.pending)
	return result
}

func (openings *openings) cancelLane(lane uint32, links map[uint32]*forwardLink) error {
	if openings == nil {
		return nil
	}
	cancel, pending := openings.pending[lane]
	if !pending {
		return nil
	}
	cancel()
	for {
		opened := <-openings.results
		delete(openings.pending, opened.lane)
		if opened.lane == lane {
			var result error
			if opened.link != nil {
				result = errors.Join(result, opened.link.close())
			}
			if retained := nonCancellationError(opened.err); retained != nil {
				result = errors.Join(result, retained)
			}
			return result
		}
		if opened.err != nil {
			return opened.err
		}
		links[opened.lane] = opened.link
	}
}

func (link *forwardLink) availableForForwarding() bool {
	link.forward.Lock()
	defer link.forward.Unlock()
	return !link.forwarding && !link.stopping
}

func (link *forwardLink) availableForClose() bool {
	link.forward.Lock()
	defer link.forward.Unlock()
	return link.stopping || !link.forwarding
}

func (link *forwardLink) startForwarding(event route.ClosedForwardingEvent, wake func(), abort func()) bool {
	link.forward.Lock()
	if link.forwarding || link.stopping {
		link.forward.Unlock()
		return false
	}
	link.forwarding = true
	done := make(chan struct{})
	link.forwardDone = done
	link.forward.Unlock()
	frame := ardp.Frame{Kind: event.Kind, Lane: link.remoteLane, Body: append([]byte(nil), event.Bytes...)}
	go func() {
		written, err := link.session.writeChildFrame(frame, link.deadline, link.reverse)
		if err == nil && event.Kind == 6 && written {
			err = link.lease.MarkUsed()
		}
		link.forward.Lock()
		link.forwardErr = errors.Join(link.forwardErr, err)
		link.forwarding = false
		close(done)
		link.forward.Unlock()
		if err != nil {
			if abort != nil {
				abort()
			}
		}
		if wake != nil {
			wake()
		}
	}()
	return true
}

func (link *forwardLink) forwardingError() error {
	link.forward.Lock()
	defer link.forward.Unlock()
	err := link.forwardErr
	link.forwardErr = nil
	return err
}

// eventAvailable keeps a queued child event accounted until
// that child's one physical writer is free. In particular, a CLOSE must not
// enter the synchronous retirement path while an earlier BYTES write for the
// same child is blocked: that would stall unrelated children behind it.
func eventAvailable(event route.ClosedForwardingEvent, links map[uint32]*forwardLink) bool {
	if event.Kind == 4 { // OPEN has no link yet.
		return true
	}
	link := links[event.Lane]
	if event.Kind == 9 && link != nil {
		return link.availableForClose()
	}
	return link != nil && link.availableForForwarding()
}

func (server *forwardServer) drainForwarding(ctx context.Context, channel *route.ClosedForwardingChannel, links map[uint32]*forwardLink, openings *openings, write func(ardp.Frame) error, abort func()) error {
	for {
		if err := openings.collect(links); err != nil {
			return err
		}
		for _, link := range links {
			if err := link.forwardingError(); err != nil {
				return err
			}
		}
		event, available := channel.NextAvailable(func(event route.ClosedForwardingEvent) bool {
			if event.Kind == 9 && openings != nil {
				if _, pending := openings.pending[event.Lane]; pending {
					return true
				}
			}
			return eventAvailable(event, links)
		})
		if !available {
			return nil
		}
		switch event.Kind {
		case 4: // OPEN
			pending := false
			if openings != nil {
				_, pending = openings.pending[event.Lane]
			}
			if links[event.Lane] != nil || (openings != nil && pending) {
				return errors.New("closed forwarding child is reused")
			}
			if openings != nil {
				openings.start(server, event, channel, write, abort)
				continue
			}
			link, err := server.openForwardingLink(ctx, event.Open, event.Restriction, event.Lane, channel, write, abort)
			if err != nil {
				return err
			}
			links[event.Lane] = link
		case 6, 7, 8: // BYTES / CREDIT / EOF
			link := links[event.Lane]
			if link == nil {
				return errors.New("closed forwarding child is unavailable")
			}
			if openings != nil {
				if !link.startForwarding(event, openings.wake, abort) {
					return errors.New("closed forwarding child is unavailable")
				}
				continue
			}
			frame := ardp.Frame{Kind: event.Kind, Lane: link.remoteLane, Body: append([]byte(nil), event.Bytes...)}
			written, err := link.session.writeChildFrame(frame, link.deadline, link.reverse)
			if err == nil && event.Kind == 6 && written {
				err = link.lease.MarkUsed()
			}
			if err != nil {
				return err
			}
		case 9: // CLOSE
			if openings != nil {
				if _, pending := openings.pending[event.Lane]; pending {
					if err := openings.cancelLane(event.Lane, links); err != nil {
						return err
					}
					continue
				}
			}
			link := links[event.Lane]
			if link == nil {
				return errors.New("closed forwarding child is unavailable")
			}
			_, err := link.session.writeChildFrame(ardp.Frame{Kind: event.Kind, Lane: link.remoteLane, Body: append([]byte(nil), event.Bytes...)}, link.deadline, link.reverse)
			if err != nil {
				return err
			}
			delete(links, event.Lane)
			if err := link.close(); err != nil {
				return err
			}
		default:
			return errors.New("closed forwarding event is unavailable")
		}
	}
}

func (server *forwardServer) openForwardingLink(ctx context.Context, open route.ClosedOpen, restriction route.ClosedChildRestriction, lane uint32, channel *route.ClosedForwardingChannel, write func(ardp.Frame) error, abort func()) (*forwardLink, error) {
	handshakeDeadline := closedForwardingHandshakeDeadline(open.Deadline, server.clock().UTC())
	handshakeCtx, cancelHandshake, err := closedForwardingHandshakeContext(ctx, handshakeDeadline)
	if err != nil {
		return nil, err
	}
	defer cancelHandshake()
	updated, err := server.dependencies.current()
	if err != nil {
		return nil, err
	}
	candidate, err := recipient(server.dependencies.authority, updated, open, server.clock(), server.dependencies.literalEndpoint)
	if err != nil {
		return nil, err
	}
	receiver, available := server.dependencies.authority.Receiver(updated, ardp.PurposeForwarding, server.clock())
	if !available {
		return nil, errors.New("closed forwarding receiver is unavailable")
	}
	dialEndpoint, err := dialAddress(candidate.Endpoint, server.dependencies.relayEndpoint)
	if err != nil {
		return nil, err
	}
	key := carrier.ClosedCarrierKey{NetworkID: receiver.NetworkID, ProfileDigest: receiver.ProfileDigest, LocalNodeID: receiver.NodeID,
		PeerNodeID: candidate.NodeID, PeerKey: candidate.PublicKey, CarrierProfile: carrier.CarrierProfile(candidate.CarrierProfile)}
	lease, err := server.pool.AcquireContext(handshakeCtx, key, func() error {
		fresh, readErr := server.dependencies.current()
		if readErr != nil {
			return readErr
		}
		selected, selectErr := recipient(server.dependencies.authority, fresh, open, server.clock(), server.dependencies.literalEndpoint)
		if selectErr != nil || selected.NodeID != candidate.NodeID || selected.PublicKey != candidate.PublicKey || selected.RecordDigest != candidate.RecordDigest ||
			selected.Endpoint != candidate.Endpoint || selected.CarrierProfile != candidate.CarrierProfile {
			return errors.New("closed forwarding recipient changed")
		}
		return nil
	}, func() (carrier.Carrier, error) {
		return carrier.OpenClosedNodeCarrier(handshakeCtx, carrier.ClosedNodeCarrierRequest{CarrierProfile: carrier.CarrierProfile(candidate.CarrierProfile), Endpoint: dialEndpoint,
			Certificate: server.certificate, ExpectedPeerKey: candidate.PublicKey, Deadline: handshakeDeadline})
	})
	if err != nil {
		return nil, err
	}
	session, err := server.sessions.acquire(handshakeCtx, key, lease, handshakeDeadline, func() (ardp.Hello, error) {
		return server.outerHello(updated, open)
	})
	if err != nil {
		_ = lease.Release()
		return nil, err
	}
	remoteLane, reverse, err := session.attach(open, restriction, func(frame ardp.Frame) error { frame.Lane = lane; return channel.QueueReverse(frame) }, func() bool { return channel.ReverseRetired(lane) })
	if err != nil {
		_ = lease.Release()
		return nil, err
	}
	link := &forwardLink{session: session, remoteLane: remoteLane, localLane: lane, deadline: open.Deadline, reverse: reverse, lease: lease, write: write, channel: channel, done: make(chan struct{}), stopped: make(chan struct{}), abort: abort}
	go link.copyReverse()
	return link, nil
}

func closedForwardingHandshakeContext(parent context.Context, deadline time.Time) (context.Context, context.CancelFunc, error) {
	if parent == nil || deadline.IsZero() {
		return nil, nil, errors.New("closed forwarding handshake deadline is unavailable")
	}
	ctx, cancel := context.WithDeadline(parent, deadline)
	if err := ctx.Err(); err != nil {
		cancel()
		return nil, nil, err
	}
	return ctx, cancel, nil
}

func (link *forwardLink) copyReverse() {
	defer close(link.done)
	defer link.stop()
	for {
		frame, ok := link.reverse.next()
		if !ok {
			break
		}
		frame.Lane = link.localLane
		reserved := frame
		var err error
		if frame.Kind == 7 {
			if len(frame.Body) != 4 {
				err = errors.New("closed forwarding credit is invalid")
			} else {
				frame, err = link.channel.Credit(link.localLane, binary.BigEndian.Uint32(frame.Body))
			}
		}
		if err == nil {
			err = link.write(frame)
		}
		link.channel.ReleaseReverse(reserved)
		if err != nil {
			if errors.Is(err, route.ErrClosedForwardingChildRetired) {
				return
			}
			link.abort()
			return
		}
		if frame.Kind == 9 {
			return
		}
	}
	select {
	case <-link.stopped:
	default:
		link.abort()
	}
}

func (link *forwardLink) stop() {
	link.once.Do(func() {
		close(link.stopped)
		link.forward.Lock()
		link.stopping = true
		done := link.forwardDone
		link.forward.Unlock()
		link.session.retire(link.remoteLane)
		if done != nil {
			<-done
		}
		link.stopErr = link.lease.Release()
	})
}

func (link *forwardLink) close() error {
	link.stop()
	<-link.done
	return link.stopErr
}
