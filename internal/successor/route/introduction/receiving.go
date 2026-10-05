package introduction

import (
	"context"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
	"io"
	"net"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

// RegistrationChannel supplies an already authenticated and admitted channel.
// Receiving composition retains its Grant and physical reservation until this
// operation and the connection join. These original bounds cannot authorize a
// channel by themselves; the same exact role authority is reobserved at effects.
type RegistrationChannel struct {
	Hello     ardp.Hello
	Authority role.Authority
	Bytes     uint64
	Deadline  time.Time
	Record    func(error)
}

// ServeRegistration owns REGISTER/RESULT/owning WITHDRAW on one original
// admitted stream. It does not close the stream or return Admission/Hosting
// resources: receiving composition joins those physical effects afterward.
func (registry *Registry) ServeRegistration(ctx context.Context, conn net.Conn, capacity *Capacity, channel RegistrationChannel) error {
	hello := channel.Hello
	if registry == nil || ctx == nil || conn == nil || capacity == nil || channel.Record == nil || hello.Purpose != ardp.PurposeIntroduction {
		return errors.New("introduction registry unavailable")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	binding, err := role.Binding(conn, hello)
	if err != nil {
		return err
	}
	check := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		_, err := channel.Authority.Hello(hello, false)
		return err
	}
	writeResult := func(nonce [32]byte, status uint8) error {
		body, err := EncodeResult(nonce, status)
		if err != nil {
			return err
		}
		if err := check(); err != nil {
			return err
		}
		if err := ardp.WriteFrame(conn, ardp.Frame{Kind: ardp.KindResult, Body: body}); err != nil {
			channel.Record(err)
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
	request, err := DecodeRequest(f.Body)
	if err != nil || request.Withdraw {
		return errors.Join(errors.New("introduction REGISTER required"), err)
	}
	if err := check(); err != nil {
		return err
	}
	registration, err := registry.Register(capacity, request, binding, channel.Bytes, channel.Deadline, role.AdmissionWireBytes, time.Now())
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
	withdraw, err := DecodeRequest(f.Body)
	if err != nil {
		return err
	}
	if err := check(); err != nil {
		return err
	}
	if err := registration.Withdraw(withdraw, binding, time.Now()); err != nil {
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
