package quic

import (
	"crypto/rand"
	"net"
	"slices"
	"time"

	"github.com/quic-go/quic-go/internal/ackhandler"
	"github.com/quic-go/quic-go/internal/monotime"
	"github.com/quic-go/quic-go/internal/protocol"
	"github.com/quic-go/quic-go/internal/utils"
	"github.com/quic-go/quic-go/internal/wire"
)

type pathID int64

const invalidPathID pathID = -1

// Maximum number of paths to keep track of.
// If the peer probes another path (before the pathTimeout of an existing path expires),
// this probing attempt is ignored.
const maxPaths = 3

// If no packet is received for a path for pathTimeout,
// the path can be evicted when the peer probes another path.
// This prevents an attacker from churning through paths by duplicating packets and
// sending them with spoofed source addresses.
const pathTimeout = 5 * time.Second

// amplificationFactor is the anti-amplification factor, see section 8 of RFC 9000.
const amplificationFactor = 3

type sentChallenge struct {
	data [8]byte
	// The datagram containing the PATH_CHALLENGE couldn't be expanded to 1200 bytes, because of the
	// anti-amplification limit. Its PATH_RESPONSE only validates the peer's address, not the path MTU
	// (section 8.2.1 of RFC 9000).
	notExpanded bool
}

type path struct {
	id             pathID
	addr           net.Addr
	lastPacketTime monotime.Time
	pathChallenges []sentChallenge // length is implicitly limited by exponential backoff
	nextProbeTime  monotime.Time
	// The peer's address was validated by a PATH_RESPONSE to a PATH_CHALLENGE that couldn't be expanded.
	// The anti-amplification limit doesn't apply to this path anymore.
	addrValidated bool
	// A PATH_CHALLENGE in an expanded datagram needs to be sent, to validate the path MTU.
	challengeDue   bool
	validated      bool
	rcvdNonProbing bool
}

type pathManager struct {
	nextPathID pathID
	// ordered by lastPacketTime, with the most recently used path at the end
	paths []*path

	getConnID    func(pathID) (_ protocol.ConnectionID, ok bool)
	retireConnID func(pathID)

	logger utils.Logger
}

func newPathManager(
	getConnID func(pathID) (_ protocol.ConnectionID, ok bool),
	retireConnID func(pathID),
	logger utils.Logger,
) *pathManager {
	return &pathManager{
		paths:        make([]*path, 0, maxPaths+1),
		getConnID:    getConnID,
		retireConnID: retireConnID,
		logger:       logger,
	}
}

// Returns a path challenge frame if one should be sent.
// May return nil.
// The path must have been created by GetOrCreatePath.
func (pm *pathManager) HandlePacket(
	remoteAddr net.Addr,
	t monotime.Time,
	pathChallenge *wire.PathChallengeFrame, // may be nil if the packet didn't contain a PATH_CHALLENGE
	isNonProbing bool,
) (_ protocol.ConnectionID, _ []ackhandler.Frame, shouldSwitch bool) {
	var p *path
	for i, path := range pm.paths {
		if addrsEqual(path.addr, remoteAddr) {
			p = path
			p.lastPacketTime = t
			if isNonProbing {
				path.rcvdNonProbing = true
			}
			if pm.logger.Debug() {
				pm.logger.Debugf("received packet for path %s, validated: %t", remoteAddr, path.validated)
			}
			shouldSwitch = path.validated && path.rcvdNonProbing
			if i != len(pm.paths)-1 {
				// move the path to the end of the list
				pm.paths = slices.Delete(pm.paths, i, i+1)
				pm.paths = append(pm.paths, p)
			}
			// Nothing to send:
			// * the packet didn't contain a PATH_CHALLENGE, and
			// * the path does not need a probe (already validated, or still inside the backoff).
			if pathChallenge == nil && (path.validated || (!path.challengeDue && t.Before(path.nextProbeTime))) {
				return protocol.ConnectionID{}, nil, shouldSwitch
			}
			break
		}
	}

	if p == nil {
		return protocol.ConnectionID{}, nil, shouldSwitch
	}

	connID, ok := pm.getConnID(p.id)
	if !ok {
		pm.logger.Debugf("skipping validation of new path %s since no connection ID is available", remoteAddr)
		return protocol.ConnectionID{}, nil, shouldSwitch
	}

	frames := make([]ackhandler.Frame, 0, 2)
	// Probe an unvalidated path with a new PATH_CHALLENGE:
	// * sent on the next packet once nextProbeTime has passed, and
	// * repeated until a PATH_RESPONSE arrives, ignoring ACKs and loss.
	// A PATH_RESPONSE to a PATH_CHALLENGE that couldn't be expanded to 1200 bytes only validated the peer's address.
	// The next PATH_CHALLENGE is sent right away, in an expanded datagram (section 8.2.1 of RFC 9000).
	if !p.validated && (p.challengeDue || !t.Before(p.nextProbeTime)) {
		backoffFactor := 1 << len(p.pathChallenges)
		p.nextProbeTime = t.Add(utils.DefaultInitialRTT * time.Duration(backoffFactor))
		p.challengeDue = false
		var b [8]byte
		rand.Read(b[:])
		p.pathChallenges = append(p.pathChallenges, sentChallenge{data: b})
		frames = append(frames, ackhandler.Frame{
			Frame:   &wire.PathChallengeFrame{Data: b},
			Handler: emptyHandler{},
		})
		pm.logger.Debugf("enqueueing PATH_CHALLENGE for new path %s", remoteAddr)
	}
	if pathChallenge != nil {
		frames = append(frames, ackhandler.Frame{
			Frame:   &wire.PathResponseFrame{Data: pathChallenge.Data},
			Handler: emptyHandler{},
		})
	}
	return connID, frames, shouldSwitch
}

