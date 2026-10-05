//go:build linux

package transport

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"net"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/successor/route/carrier"
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
type Present func(context.Context, ardp.Hello) ([]byte, error)

const admissionWireBytes = 3*ardp.HeaderSize + 209 + 355 + 5

func channelBinding(conn net.Conn, h ardp.Hello) (Channel, error) {
	if owned, ok := conn.(*retiredConn); ok {
		conn = owned.Conn
	}
	body, err := ardp.EncodeHello(h)
	if err != nil {
		return Channel{}, err
	}
	export, err := carrier.ClosedRoleTLSExporter(conn)
	if err != nil {
		return Channel{}, err
	}
	digest := sha256.Sum256(body)
	raw, err := export("EXPORTER-ardents-channel-v3", digest[:], 32)
	if err != nil {
		return Channel{}, err
	}
	defer clear(raw)
	if len(raw) != 32 {
		return Channel{}, errors.New("route exporter unavailable")
	}
	c := Channel{Hello: h}
	copy(c.Binding[:], raw)
	return c, nil
}

func freshHello(a Authority, end time.Time) (ardp.Hello, error) {
	return freshPurposeHello(a, end, ardp.PurposeForwarding, false)
}

func freshPurposeHello(a Authority, end time.Time, purpose ardp.Purpose, outer bool) (ardp.Hello, error) {
	m, err := a.member()
	if err != nil {
		return ardp.Hello{}, err
	}
	p := a.Profile
	h := ardp.Hello{NetworkID: p.Network, StateGeneration: p.Generation, StateDigest: p.EpochDigest, ProfileDigest: p.Digest,
		RecipientNodeID: m.NodeID, RecipientDutyGeneration: m.DutyGeneration, Purpose: purpose, Deadline: end}
	if _, err := rand.Read(h.ChannelNonce[:]); err != nil {
		return ardp.Hello{}, err
	}
	if _, err := a.hello(h, outer); err != nil {
		return ardp.Hello{}, err
	}
	return h, nil
}

func sendHello(conn net.Conn, h ardp.Hello) error {
	body, err := ardp.EncodeHello(h)
	if err != nil {
		return err
	}
	return ardp.WriteFrame(conn, ardp.Frame{Kind: ardp.KindHello, Body: body})
}
func readHello(conn net.Conn) (ardp.Hello, error) {
	f, err := ardp.ReadFrame(conn)
	if err != nil {
		return ardp.Hello{}, err
	}
	if f.Kind != ardp.KindHello || f.Lane != 0 {
		return ardp.Hello{}, errors.New("route HELLO required")
	}
	return ardp.DecodeHello(f.Body)
}
func acceptChannel(conn net.Conn) error {
	f, err := ardp.AcceptFrame(0, window)
	if err != nil {
		return err
	}
	return ardp.WriteFrame(conn, f)
}
func acceptedChannel(conn net.Conn) error {
	f, err := ardp.ReadFrame(conn)
	if err != nil {
		return err
	}
	status, credit, err := ardp.DecodeAcceptFrame(f)
	if err != nil || status != 0 || credit != window {
		return errors.Join(errors.New("route admission refused"), err)
	}
	return nil
}

func presentChannel(ctx, caller context.Context, conn net.Conn, a Authority, h ardp.Hello, present Present) error {
	check := func() error {
		if err := errors.Join(ctx.Err(), caller.Err()); err != nil {
			return err
		}
		_, err := a.hello(h, false)
		// Authority observation may itself perform I/O. Neither cancellation
		// propagation nor an unscheduled callback authorizes a later effect.
		return errors.Join(err, ctx.Err(), caller.Err())
	}
	if _, err := channelBinding(conn, h); err != nil {
		return err
	}
	if err := check(); err != nil {
		return err
	}
	if err := sendHello(conn, h); err != nil {
		return err
	}
	if err := check(); err != nil {
		return err
	}
	// Stock marks presentation durably before returning token bytes. Recheck
	// after that I/O and before emitting them; refusal never restores stock.
	raw, err := present(ctx, h)
	if err != nil {
		return err
	}
	defer clear(raw)
	if len(raw) != 354 {
		return errors.New("route token length invalid")
	}
	class, err := channelClass(h.Purpose)
	if err != nil {
		return err
	}
	body := append([]byte{byte(class)}, raw...)
	defer clear(body)
	if err := check(); err != nil {
		return err
	}
	if err := ardp.WriteFrame(conn, ardp.Frame{Kind: ardp.KindAdmit, Body: body}); err != nil {
		return err
	}
	if err := acceptedChannel(conn); err != nil {
		return err
	}
	return check()
}

func receiveChannel(ctx context.Context, conn net.Conn, a Authority, admit Admit, opened *ardpHello, capacity *admissionRetirement) (receiving.Grant, ardp.Hello, error) {
	h, err := readHello(conn)
	if err != nil {
		return receiving.Grant{}, h, err
	}
	if _, err := a.hello(h, false); err != nil {
		return receiving.Grant{}, h, err
	}
	if opened != nil && (h.RecipientNodeID != opened.RecipientNodeID || h.RecipientDutyGeneration != opened.RecipientDutyGeneration || uint8(h.Purpose) != opened.Purpose || h.Deadline.After(opened.Deadline)) {
		return receiving.Grant{}, h, errors.New("route inner HELLO differs from OPEN")
	}
	binding, err := channelBinding(conn, h)
	if err != nil {
		return receiving.Grant{}, h, err
	}
	binding.capacity = capacity
	f, err := ardp.ReadFrame(conn)
	if err != nil {
		return receiving.Grant{}, h, err
	}
	class, classErr := channelClass(h.Purpose)
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
	_, err = a.hello(h, false)
	return grant, h, err
}

func channelClass(purpose ardp.Purpose) (admission.Class, error) {
	switch purpose {
	case ardp.PurposeForwarding, ardp.PurposeDataJoin:
		return admission.ForwardClass, nil
	case ardp.PurposeIntroduction:
		return admission.RegistrationClass, nil
	default:
		return 0, errors.New("route terminal purpose unavailable")
	}
}
