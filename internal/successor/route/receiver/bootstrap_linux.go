//go:build linux

package receiver

import (
	"context"
	"errors"
	"net"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/successor/route/bootstrap"
	framing "github.com/dianabuilds/ardents-network/internal/successor/route/channel"
	issuertransport "github.com/dianabuilds/ardents-network/internal/successor/route/issuer"
)

// serveBootstrap owns one finite incoming bootstrap lifetime, independently of
// receiving Admission. The concrete Receiver composition has native root owners;
// its channel, queue and output mechanisms remain the portable implementations.
func (r *Receiver) serveBootstrap(ctx context.Context, conn net.Conn, outer *framing.Lane, h ardp.Hello, claim *bootstrap.Claim) (result error) {
	m, err := r.config.Authority.Hello(h, false)
	issuerDuty := m.RoleDomain == 2 && m.Subrole == 6 && h.Purpose == ardp.PurposeIssuer
	forwardingDuty := h.Purpose == ardp.PurposeForwarding && m.RoleDomain == 1 && (m.Subrole == 1 || m.Subrole == 2)
	if err != nil || (!issuerDuty && !forwardingDuty) {
		return errors.Join(errors.New("bootstrap forwarding duty unavailable"), err)
	}
	ctx, cancel := context.WithDeadline(ctx, h.Deadline)
	defer cancel()
	var adjacency *bootstrap.Adjacency
	var release func() error
	var releaseControl func()
	var s *framing.Session
	defer func() {
		if s != nil {
			result = errors.Join(result, r.finishSession(s))
		} else {
			result = errors.Join(result, conn.Close())
		}
		if releaseControl != nil {
			releaseControl()
		}
		if release != nil {
			releaseErr := release()
			r.record(releaseErr)
			result = errors.Join(result, releaseErr)
		}
		if adjacency != nil {
			adjacency.Seal()
			claim.ReleaseAfterJoin()
		}
	}()
	if outer == nil {
		if m.Subrole != 1 || r.config.ReserveBootstrap == nil {
			return errors.New("bootstrap Entry composition unavailable")
		}
		adjacency = r.bootstrap.Adjacency()
		claim, err = adjacency.Reserve(h.Deadline)
		if err != nil {
			return err
		}
		if h.Deadline.After(claim.Deadline()) {
			return errors.New("bootstrap original horizon exceeds reserved work")
		}
		release, err = r.config.ReserveBootstrap(ctx, claim.Deadline())
		if err != nil || release == nil {
			return errors.Join(errors.New("bootstrap Entry physical capacity unavailable"), err)
		}
	} else if (!issuerDuty && m.Subrole != 2) || claim == nil {
		return errors.New("bootstrap incoming restricted claim absent")
	}
	if h.Deadline.After(claim.Deadline()) {
		return errors.New("bootstrap original horizon exceeds reserved work")
	}
	releaseControl, err = r.bootstrap.Queues().HoldControl()
	if err != nil {
		return err
	}
	check := func() error {
		_, err := r.config.Authority.Hello(h, false)
		_, restrictionErr := claim.Restriction()
		return errors.Join(err, restrictionErr, ctx.Err())
	}
	if err := check(); err != nil {
		return err
	}
	const setupBytes = 3*ardp.HeaderSize + 209 + 1 + 5
	if err := conn.SetDeadline(h.Deadline); err != nil {
		return err
	}
	if outer != nil {
		if err := outer.Admit(h.Deadline); err != nil {
			return err
		}
	} else if err := claim.ChargeOutput(ardp.HeaderSize + 5); err != nil {
		return err
	}
	if err := framing.Accept(conn); err != nil {
		return err
	}
	if err := check(); err != nil {
		return err
	}
	if issuerDuty {
		return issuertransport.Serve(ctx, conn, h, bootstrap.LaneBytes-setupBytes, check, func(ctx context.Context, request []byte) ([]byte, error) { return r.config.Issue(ctx, request, true) })
	}
	handlers := framing.Handlers{Open: func(ctx context.Context, l *framing.Lane, body []byte) error {
		restriction, err := claim.Restriction()
		if err != nil {
			return err
		}
		return r.forward(ctx, l, h, body, restriction)
	}}
	if outer == nil {
		// Incoming Node children already charge ciphertext and complete outer
		// frames at their original claim. Entry charges its direct framing once.
		handlers.Output = claim.ChargeOutput
	}
	s = framing.Prepare(ctx, conn, h.Deadline, bootstrap.LaneBytes-setupBytes, check, false, r.bootstrap.Queues(), handlers)
	s.Start()
	<-s.Done()
	return nil
}
