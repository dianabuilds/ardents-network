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

func receiveChannel(ctx context.Context, conn net.Conn, a role.Authority, admit Admit, opened *ardp.Open, capacity *admissionRetirement) (receiving.Grant, ardp.Hello, error) {
	h, err := framing.ReadHello(conn)
	if err != nil {
		return receiving.Grant{}, h, err
	}
	if _, err := a.Hello(h, false); err != nil {
		return receiving.Grant{}, h, err
	}
	if opened != nil && (h.RecipientNodeID != opened.RecipientNodeID || h.RecipientDutyGeneration != opened.RecipientDutyGeneration || uint8(h.Purpose) != opened.Purpose || h.Deadline.After(opened.Deadline)) {
		return receiving.Grant{}, h, errors.New("route inner HELLO differs from OPEN")
	}
	hash, err := role.Binding(conn, h)
	if err != nil {
		return receiving.Grant{}, h, err
	}
	binding := Channel{Hello: h, Binding: hash, capacity: capacity}
	f, err := ardp.ReadFrame(conn)
	if err != nil {
		return receiving.Grant{}, h, err
	}
	class, classErr := role.AdmissionClass(h.Purpose)
	if classErr != nil || f.Kind != ardp.KindAdmit || f.Lane != 0 || f.Body[0] != byte(class) {
		return receiving.Grant{}, h, errors.New("route purpose-bound admission required")
	}
	grant, err := admit(ctx, binding, f.Body[1:])
	clear(f.Body)
	if err != nil {
		return receiving.Grant{}, h, err
	}
	// The caller receives the Grant even on post-spend/ACK failure and joins
	// its transport before releasing it. Admission never owns that join.
	if grant.Allowance().Deadline() != h.Deadline {
		return grant, h, errors.New("route admitted horizon differs")
	}
	_, err = a.Hello(h, false)
	return grant, h, err
}

// minDeadline preserves the original role/parent setup bound.
func minDeadline(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
