package endpoint

import "context"

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
