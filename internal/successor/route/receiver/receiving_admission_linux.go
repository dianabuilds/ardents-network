//go:build linux

package receiver

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	framing "github.com/dianabuilds/ardents-network/internal/successor/route/channel"
	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
)

// Channel is observed only after exact peer TLS and HELLO validation. Binding
// remains local to this channel and is never a token challenge or hop label.
type Channel struct {
	Hello    ardp.Hello
	Binding  [32]byte
	capacity *admissionRetirement
}

// Admit is application composition over the actual receiving Admission owner.
// A successful Grant transfers its reservation to this physical work owner.
type Admit func(context.Context, Channel, []byte) (receiving.Grant, error)

// Refill composes genuine receiving Admission with additional Hosting capacity
// for the exact original admitted forwarding parent. remaining excludes ADMIT.
type Refill func(context.Context, Channel, receiving.Grant, uint64, []byte) (receiving.Grant, error)

// readChannelStart authenticates the common channel binding before selecting
// ordinary Admission or narrowly bounded bootstrap. It grants neither mode.
func readChannelStart(ctx context.Context, conn net.Conn, a role.Authority, opened *ardp.Open, capacity *admissionRetirement) (Channel, ardp.Frame, error) {
	h, err := framing.ReadHello(conn)
	if err != nil {
		return Channel{}, ardp.Frame{}, err
	}
	if _, err := a.Hello(h, false); err != nil || ctx.Err() != nil {
		return Channel{}, ardp.Frame{}, errors.Join(err, ctx.Err())
	}
	if opened != nil && (h.RecipientNodeID != opened.RecipientNodeID || h.RecipientDutyGeneration != opened.RecipientDutyGeneration || uint8(h.Purpose) != opened.Purpose || h.Deadline.After(opened.Deadline)) {
		return Channel{}, ardp.Frame{}, errors.New("route inner HELLO differs from OPEN")
	}
	hash, err := role.Binding(conn, h)
	if err != nil {
		return Channel{}, ardp.Frame{}, err
	}
	binding := Channel{Hello: h, Binding: hash, capacity: capacity}
	if err := conn.SetReadDeadline(h.Deadline); err != nil {
		return Channel{}, ardp.Frame{}, err
	}
	f, err := ardp.ReadFrame(conn)
	if err != nil || ctx.Err() != nil {
		clear(f.Body)
		return Channel{}, ardp.Frame{}, errors.Join(err, ctx.Err())
	}
	return binding, f, nil
}

func receiveAdmission(ctx context.Context, a role.Authority, admit Admit, binding Channel, f ardp.Frame) (receiving.Grant, error) {
	h := binding.Hello
	defer clear(f.Body)
	class, classErr := role.AdmissionClass(h.Purpose)
	if classErr != nil || f.Kind != ardp.KindAdmit || f.Lane != 0 || f.Body[0] != byte(class) {
		return receiving.Grant{}, errors.New("route purpose-bound admission required")
	}
	grant, err := admit(ctx, binding, f.Body[1:])
	if err != nil {
		return receiving.Grant{}, err
	}
	// The caller receives the Grant even on post-spend/ACK failure and joins
	// its transport before releasing it. Admission never owns that join.
	if grant.Allowance().Deadline() != h.Deadline {
		return grant, errors.New("route admitted horizon differs")
	}
	_, err = a.Hello(h, false)
	return grant, errors.Join(err, ctx.Err())
}

// minDeadline preserves the original role/parent setup bound.
func minDeadline(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
