package endpoint

import "context"

func streamFailure(ctx context.Context, accepted, received uint32, err error) (runtimeResult, error) {
	if ctx.Err() != nil {
		result, failure := failed("local timeout or cancellation", "Service Connection was cancelled locally", ctx.Err())
		result.AcceptedBytes, result.ReceivedBytes = accepted, received
		return result, failure
	}
	result, failure := failed("abrupt connection loss", "remote Application completion is unknown", err)
	result.AcceptedBytes, result.ReceivedBytes = accepted, received
	return result, failure
}

// preferCallerCancellation preserves authenticated stream evidence while making
// the caller's completed cancellation authoritative over a concurrent carrier
// close observed through a derived Application session.
func preferCallerCancellation(ctx context.Context, result runtimeResult, err error) (runtimeResult, error) {
	if err == nil || ctx == nil || ctx.Err() == nil {
		return result, err
	}
	result.Class = "local timeout or cancellation"
	result.Reason = "Service Connection was cancelled locally"
	return result, ctx.Err()
}
