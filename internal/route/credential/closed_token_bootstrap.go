package credential

import (
	"context"
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// ServeBootstrap serves one target-free issuer bootstrap exchange on an
// already authenticated successor channel. It permits no private destination,
// Application Data, or lane upgrade; a successful batch remains bootstrap-only.
func (issuer *ClosedTokenIssuer) ServeBootstrap(ctx context.Context, carrier io.ReadWriter, controller *route.ClosedBootstrapController, adjacency [32]byte) error {
	if issuer == nil || ctx == nil || carrier == nil || controller == nil || adjacency == [32]byte{} {
		return errors.New("closed issuer bootstrap is invalid")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	helloFrame, err := route.ReadClosedLaneFrame(carrier)
	if err != nil {
		return err
	}
	hello, err := route.DecodeClosedHello(helloFrame.Body)
	if err != nil || !issuer.acceptsBootstrapHello(hello) {
		return errors.New("closed issuer bootstrap HELLO is unavailable")
	}
	lease, err := controller.Admit(adjacency, hello.Deadline)
	if err != nil {
		return err
	}
	defer lease.Release()
	accepted, err := route.ClosedAcceptFrame(0, 64<<10)
	if err != nil {
		return err
	}
	if err := writeClosedBootstrapFrame(carrier, lease, accepted); err != nil {
		return err
	}
	bootstrap, err := route.ReadClosedLaneFrame(carrier)
	if err != nil || bootstrap.Kind != 3 || bootstrap.Lane != 0 {
		return errors.New("closed issuer bootstrap operation is invalid")
	}
	if err := lease.Receive(uint64(16 + len(bootstrap.Body))); err != nil {
		return err
	}
	issuerOperation, err := route.DecodeClosedBootstrap(bootstrap.Body)
	if err != nil || !issuerOperation {
		return errors.New("closed issuer bootstrap operation is unavailable")
	}
	operation, err := route.ReadClosedLaneFrame(carrier)
	if err != nil || operation.Kind != 10 || operation.Lane != 0 || len(operation.Body) != 16<<10 {
		return errors.New("closed issuer bootstrap terminal operation is invalid")
	}
	if err := lease.Receive(uint64(16 + len(operation.Body))); err != nil {
		return err
	}
	result, err := issuer.IssueTerminalOperation(operation.Body)
	if err != nil {
		return err
	}
	return writeClosedBootstrapFrame(carrier, lease, route.ClosedLaneFrame{Kind: 11, Lane: 0, Body: result})
}

func (issuer *ClosedTokenIssuer) acceptsBootstrapHello(hello route.ClosedHello) bool {
	issuer.mu.Lock()
	defer issuer.mu.Unlock()
	if issuer.closed || !issuer.profileCurrent() {
		return false
	}
	return hello.NetworkID == issuer.network && hello.StateGeneration == issuer.profile.StateGeneration && hello.StateDigest == issuer.profile.StateDigest &&
		hello.ProfileDigest == issuer.profile.Digest && hello.RecipientNodeID == issuer.profile.IssuerNodeID &&
		hello.RecipientDutyGeneration == issuer.profile.IssuerDutyGeneration && hello.Purpose == route.ClosedPurposeIssuer
}

func writeClosedBootstrapFrame(carrier io.Writer, lease *route.ClosedBootstrapLease, frame route.ClosedLaneFrame) error {
	raw, err := route.EncodeClosedLaneFrame(frame)
	if err != nil {
		return err
	}
	if err := lease.Queue(uint64(len(raw))); err != nil {
		return err
	}
	defer func() { _ = lease.Dequeue(uint64(len(raw))) }()
	if err := lease.Send(uint64(len(raw))); err != nil {
		return err
	}
	_, err = carrier.Write(raw)
	return err
}
