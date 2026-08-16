package main

import (
	"context"
	"errors"
	"net"
	"os"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/camouflage"
)

type entryRuntime struct {
	bridge                   bridgeOwner
	closeNetwork, closeRoles func() error
	client                   camouflage.Client
	transition               *os.File
	deadline                 time.Time
	manifest                 [32]byte
	mu                       sync.Mutex
	used                     bool
}

type bridgeOwner interface {
	BeginContact([]byte, [32]byte, time.Time) ([32]byte, []byte, byte, error)
	FinishContact(byte, bool, bool) error
	Close() error
}

func (owner *entryRuntime) open(ctx context.Context) (net.Conn, func() error, error) {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.used {
		return nil, nil, errors.New("bridge entry contact already consumed")
	}
	owner.used = true
	frame, err := readInheritedPipe(ctx, owner.transition, 256)
	if err != nil {
		return nil, nil, err
	}
	identity, envelope, ordinal, err := owner.bridge.BeginContact(frame, owner.manifest, owner.deadline)
	if err != nil {
		return nil, nil, err
	}
	config, err := camouflage.Validate(envelope, identity)
	if err != nil {
		return nil, nil, errors.Join(err, owner.bridge.FinishContact(ordinal, false, true))
	}
	client := owner.client
	if deadline, ok := ctx.Deadline(); ok && deadline.Before(client.Deadline) {
		client.Deadline = deadline
	}
	carrier, cleanup, err := camouflage.OpenClient(ctx, config, client)
	if err != nil {
		return nil, nil, errors.Join(err,
			owner.bridge.FinishContact(ordinal, false, camouflage.CleanupComplete(err)))
	}
	wrapped := func() error {
		cleanupErr := cleanup()
		return errors.Join(cleanupErr, owner.bridge.FinishContact(ordinal, true, cleanupErr == nil))
	}
	return carrier, wrapped, nil
}

func (owner *entryRuntime) close() error {
	return errors.Join(owner.transition.Close(), owner.bridge.Close(), owner.closeRoles(), owner.closeNetwork())
}
