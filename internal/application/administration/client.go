package administration

import (
	"context"
	"errors"
	"io"
	"net"
	"time"
)

// Request invokes one closed operation through the local client Adapter.
func Request(ctx context.Context, path string, operation Operation) (Outcome, error) {
	if ctx == nil || path == "" || (operation != Publish && operation != Withdraw) {
		return "", errors.New("local Service Administration request is invalid")
	}
	return request(ctx, path, func(output io.Writer) error {
		_, err := io.WriteString(output, string(operation)+"\n")
		return err
	}, map[Operation]Outcome{Publish: Published, Withdraw: Withdrawn}[operation])
}

func request(ctx context.Context, path string, write func(io.Writer) error, expected Outcome) (Outcome, error) {
	response, err := requestBytes(ctx, path, write, 64)
	if err != nil {
		return "", err
	}
	outcome := map[string]Outcome{"published\n": Published, "withdrawn\n": Withdrawn}[string(response)]
	if outcome != expected {
		return "", errors.New("local Service Administration request failed")
	}
	return outcome, nil
}

func requestBytes(ctx context.Context, path string, write func(io.Writer) error, maximum int64) ([]byte, error) {
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
		return nil, requestError(ctx, err)
	}
	if err := connection.CloseWrite(); err != nil {
		return nil, requestError(ctx, err)
	}
	response, err := io.ReadAll(io.LimitReader(connection, maximum))
	if err != nil {
		return nil, requestError(ctx, err)
	}
	if !stopCancellation() {
		<-cancelled
		return nil, requestError(ctx, nil)
	}
	callbackStopped = true
	if err := requestError(ctx, nil); err != nil {
		return nil, err
	}
	return response, nil
}

func requestError(ctx context.Context, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	if deadline, available := ctx.Deadline(); available && !time.Now().Before(deadline) {
		return context.DeadlineExceeded
	}
	return err
}
