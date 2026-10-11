package introduction

import (
	"context"
	"errors"
	"io"
	"net"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
)

// ServeSubmission consumes one genuine purpose-5 admitted channel and forwards
// only its unchanged sealed capsule. Its class-1 allowance never enlarges the
// selected registration's original byte/time bounds. Receiving composition
// retains the Grant and physically closes this connection after the exchange.
func (registry *Registry) ServeSubmission(ctx context.Context, conn net.Conn, channel RegistrationChannel) (result error) {
	hello := channel.Hello
	const exchange = uint64(2*ardp.HeaderSize + 4096 + 16384)
	if registry == nil || ctx == nil || conn == nil || channel.Record == nil || hello.Purpose != ardp.PurposeSubmission ||
		channel.Bytes < role.AdmissionWireBytes+exchange || channel.Deadline.IsZero() {
		return errors.New("introduction submission unavailable")
	}
	end := channel.Deadline
	if hello.Deadline.Before(end) {
		end = hello.Deadline
	}
	check := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !time.Now().Before(end) {
			return context.DeadlineExceeded
		}
		_, err := channel.Authority.Hello(hello, false)
		if !time.Now().Before(end) {
			err = errors.Join(err, context.DeadlineExceeded)
		}
		return errors.Join(err, ctx.Err())
	}
	if err := check(); err != nil {
		return err
	}
	defer func() { result = errors.Join(result, check()) }()
	if _, err := role.Binding(conn, hello); err != nil {
		return err
	}
	frame, err := ardp.ReadFrame(conn)
	if err != nil {
		return err
	}
	if frame.Kind != ardp.KindOperation || frame.Lane != 0 {
		return errors.New("introduction SUBMIT lane required")
	}
	nonce, envelope, err := decodeDelivery(frame.Body)
	if err != nil {
		return err
	}
	header := envelope.Header()
	if !time.Now().Before(header.Expiry) || header.Expiry.After(time.Now().Add(10*time.Second)) ||
		header.Expiry.After(channel.Deadline) || header.Expiry.After(hello.Deadline) {
		return errors.New("introduction submission expiry unavailable")
	}
	end = header.Expiry
	if err := conn.SetDeadline(header.Expiry); err != nil {
		return err
	}
	if err := check(); err != nil {
		return err
	}
	status, deliveryErr := registry.deliver(ctx, nonce, envelope)
	// A failed child never supplies success to the submitter. Its original
	// registration retains any protocol/physical failure independently.
	if deliveryErr != nil {
		status = 1
	}
	body, err := EncodeResult(nonce, status)
	if err != nil {
		return errors.Join(deliveryErr, err)
	}
	if err := check(); err != nil {
		return errors.Join(deliveryErr, err)
	}
	if err := ardp.WriteFrame(conn, ardp.Frame{Kind: ardp.KindResult, Body: body}); err != nil {
		channel.Record(err)
		return errors.Join(deliveryErr, err)
	}
	if err := check(); err != nil {
		return errors.Join(deliveryErr, err)
	}
	var trailing [1]byte
	n, err := conn.Read(trailing[:])
	if n == 0 && errors.Is(err, io.EOF) {
		return deliveryErr
	}
	return errors.Join(deliveryErr, errors.New("introduction submission channel did not close"), err)
}
