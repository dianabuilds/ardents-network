package route

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"syscall"
	"time"
)

const maximumOpaqueBytes = 256 << 20

func serveNode(ctx context.Context, input Actor, ready func(Evidence)) (Evidence, error) {
	observation := Evidence{Schema: observationSchema, Kind: "complete", Role: input.Role,
		PID: os.Getpid(), ManifestDigest: input.ManifestDigest, NetworkID: input.NetworkID, EpochDigest: input.EpochDigest,
		NodeID: input.NodeID, PreviousPin: input.UpstreamPin, NextNodeID: input.NextNodeID}
	if err := validateNode(input); err != nil {
		return observation, err
	}
	if input.MaximumAttachments > 1 {
		return serveNodeCapacity(ctx, input, ready, observation)
	}
	listener, err := net.Listen("tcp", input.ListenAddress)
	if err != nil {
		return observation, fmt.Errorf("listen for %s: %w", input.Role, err)
	}
	defer listener.Close()
	if err := bindListenerLifetime(ctx, listener.(*net.TCPListener), input.Role); err != nil {
		return observation, err
	}
	stop := context.AfterFunc(ctx, func() { _ = listener.Close() })
	defer stop()
	if ready != nil {
		ready(Evidence{Schema: observationSchema, Kind: "ready", Role: input.Role, PID: os.Getpid(),
			NetworkID: input.NetworkID, EpochDigest: input.EpochDigest, NodeID: input.NodeID,
			PreviousPin: input.UpstreamPin, NextNodeID: input.NextNodeID,
			DeadlineMillis: uint32(input.Deadline / time.Millisecond),
			LifetimeMillis: uint32(input.Lifetime / time.Millisecond)})
	}
	upstream, err := listener.Accept()
	if err != nil {
		return observation, contextError(ctx, err)
	}
	stop()
	if err := listener.Close(); err != nil {
		upstream.Close()
		return observation, fmt.Errorf("close %s bounded Attachment listener: %w", input.Role, err)
	}
	result, err := carryNodeConnection(ctx, input, upstream, observation, func() bool { return true })
	if err == nil {
		result.AttachmentsCompleted = 1
	}
	return result, err
}

func validateNode(input Actor) error {
	if !emptyPlan(input.Plan) || input.PublisherPin != [32]byte{} || input.Stream != nil ||
		!emptyCertificate(input.ClientCertificate) || !emptyCertificate(input.ServiceCertificate) || input.RawAttachment || input.OpenEntry != nil ||
		input.IntroductionSetupPublic != [32]byte{} || input.IntroductionServicePublic != [32]byte{} ||
		input.IntroductionSetupNode != [32]byte{} {
		return errors.New("node received information outside its role-local duty")
	}
	roleOK := false
	for _, role := range routeRoles {
		roleOK = roleOK || input.Role == role
	}
	if !roleOK || input.MaximumAttachments > maximumRoleAttachments || input.NetworkID == [32]byte{} ||
		input.EpochDigest == [32]byte{} || input.NodeID == [32]byte{} ||
		input.UpstreamPin == [32]byte{} || input.NextNodeID == [32]byte{} || input.NextPin == [32]byte{} {
		return errors.New("role-local carrier duty is invalid")
	}
	if err := validateCapacity(input); err != nil {
		return err
	}
	if input.Role == "introduction" {
		present := input.IntroductionSetupSocket != ""
		if present != (input.IntroductionSetupPeer != [32]byte{}) ||
			present != (input.IntroductionForwardSocket != "") || present != (input.IntroductionForwardPublic != [32]byte{}) {
			return errors.New("sealed Introduction setup duty is incomplete")
		}
	} else if input.IntroductionSetupSocket != "" || input.IntroductionSetupPeer != [32]byte{} ||
		input.IntroductionForwardSocket != "" || input.IntroductionForwardPublic != [32]byte{} {
		return errors.New("sealed Introduction setup reached a different Node role")
	}
	if err := validateEndpoint(input.ListenAddress); err != nil {
		return err
	}
	if err := validateEndpoint(input.NextAddress); err != nil {
		return err
	}
	if err := validateCertificate(input.Certificate); err != nil {
		return err
	}
	return validateDeadline(input.Deadline)
}

type opaqueDirection struct {
	count  uint64
	digest [32]byte
}

func relayOpaque(upstream, downstream io.ReadWriteCloser) (opaqueDirection, opaqueDirection, error) {
	type copyResult struct {
		direction opaqueDirection
		forward   bool
		err       error
	}
	results := make(chan copyResult, 2)
	copyDirection := func(destination, source io.ReadWriteCloser, forward bool) {
		hash := sha256.New()
		writer := io.MultiWriter(destination, hash)
		count, err := io.Copy(writer, io.LimitReader(source, maximumOpaqueBytes))
		if closer, ok := destination.(interface{ CloseWrite() error }); ok {
			_ = closer.CloseWrite()
		}
		var digest [32]byte
		copy(digest[:], hash.Sum(nil))
		results <- copyResult{direction: opaqueDirection{count: uint64(count), digest: digest}, forward: forward, err: err}
	}
	go copyDirection(downstream, upstream, true)
	go copyDirection(upstream, downstream, false)
	first, second := <-results, <-results
	forward, reverse := first.direction, second.direction
	if !first.forward {
		forward, reverse = second.direction, first.direction
	}
	if first.err != nil || second.err != nil {
		return forward, reverse, errors.Join(first.err, second.err)
	}
	return forward, reverse, nil
}

func benignStreamError(err error) bool {
	return err == nil || errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) || errors.Is(err, io.ErrClosedPipe) ||
		errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE) || platformBenignStreamError(err)
}

func contextError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}
