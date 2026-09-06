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
	raw, err := (&net.Dialer{}).DialContext(ctx, "unix", path)
	if err != nil {
		return "", err
	}
	connection, ok := raw.(*net.UnixConn)
	if !ok {
		_ = raw.Close()
		return "", errors.New("local Service Administration attachment is not a Unix connection")
	}
	defer connection.Close()
	if deadline, available := ctx.Deadline(); available {
		_ = connection.SetDeadline(deadline)
	} else {
		_ = connection.SetDeadline(time.Now().Add(15 * time.Second))
	}
	stopCancellation := context.AfterFunc(ctx, func() { _ = connection.SetDeadline(time.Now()) })
	defer stopCancellation()
	if _, err := io.WriteString(connection, string(operation)+"\n"); err != nil {
		return "", requestError(ctx, err)
	}
	if err := connection.CloseWrite(); err != nil {
		return "", requestError(ctx, err)
	}
	response, err := io.ReadAll(io.LimitReader(connection, 64))
	if err != nil {
		return "", requestError(ctx, err)
	}
	if !stopCancellation() {
		return "", requestError(ctx, nil)
	}
	outcome := map[string]Outcome{"published\n": Published, "withdrawn\n": Withdrawn}[string(response)]
	if outcome == "" {
		return "", errors.New("local Service Administration request failed")
	}
	return outcome, nil
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
