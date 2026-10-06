package issuer

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"net"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

// Exchange carries one prepared batch over an already authenticated and admitted
// role channel under its original deadline. The caller holds finite copy/queue
// capacity, interrupts physical I/O and joins the lower channel before Stock
// completion. Returned bytes are caller-owned and require Stock verification.
func Exchange(conn net.Conn, request []byte, check func() error) (payload []byte, result error) {
	if conn == nil || check == nil {
		return nil, errors.New("route issuer exchange composition absent")
	}
	defer func() { result = errors.Join(result, check()) }()
	if err := check(); err != nil {
		return nil, err
	}
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	body, err := ardp.EncodeIssuerRequest(nonce, request)
	if err != nil {
		return nil, err
	}
	defer clear(body)
	if err := ardp.WriteFrame(conn, ardp.Frame{Kind: ardp.KindOperation, Body: body}); err != nil {
		return nil, err
	}
	if err := check(); err != nil {
		return nil, err
	}
	frame, err := ardp.ReadFrame(conn)
	defer clear(frame.Body)
	if err != nil {
		return nil, err
	}
	if frame.Kind != ardp.KindResult || frame.Lane != 0 {
		return nil, errors.New("route issuer RESULT required")
	}
	status, borrowed, err := ardp.DecodeIssuerResult(frame.Body, nonce)
	if err != nil {
		return nil, err
	}
	if err := checkResult(status, borrowed); err != nil {
		return nil, err
	}
	if err := check(); err != nil {
		return nil, err
	}
	payload = append([]byte(nil), borrowed...)
	writer, ok := conn.(interface{ CloseWrite() error })
	if !ok {
		return payload, errors.New("route issuer TLS half-close unavailable")
	}
	if err := writer.CloseWrite(); err != nil {
		return payload, err
	}
	var extra [1]byte
	if n, err := conn.Read(extra[:]); n != 0 || !errors.Is(err, io.EOF) {
		return payload, errors.Join(errors.New("route issuer TLS peer termination required"), err)
	}
	return payload, nil
}

// Serve carries exactly one exchange after authenticating and admitting the
// role. check retains genuine original authority; issue invokes issuing
// Admission with the kind already derived from the channel, not request bytes.
// The caller retains exclusive work, closes/joins the original connection and
// lower lane after return, and only then releases its queues and reservations.
func Serve(ctx context.Context, conn net.Conn, h ardp.Hello, remaining uint64, check func() error, issue func(context.Context, []byte) ([]byte, error)) (result error) {
	const exchangeBytes = 2 * (ardp.HeaderSize + ardp.IssuerBodySize)
	if ctx == nil || conn == nil || issue == nil || h.Purpose != ardp.PurposeIssuer || remaining < exchangeBytes || check == nil {
		return errors.New("route issuer exchange bounds invalid")
	}
	ctx, cancel := context.WithDeadline(ctx, h.Deadline)
	defer cancel()
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(interrupted); _ = conn.SetDeadline(time.Now()) })
	defer func() {
		if !stop() {
			<-interrupted
		}
	}()
	recheck := func() error { return errors.Join(ctx.Err(), check()) }
	// Deadline interruption may turn canceled I/O into a socket timeout. Retain
	// the physical failure and original cause before the enclosing physical join.
	defer func() { result = errors.Join(result, recheck()) }()
	if err := recheck(); err != nil {
		return err
	}
	frame, err := ardp.ReadFrame(conn)
	defer clear(frame.Body)
	if err = errors.Join(err, recheck()); err != nil {
		return err
	}
	if frame.Kind != ardp.KindOperation || frame.Lane != 0 {
		return errors.New("route issuer operation required")
	}
	nonce, padded, err := ardp.DecodeIssuerRequest(frame.Body)
	if err != nil {
		return err
	}
	batch, err := admission.UnpadClosedTokenBatch(padded)
	if err != nil {
		return err
	}
	if err := recheck(); err != nil {
		return err
	}
	payload, err := issue(ctx, batch)
	defer clear(payload)
	if err = errors.Join(err, recheck()); err != nil {
		return err
	}
	outcome, err := admission.DecodeClosedTokenBatchResult(payload)
	if err != nil {
		return err
	}
	for _, signature := range outcome.Signatures {
		clear(signature)
	}
	status, err := resultStatus(outcome.Status)
	if err != nil {
		return err
	}
	body, err := ardp.EncodeIssuerResult(nonce, status, payload)
	if err != nil {
		return err
	}
	defer clear(body)
	if err := recheck(); err != nil {
		return err
	}
	if err := ardp.WriteFrame(conn, ardp.Frame{Kind: ardp.KindResult, Body: body}); err != nil {
		return err
	}
	writer, ok := conn.(interface{ CloseWrite() error })
	if !ok {
		return errors.New("route issuer TLS half-close unavailable")
	}
	if err := writer.CloseWrite(); err != nil {
		return err
	}
	var extra [1]byte
	n, err := conn.Read(extra[:])
	if n != 0 || !errors.Is(err, io.EOF) {
		return errors.Join(errors.New("route issuer peer termination required"), err)
	}
	return recheck()
}

func resultStatus(status admission.ClosedTokenBatchStatus) (uint8, error) {
	switch status {
	case admission.ClosedTokenIssued:
		return 0, nil
	case admission.ClosedTokenExhausted:
		return 2, nil
	case admission.ClosedTokenWithdrawn:
		return 4, nil
	case admission.ClosedTokenUnavailable:
		return 1, nil
	default:
		return 0, errors.New("route issuer outcome invalid")
	}
}

func checkResult(status uint8, payload []byte) error {
	outcome, err := admission.DecodeClosedTokenBatchResult(payload)
	if err != nil {
		return err
	}
	for _, signature := range outcome.Signatures {
		clear(signature)
	}
	want, err := resultStatus(outcome.Status)
	if err != nil || status != want {
		return errors.Join(errors.New("route issuer envelope contradicts Admission outcome"), err)
	}
	return nil
}
