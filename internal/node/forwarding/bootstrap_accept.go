package forwarding

import (
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
)

func (server *closedForwardingServer) admitBootstrap(receiver route.ClosedRoleReceiver, incomingKey [32]byte, hello ardp.Hello, helloSize int, frame ardp.Frame) (*route.ClosedForwardingChannel, time.Time, error) {
	if receiver.Subrole != 1 && receiver.Subrole != 2 || receiver.Subrole == 1 && incomingKey != [32]byte{} {
		return nil, time.Time{}, errors.New("closed bootstrap receiving adjacency is unavailable")
	}
	issuer, err := ardp.DecodeBootstrap(frame.Body)
	if err != nil || !issuer || frame.Kind != 3 || frame.Lane != 0 || hello == (ardp.Hello{}) {
		return nil, time.Time{}, errors.New("closed bootstrap issuer operation is required")
	}
	adjacency := incomingKey
	if adjacency == [32]byte{} {
		adjacency[0] = 1
	}
	deadline := server.clock().UTC().Add(10 * time.Second)
	if hello.Deadline.Before(deadline) {
		deadline = hello.Deadline
	}
	lease, err := server.receiving.bootstrap.Admit(adjacency, deadline)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer lease.Release()
	if err := lease.Receive(uint64(helloSize + 16 + len(frame.Body))); err != nil {
		return nil, time.Time{}, err
	}
	channel, err := route.NewClosedBootstrapForwardingChannel(lease, server.receiving.limits, func(open route.ClosedOpen) error {
		current, err := server.dependencies.current()
		if err != nil {
			return err
		}
		return BootstrapRecipient(server.dependencies.authority, current, receiver, incomingKey, open, server.clock().UTC(), server.dependencies.literalEndpoint)
	}, server.clock)
	return channel, deadline, err
}
