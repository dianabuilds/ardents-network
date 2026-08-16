package main

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"os"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/bridge"
	"github.com/dianabuilds/ardents-network/internal/camouflage"
)

type entryRuntime struct {
	bridge                   bridgeOwner
	closeNetwork, closeRoles func() error
	client                   camouflage.Client
	transition               *os.File
	manifest                 [32]byte
	mu                       sync.Mutex
	used                     bool
}

type bridgeOwner interface {
	Acquire(context.Context, []byte, [32]byte, time.Time,
		func(context.Context, [32]byte, []byte, time.Time) (net.Conn, func() error, bool, error),
	) (net.Conn, func() error, error)
	Evidence() (bridge.AttemptEvidence, error)
	Close() error
}

func (owner *entryRuntime) open(ctx context.Context, authenticate func(context.Context, net.Conn) (*tls.Conn, error)) (*tls.Conn, func() error, error) {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.used {
		return nil, nil, errors.New("bridge-attempt-exhausted")
	}
	owner.used = true
	frame, err := readInheritedPipe(ctx, owner.transition, 256)
	if err != nil {
		return nil, nil, err
	}
	var parentDeadline time.Time
	if deadline, ok := ctx.Deadline(); ok {
		parentDeadline = deadline
	}
	channel, cleanup, err := owner.bridge.Acquire(ctx, frame, owner.manifest, parentDeadline,
		entryContactOpener{client: owner.client, authenticate: authenticate}.openContact)
	if err != nil {
		return nil, nil, err
	}
	secured, ok := channel.(*tls.Conn)
	if !ok {
		return nil, nil, errors.Join(errors.New("bridge-local-denial"), cleanup())
	}
	return secured, cleanup, nil
}

func (owner *entryRuntime) close() error {
	return errors.Join(owner.transition.Close(), owner.bridge.Close(), owner.closeRoles(), owner.closeNetwork())
}
