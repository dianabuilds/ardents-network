package main

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"time"

	"github.com/dianabuilds/ardents-network/internal/camouflage"
)

type entryContactOpener struct {
	client       camouflage.Client
	authenticate func(context.Context, net.Conn) (*tls.Conn, error)
}

func (opener entryContactOpener) openContact(ctx context.Context, identity [32]byte, envelope []byte,
	deadline time.Time,
) (net.Conn, func() error, bool, error) {
	config, err := camouflage.Validate(envelope, identity)
	if err != nil {
		return nil, nil, true, err
	}
	client := opener.client
	client.Deadline = deadline
	carrier, cleanup, cleanupComplete, err := camouflage.OpenClient(ctx, config, client)
	if err != nil {
		return nil, nil, cleanupComplete, err
	}
	secured, authErr := opener.authenticate(ctx, carrier)
	if authErr != nil {
		cleanupErr := cleanup()
		return nil, nil, cleanupErr == nil, errors.Join(authErr, cleanupErr)
	}
	return secured, cleanup, true, nil
}
