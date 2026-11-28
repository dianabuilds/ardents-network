package administration

import (
	"context"
	"errors"
	"io"
)

// Request invokes one closed operation through the local client Adapter.
func Request(ctx context.Context, path string, operation Operation) (Outcome, error) {
	if ctx == nil || path == "" || (operation != Publish && operation != Withdraw) {
		return "", errors.New("local Service Administration request is invalid")
	}
	return requestOperation(ctx, path, func(output io.ReadWriter) error {
		_, err := io.WriteString(output, string(operation)+"\n")
		return err
	}, map[Operation]Outcome{Publish: Published, Withdraw: Withdrawn}[operation])
}

func requestOperation(ctx context.Context, path string, write func(io.ReadWriter) error, expected Outcome) (Outcome, error) {
	response, err := exchangeLocalRequest(ctx, path, write, 64)
	if err != nil {
		return "", err
	}
	outcome := map[string]Outcome{"published\n": Published, "withdrawn\n": Withdrawn}[string(response)]
	if outcome != expected {
		return "", errors.New("local Service Administration request failed")
	}
	return outcome, nil
}
