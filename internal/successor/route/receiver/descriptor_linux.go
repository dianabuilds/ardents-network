//go:build linux

package receiver

import (
	"context"
	"errors"
	"io"
	"net"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/reachability"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

// serveDescriptor owns one admitted purpose-3 operation and its exact role
// termination. Verification, floor transitions and durable publication belong
// to the genuine Store; no supplied storage-success/ACK callback is accepted.
func (r *Receiver) serveDescriptor(ctx context.Context, conn net.Conn, hello ardp.Hello, remaining uint64, check func() error) (result error) {
	const maximumExchange = 2 * (ardp.HeaderSize + ardp.DescriptorResultBodySize)
	if ctx == nil || conn == nil || check == nil || hello.Purpose != ardp.PurposeReachability || r.config.DescriptorStore == nil || remaining < maximumExchange {
		return errors.New("route Descriptor admission or allowance unavailable")
	}
	// serveRole holds the finite copy reservation before irreversible Admission
	// and returns it only after this exact connection has physically joined.
	ctx, cancel := context.WithDeadline(ctx, hello.Deadline)
	defer cancel()
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(interrupted); _ = conn.SetDeadline(time.Now()) })
	defer func() {
		if !stop() {
			<-interrupted
		}
	}()
	recheck := func() error { return errors.Join(ctx.Err(), check()) }
	effectGuard := recheck
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
		return errors.New("route Descriptor lane-zero OPERATION required")
	}
	request, err := ardp.DecodeDescriptorRequest(frame.Body)
	if err != nil {
		return err
	}
	status := uint8(1)
	var payload []byte
	if request.Operation == ardp.DescriptorPublish {
		proof, verifyErr := reachability.VerifyPublish(request.Proof, hello.NetworkID, hello.ProfileDigest, time.Now())
		if verifyErr == nil {
			guard := func() error { return errors.Join(recheck(), r.descriptorIntroduction(hello, proof)) }
			effectGuard = guard
			if err := guard(); err != nil {
				return err
			}
			outcome, storeErr := r.config.DescriptorStore.Publish(request.Proof, hello.ProfileDigest, time.Now(), guard)
			status = descriptorStatus(outcome, storeErr)
			if err := guard(); err != nil {
				return err
			}
		}
	} else {
		var outcome reachability.Outcome
		payload, outcome, err = r.config.DescriptorStore.Lookup(request.Target, hello.ProfileDigest, time.Now())
		status = descriptorStatus(outcome, err)
		if status == 0 {
			proof, verifyErr := reachability.Verify(payload, request.Target, hello.NetworkID, hello.ProfileDigest, time.Now())
			if verifyErr != nil {
				status = 1
			} else {
				effectGuard = func() error { return errors.Join(recheck(), r.descriptorIntroduction(hello, proof)) }
				if err := effectGuard(); err != nil {
					return err
				}
			}
		}
	}
	defer clear(payload)
	if status != 0 {
		clear(payload)
		payload = nil
	}
	body, err := ardp.EncodeDescriptorResult(request.Operation, request.Nonce, status, payload)
	if err != nil {
		return err
	}
	defer clear(body)
	if err := effectGuard(); err != nil {
		return err
	}
	if err := ardp.WriteFrame(conn, ardp.Frame{Kind: ardp.KindResult, Body: body}); err != nil {
		return err
	}
	if err := effectGuard(); err != nil {
		return err
	}
	writer, ok := conn.(interface{ CloseWrite() error })
	if !ok {
		return errors.New("route Descriptor TLS half-close unavailable")
	}
	if err := writer.CloseWrite(); err != nil {
		return err
	}
	var extra [1]byte
	if n, err := conn.Read(extra[:]); n != 0 || !errors.Is(err, io.EOF) {
		return errors.Join(errors.New("route Descriptor peer termination required"), err)
	}
	return effectGuard()
}

func descriptorStatus(outcome reachability.Outcome, err error) uint8 {
	if err == nil && (outcome == reachability.Accepted || outcome == reachability.AlreadyCurrent) {
		return 0
	}
	if outcome == reachability.Stale || outcome == reachability.Conflicting {
		return 3
	}
	return 1
}

func (r *Receiver) descriptorIntroduction(hello ardp.Hello, proof reachability.Descriptor) error {
	now := time.Now()
	local, err := r.config.Authority.Hello(hello, false)
	if err != nil {
		return err
	}
	view, err := r.config.Authority.Current()
	if err != nil {
		return err
	}
	if view.Profile().ProfileBinding != r.config.Authority.Profile || proof.ProfileDigest != hello.ProfileDigest || now.Before(proof.Introduction.NotBefore) || !now.Before(proof.Introduction.NotAfter) || proof.Introduction.NotAfter.After(local.NotAfter()) || proof.Introduction.NotAfter.After(view.Profile().NotAfter) {
		return errors.New("route Descriptor current profile or Introduction interval unavailable")
	}
	member, err := view.Member(proof.Introduction.Node, now)
	if err != nil || member.RoleDomain != 4 || member.Subrole != 3 || proof.Introduction.NotAfter.After(member.NotAfter()) {
		return errors.Join(errors.New("route Descriptor Introduction assignment unavailable"), err)
	}
	_, err = r.config.Authority.Hello(hello, false)
	if err != nil {
		return err
	}
	return proof.Current(time.Now())
}
