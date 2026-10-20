package source

import (
	"context"
	"net"
)

// sourceConnectionCancellation retains the exact original socket until its
// cancellation callback has physically returned. Each caller defers the one
// returned retirement before releasing its connection/key/user ownership.
func sourceConnectionCancellation(ctx context.Context, connection net.Conn) func() {
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		_ = connection.Close()
		close(interrupted)
	})
	return func() {
		if !stop() {
			<-interrupted
		}
	}
}
