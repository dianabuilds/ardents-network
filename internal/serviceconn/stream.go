package serviceconn

import (
	"context"
	"errors"
	"io"
)

func exchangeExact(application, route io.ReadWriter, count uint32) (uint32, uint32, error) {
	type copyResult struct {
		accepted bool
		count    int64
		err      error
	}
	results := make(chan copyResult, 2)
	copyDirection := func(destination io.Writer, source io.Reader, accepted bool) {
		copied, err := io.CopyN(destination, source, int64(count))
		if accepted {
			if closer, ok := destination.(interface{ CloseWrite() error }); ok {
				_ = closer.CloseWrite()
			}
		}
		results <- copyResult{accepted: accepted, count: copied, err: err}
	}
	go copyDirection(route, application, true)
	go copyDirection(application, route, false)
	first, second := <-results, <-results
	accepted, received := uint32(first.count), uint32(second.count)
	if !first.accepted {
		accepted, received = received, accepted
	}
	if first.count != int64(count) || second.count != int64(count) || first.err != nil || second.err != nil {
		return accepted, received, errors.Join(first.err, second.err,
			errors.New("bounded Service Connection ended before exact byte counts"))
	}
	return accepted, received, nil
}

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

func streamFailure(ctx context.Context, accepted, received uint32, err error) (Result, error) {
	if ctx.Err() != nil {
		result, failure := failed("local timeout or cancellation", "Service Connection was cancelled locally", ctx.Err())
		result.AcceptedBytes, result.ReceivedBytes = accepted, received
		return result, failure
	}
	result, failure := failed("abrupt connection loss", "remote Application completion is unknown", err)
	result.AcceptedBytes, result.ReceivedBytes = accepted, received
	return result, failure
}