// GetOrCreatePath returns the ID of the path for the remote address, creating the path if necessary.
// It returns false if the path limit is reached.
func (pm *pathManager) GetOrCreatePath(remoteAddr net.Addr, t monotime.Time) (_ pathID, isNew, ok bool) {
	for _, p := range pm.paths {
		if addrsEqual(p.addr, remoteAddr) {
			return p.id, false, true
		}
	}

	// new path while already at maxPaths
	if len(pm.paths) >= maxPaths {
		// ignore it if the oldest path received a packet within pathTimeout
		if pm.paths[0].lastPacketTime.Add(pathTimeout).After(t) {
			if pm.logger.Debug() {
				pm.logger.Debugf("received packet for previously unseen path %s, but already have %d paths", remoteAddr, len(pm.paths))
			}
			return invalidPathID, false, false
		}
		// evict the oldest path, if the last packet was received more than pathTimeout ago
		pm.retireConnID(pm.paths[0].id)
		pm.paths = pm.paths[1:]
	}

	p := &path{
		id:             pm.nextPathID,
		addr:           remoteAddr,
		lastPacketTime: t,
		nextProbeTime:  t,
		pathChallenges: make([]sentChallenge, 0, 2),
	}
	pm.nextPathID++
	pm.paths = append(pm.paths, p)
	return p.id, true, true
}

func (pm *pathManager) HandlePathResponseFrame(f *wire.PathResponseFrame) {
	for _, p := range pm.paths {
		for i, ch := range p.pathChallenges {
			if ch.data != f.Data {
				continue
			}
			if ch.notExpanded {
				// Section 8.2.3 of RFC 9000: the peer's address is validated, but not the path MTU.
				// The anti-amplification limit doesn't apply to this path anymore, so the next PATH_CHALLENGE
				// can be sent in an expanded datagram.
				p.pathChallenges = slices.Delete(p.pathChallenges, i, i+1)
				p.addrValidated = true
				// Another PATH_CHALLENGE is only needed if no expanded one is outstanding.
				p.challengeDue = !slices.ContainsFunc(p.pathChallenges, func(c sentChallenge) bool { return !c.notExpanded })
				pm.logger.Debugf("address %s validated, validating the path MTU", p.addr)
			} else {
				// path validated
				p.validated = true
				p.pathChallenges = nil
				p.challengeDue = false
				pm.logger.Debugf("path %s validated", p.addr)
			}
			return
		}
	}
}

// ChallengeNotExpanded is called when the datagram containing the PATH_CHALLENGE frame couldn't be expanded
// to 1200 bytes, because of the anti-amplification limit (section 8.2.1 of RFC 9000).
func (pm *pathManager) ChallengeNotExpanded(data [8]byte) {
	for _, p := range pm.paths {
		for i := range p.pathChallenges {
			if p.pathChallenges[i].data == data {
				p.pathChallenges[i].notExpanded = true
				return
			}
		}
	}
}

// AddrValidated says if the peer's address of a path was validated, see HandlePathResponseFrame.
// Packets sent to a validated address are not limited by the anti-amplification limit.
func (pm *pathManager) AddrValidated(remoteAddr net.Addr) bool {
	for _, p := range pm.paths {
		if addrsEqual(p.addr, remoteAddr) {
			return p.addrValidated
		}
	}
	return false
}

// SwitchToPath is called when the connection switches to a new path
func (pm *pathManager) SwitchToPath(addr net.Addr) {
	// retire all other paths
	for _, path := range pm.paths {
		if addrsEqual(path.addr, addr) {
			pm.logger.Debugf("switching to path %d (%s)", path.id, addr)
			continue
		}
		pm.retireConnID(path.id)
	}
	clear(pm.paths)
	pm.paths = pm.paths[:0]
}

func addrsEqual(addr1, addr2 net.Addr) bool {
	if addr1 == nil || addr2 == nil {
		return false
	}
	a1, ok1 := addr1.(*net.UDPAddr)
	a2, ok2 := addr2.(*net.UDPAddr)
	if ok1 && ok2 {
		return a1.IP.Equal(a2.IP) && a1.Port == a2.Port
	}
	return addr1.String() == addr2.String()
}
