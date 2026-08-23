package endpoint

import (
	"context"
	"io"
)

func writeAll(writer io.Writer, value []byte) error {
	for len(value) > 0 {
		count, err := writer.Write(value)
		if err != nil {
			return err
		}
		if count == 0 {
			return io.ErrNoProgress
		}
		value = value[count:]
	}
	return nil
}

func streamFailure(ctx context.Context, accepted, received uint32, err error) (RuntimeResult, error) {
	if ctx.Err() != nil {
		result, failure := failed("local timeout or cancellation", "Service Connection was cancelled locally", ctx.Err())
		result.AcceptedBytes, result.ReceivedBytes = accepted, received
		return result, failure
	}
	result, failure := failed("abrupt connection loss", "remote Application completion is unknown", err)
	result.AcceptedBytes, result.ReceivedBytes = accepted, received
	return result, failure
}
