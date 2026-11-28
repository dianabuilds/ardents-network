package administration

import (
	"context"
	"errors"
	"io"
	"net"
	"time"
)

func exchangeLocalRequest(ctx context.Context, path string, write func(io.ReadWriter) error, maximum int64) ([]byte, error) {
	if ctx == nil || path == "" || write == nil {
		return nil, errors.New("local Service Administration request is invalid")
	}
	raw, err := (&net.Dialer{}).DialContext(ctx, "unix", path)
	if err != nil {
		return nil, err
	}
	connection, ok := raw.(*net.UnixConn)
	if !ok {
		_ = raw.Close()
		return nil, errors.New("local Service Administration attachment is not a Unix connection")
	}
	defer connection.Close()
	if deadline, available := ctx.Deadline(); available {
		_ = connection.SetDeadline(deadline)
	} else {
		_ = connection.SetDeadline(time.Now().Add(15 * time.Second))
	}
	cancelled := make(chan struct{})
	callbackStopped := false
	stopCancellation := context.AfterFunc(ctx, func() { defer close(cancelled); _ = connection.SetDeadline(time.Now()) })
	defer func() {
		if !callbackStopped && !stopCancellation() {
			<-cancelled
		}
	}()
	if err := write(connection); err != nil {
		return nil, localRequestError(ctx, err)
	}
	if err := connection.CloseWrite(); err != nil {
		return nil, localRequestError(ctx, err)
	}
	response, err := io.ReadAll(io.LimitReader(connection, maximum))
	if err != nil {
		return nil, localRequestError(ctx, err)
	}
	if !stopCancellation() {
		<-cancelled
		return nil, localRequestError(ctx, nil)
	}
	callbackStopped = true
	if err := localRequestError(ctx, nil); err != nil {
		return nil, err
	}
	return response, nil
}

func localRequestError(ctx context.Context, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	if deadline, available := ctx.Deadline(); available && !time.Now().Before(deadline) {
		return context.DeadlineExceeded
	}
	return err
}
