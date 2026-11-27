//go:build linux

package administration

import (
	"context"
	"net"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/endpointstream"
	endpointserver "github.com/dianabuilds/ardents-network/internal/application/endpointstream/server"
)

// ByteStreamPublisher admits an operator's local attachment before switching
// the Administration socket to the bounded Application byte framing.
type ByteStreamPublisher interface {
	OpenByteStream(context.Context) (endpointstream.Stream, error)
}

func (server *server) handleByteStream(local *net.UnixConn, raw []byte, started time.Time) bool {
	if string(raw) != endpointstream.PublishByteStreamRequest {
		return false
	}
	publisher, ok := server.owner.(ByteStreamPublisher)
	if !ok {
		_ = endpointserver.WriteSetupRefusal(local, endpointstream.Refuse(endpointstream.Outcome{Class: endpointstream.ServiceUnavailable}))
		return true
	}
	const lifetime = 10 * time.Minute
	deadline := started.Add(lifetime)
	ctx, cancel := context.WithDeadline(server.ctx, deadline)
	defer cancel()
	if err := local.SetDeadline(deadline); err != nil {
		return true
	}
	stream, err := publisher.OpenByteStream(ctx)
	if err != nil || stream == nil {
		_ = endpointserver.WriteSetupRefusal(local, err)
		return true
	}
	_ = endpointserver.ServeStream(ctx, local, stream, true)
	return true
}
