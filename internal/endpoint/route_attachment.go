package endpoint

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	serviceconnection "github.com/dianabuilds/ardents-network/internal/service/connection"
)

func routeAttachmentOpener(listener *net.UnixListener,
	resources func(string, int) uint32) func(context.Context, serviceconnection.Recovery) (net.Conn, error) {
	return func(ctx context.Context, request serviceconnection.Recovery) (net.Conn, error) {
		if request.Generation == 0 || request.Deadline.IsZero() {
			return nil, errors.New("route Attachment request is incomplete")
		}
		deadline := request.Deadline
		if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
			deadline = contextDeadline
		}
		if err := listener.SetDeadline(deadline); err != nil {
			return nil, err
		}
		cancelled := make(chan struct{})
		stop := context.AfterFunc(ctx, func() {
			_ = listener.SetDeadline(time.Now())
			close(cancelled)
		})
		connection, err := listener.AcceptUnix()
		if !stop() {
			<-cancelled
		}
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, err
		}
		resources("accepted-ipc", 1)
		resources("route-attachment-accept", 1)
		return &observedRouteAttachment{Conn: connection, resources: resources}, nil
	}
}

type observedRouteAttachment struct {
	net.Conn
	resources func(string, int) uint32
	once      sync.Once
}

func (attachment *observedRouteAttachment) Close() error {
	err := attachment.Conn.Close()
	attachment.once.Do(func() { attachment.resources("accepted-ipc", -1) })
	return err
}
