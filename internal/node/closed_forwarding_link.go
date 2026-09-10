package node

import (
	"context"
	"encoding/binary"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// closedForwardingLink owns exactly one admitted child and its selected next
// Carrier. It is deliberately not a route cache: every link begins with a
// fresh outer HELLO/OPEN and closes with its child.
type closedForwardingLink struct {
	session    *closedForwardingSession
	remoteLane uint32
	localLane  uint32
	reverse    *closedForwardingQueue
	lease      *route.ClosedCarrierLease
	write      func(route.ClosedLaneFrame) error
	work       sync.Mutex
	deadline   time.Time
	once       sync.Once
	channel    *route.ClosedForwardingChannel
	done       chan struct{}
	stopped    chan struct{}
	abort      func()
	stopErr    error
}

func (server *closedForwardingServer) drainForwarding(ctx context.Context, channel *route.ClosedForwardingChannel, links map[uint32]*closedForwardingLink, write func(route.ClosedLaneFrame) error, abort func()) error {
	for {
		event, available := channel.Next()
		if !available {
			return nil
		}
		switch event.Kind {
		case 4: // OPEN
			if links[event.Lane] != nil {
				return errors.New("closed forwarding child is reused")
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
			frame := route.ClosedLaneFrame{Kind: event.Kind, Lane: link.remoteLane, Body: append([]byte(nil), event.Bytes...)}
			link.work.Lock()
			written, err := link.session.writeChildFrame(frame, link.deadline, link.reverse)
			if err == nil && event.Kind == 6 && written {
				err = link.lease.MarkUsed()
			}
			link.work.Unlock()
			if err != nil {
				return err
			}
		case 9: // CLOSE
			link := links[event.Lane]
			if link == nil {
				return errors.New("closed forwarding child is unavailable")
			}
			link.work.Lock()
			_, err := link.session.writeChildFrame(route.ClosedLaneFrame{Kind: event.Kind, Lane: link.remoteLane, Body: append([]byte(nil), event.Bytes...)}, link.deadline, link.reverse)
			link.work.Unlock()
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

func (server *closedForwardingServer) openForwardingLink(ctx context.Context, open route.ClosedOpen, restriction route.ClosedChildRestriction, lane uint32, channel *route.ClosedForwardingChannel, write func(route.ClosedLaneFrame) error, abort func()) (*closedForwardingLink, error) {
	handshakeDeadline := closedForwardingHandshakeDeadline(open.Deadline, server.clock().UTC())
	updated, err := currentFacts(server.config)
	if err != nil {
		return nil, err
	}
	candidate, err := closedForwardRecipient(server.config, updated, open, server.clock())
	if err != nil {
		return nil, err
	}
	receiver, available := closedRouteReceiver(server.config, updated, route.ClosedPurposeForwarding, server.clock())
	if !available {
		return nil, errors.New("closed forwarding receiver is unavailable")
	}
	key := route.ClosedCarrierKey{NetworkID: receiver.NetworkID, ProfileDigest: receiver.ProfileDigest, LocalNodeID: receiver.NodeID,
		PeerNodeID: candidate.NodeID, PeerKey: candidate.PublicKey, CarrierProfile: route.CarrierProfile(candidate.CarrierProfile)}
	lease, err := server.pool.Acquire(key, func() error {
		fresh, readErr := currentFacts(server.config)
		if readErr != nil {
			return readErr
		}
		selected, selectErr := closedForwardRecipient(server.config, fresh, open, server.clock())
		if selectErr != nil || selected.NodeID != candidate.NodeID || selected.PublicKey != candidate.PublicKey || selected.RecordDigest != candidate.RecordDigest ||
			selected.Endpoint != candidate.Endpoint || selected.CarrierProfile != candidate.CarrierProfile {
			return errors.New("closed forwarding recipient changed")
		}
		return nil
	}, func() (route.Carrier, error) {
		return route.OpenClosedNodeCarrier(ctx, route.ClosedNodeCarrierRequest{CarrierProfile: route.CarrierProfile(candidate.CarrierProfile), Endpoint: candidate.Endpoint,
			Certificate: server.certificate, ExpectedPeerKey: candidate.PublicKey, Deadline: handshakeDeadline})
	})
	if err != nil {
		return nil, err
	}
	session, err := server.sessions.acquire(key, lease, handshakeDeadline, func() (route.ClosedHello, error) {
		return server.closedForwardingOuterHello(updated, open)
	})
	if err != nil {
		_ = lease.Release()
		return nil, err
	}
	remoteLane, reverse, err := session.attach(open, restriction, func(frame route.ClosedLaneFrame) error { frame.Lane = lane; return channel.QueueReverse(frame) }, func() bool { return channel.ReverseRetired(lane) })
	if err != nil {
		_ = lease.Release()
		return nil, err
	}
	link := &closedForwardingLink{session: session, remoteLane: remoteLane, localLane: lane, deadline: open.Deadline, reverse: reverse, lease: lease, write: write, channel: channel, done: make(chan struct{}), stopped: make(chan struct{}), abort: abort}
	go link.copyReverse()
	return link, nil
}

func (link *closedForwardingLink) copyReverse() {
	defer close(link.done)
	defer link.stop()
	for {
		frame, ok := link.reverse.next()
		if !ok {
			break
		}
		frame.Lane = link.localLane
		size := uint64(16 + len(frame.Body))
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
		link.channel.ReleaseReverse(link.localLane, size)
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

func (link *closedForwardingLink) stop() {
	link.once.Do(func() {
		close(link.stopped)
		link.session.retire(link.remoteLane)
		link.work.Lock()
		link.stopErr = link.lease.Release()
		link.work.Unlock()
	})
}

func (link *closedForwardingLink) close() error {
	link.stop()
	<-link.done
	return link.stopErr
}
