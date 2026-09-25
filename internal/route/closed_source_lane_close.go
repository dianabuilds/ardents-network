//go:build linux

package route

import (
	"errors"
	"io"
	"net"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/ardp"
)

func (lane *closedSourceLane) Close() error {
	lane.closeOnce.Do(func() {
		cleanupEnd := time.Now().Add(time.Second)
		owner := lane.owner
		owner.mu.Lock()
		lane.closed = true
		// Release this lane's queued readers/writers even while a sibling owns
		// the physical writer. An already active frame must finish or retire
		// the parent because its wire prefix may have been emitted.
		for _, queue := range []*[]*closedSourceWrite{&owner.terminals, &owner.controls, &owner.data} {
			for index := len(*queue) - 1; index >= 0; index-- {
				request := (*queue)[index]
				if request.lane == lane {
					owner.removeQueuedWriteLocked(request, net.ErrClosed)
				}
			}
		}
		owner.releaseQueuedLocked(uint64(len(lane.buffer)))
		clear(lane.buffer)
		lane.buffer = nil
		var activeCredit *closedSourceWrite
		if owner.active != nil && owner.active.lane == lane && owner.active.frame.Kind != ardp.KindClose {
			deadline := time.Now()
			if owner.active.frame.Kind == ardp.KindCredit {
				activeCredit = owner.active
				// This already emitted control frame can finish before CLOSE.
				// Cutting it short needlessly retires the shared physical prefix.
				// Never extend its original write authority or the cleanup bound.
				deadline = cleanupEnd
				if lane.writeEnd.Before(deadline) {
					deadline = lane.writeEnd
				}
				owner.active.end = deadline
			}
			_ = owner.parent.SetWriteDeadline(deadline)
		}
		var opening <-chan struct{}
		if owner.active != nil && owner.active.lane == lane && owner.active.frame.Kind == ardp.KindOpen {
			opening = owner.active.done
		}
		owner.signalLocked()
		owner.mu.Unlock()
		if opening != nil {
			<-opening
		}
		var creditErr error
		if activeCredit != nil {
			<-activeCredit.done
			creditErr = activeCredit.err
		}
		owner.mu.Lock()
		opened, parentEnded := lane.opened, owner.terminal != nil
		peerEndedJoin := owner.retainClosedRead && lane.remoteClosed
		var terminal *closedSourceWrite
		// Observe parent retirement and queue cleanup atomically. If that parent
		// ends before emission starts, its joined physical retirement replaces
		// an undelivered child CLOSE without inventing a cleanup failure.
		if opened && !parentEnded && !peerEndedJoin {
			terminal, lane.closeErr = lane.enqueueLocked(ardp.Frame{Kind: ardp.KindClose, Lane: lane.id, Body: []byte{lane.closeStatus}}, cleanupEnd)
		}
		owner.mu.Unlock()
		if terminal == nil && errors.Is(lane.closeErr, errClosedSourceOutputQueueFull) && owner.retainClosedRead && owner.queueParentEnded() {
			// The outer joined owner already published retirement. Its cause is
			// returned by the joined stream; an inner CLOSE can no longer enter
			// that parent's queue and must not invent a local cleanup failure.
			lane.closeErr = nil
		}
		if terminal != nil {
			lane.closeErr = owner.awaitWrite(terminal)
			if lane.closeErr != nil {
				lane.closeErr = errors.Join(errors.New("closed source CLOSE write failed"), lane.closeErr)
			}
		}
		if terminal != nil && terminal.unwritten {
			// The lower peer has closed cleanly and no physical frame began for
			// this CLOSE. Join that parent before releasing this unused request.
			owner.fail(terminal.err)
		}
		owner.mu.Lock()
		unemitted := owner.terminal != nil && (terminal == nil || !terminal.attempted || terminal.unwritten)
		joinedPeerEnd := owner.retainClosedRead && owner.terminal == io.EOF &&
			lane.remoteClosed && lane.failure == io.EOF && owner.framedParent != nil
		owner.mu.Unlock()
		joinedLowerClose, joinedLowerCredit := false, false
		if (lane.closeErr != nil || creditErr != nil) && joinedPeerEnd {
			<-owner.done
			if lane.closeErr != nil {
				_, active, clean := owner.framedParent.closeWriteWitness()
				joinedLowerClose = !active && clean
			}
			if creditErr != nil && errors.Is(creditErr, io.EOF) {
				_, active, clean := owner.framedParent.writeWitness()
				joinedLowerCredit = !active && clean
			}
		}
		if lane.closeErr != nil && (unemitted || joinedLowerClose || errors.Is(lane.closeErr, ErrClosedSourceStopped)) {
			// Existing traffic errors remain at their operation/parent owner;
			// actual physical retirement failures remain cleanup failures.
			if !joinedPeerEnd {
				<-owner.done
			}
			lane.closeErr = owner.retire()
		}
		// Check the retained CREDIT even if its failure ended the parent before
		// CLOSE could be queued. Whole-parent intentional stop remains distinct.
		if creditErr != nil && !joinedLowerCredit && !errors.Is(creditErr, ErrClosedSourceStopped) {
			lane.closeErr = errors.Join(lane.closeErr, errors.Join(errors.New("closed source in-flight CREDIT write failed"), creditErr))
		}
		if lane.closeErr != nil {
			lane.closeErr = errors.Join(ErrClosedSourceCleanup, errors.New("closed source child terminal write failed"), lane.closeErr)
			owner.fail(lane.closeErr)
			<-owner.done
		}
		// Join existing I/O and keep later calls out until the lane has been
		// removed and the retained parent's idle state has been committed.
		lane.writing.Lock()
		defer lane.writing.Unlock()
		lane.reading.Lock()
		defer lane.reading.Unlock()
		owner.mu.Lock()
		// Capture peer refusal atomically with removal, including one received
		// while our terminal write or existing application I/O was finishing.
		var peerErr error
		if owner.retainClosedRead && lane.remoteClosed && lane.failure != io.EOF {
			peerErr = lane.failure
		}
		delete(owner.lanes, lane.id)
		if lane.active {
			owner.idleUntil = time.Now().Add(closedSourceRetention)
		}
		owner.signalLocked()
		owner.mu.Unlock()
		if peerErr != nil {
			lane.closeErr = errors.Join(lane.closeErr, ErrClosedSourceCleanup, peerErr)
			owner.fail(lane.closeErr)
			<-owner.done
		}
	})
	return lane.closeErr
}
