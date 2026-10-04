//go:build linux

package transport

import (
	"context"
	"errors"
	"io"
	"net"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/successor/route/introduction"
)

func (r *Receiver) serveRegistration(ctx context.Context, conn net.Conn, grant receiving.Grant, hello ardp.Hello, capacity *introduction.Capacity) error {
	if r.registrations == nil {
		return errors.New("introduction registry unavailable")
	}
	channel, err := channelBinding(conn, hello)
	if err != nil {
		return err
	}
	check := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		_, err := r.config.Authority.hello(hello, false)
		return err
	}
	writeResult := func(nonce [32]byte, status uint8) error {
		body, err := introduction.EncodeResult(nonce, status)
		if err != nil {
			return err
		}
		if err := check(); err != nil {
			return err
		}
		if err := ardp.WriteFrame(conn, ardp.Frame{Kind: ardp.KindResult, Body: body}); err != nil {
			r.record(err)
			return err
		}
		return check()
	}
	f, err := ardp.ReadFrame(conn)
	if err != nil {
		return err
	}
	if f.Kind != ardp.KindOperation || f.Lane != 0 {
		return errors.New("introduction REGISTER lane required")
	}
	request, err := introduction.DecodeRequest(f.Body)
	if err != nil || request.Withdraw {
		return errors.Join(errors.New("introduction REGISTER required"), err)
	}
	if err := check(); err != nil {
		return err
	}
	registration, err := r.registrations.Register(capacity, request, channel.Binding, grant.Allowance().Bytes(), grant.Allowance().Deadline(), admissionWireBytes, time.Now())
	if err != nil {
		return errors.Join(err, writeResult(request.Nonce, 1))
	}
	defer registration.Retire()
	if err := conn.SetDeadline(request.Expiry); err != nil {
		return err
	}
	// The claim is durable even if duty loss or a partial RESULT now refuses.
	if err := writeResult(request.Nonce, 0); err != nil {
		return err
	}
	if err := registration.Acknowledge(time.Now()); err != nil {
		return err
	}
	f, err = ardp.ReadFrame(conn)
	if err != nil {
		return err
	}
	if f.Kind != ardp.KindOperation || f.Lane != 0 {
		return errors.New("introduction WITHDRAW lane required")
	}
	withdraw, err := introduction.DecodeRequest(f.Body)
	if err != nil {
		return err
	}
	if err := check(); err != nil {
		return err
	}
	if err := registration.Withdraw(withdraw, channel.Binding, time.Now()); err != nil {
		return errors.Join(err, writeResult(withdraw.Nonce, 1))
	}
	if err := writeResult(withdraw.Nonce, 0); err != nil {
		return err
	}
	// Completing a write into a nested lane does not mean the holder consumed
	// the RESULT. Keep the original channel and parents alive until its peer
	// closes after that result, or the original expiry/cancellation interrupts
	// them. Closing immediately can cancel a forwarder with ACK bytes buffered.
	var trailing [1]byte
	n, err := conn.Read(trailing[:])
	if n == 0 && errors.Is(err, io.EOF) {
		return nil
	}
	return errors.Join(errors.New("introduction withdrawn channel did not close"), err)
}
