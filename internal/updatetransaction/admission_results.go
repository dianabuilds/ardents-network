package updatetransaction

// resourceDeniedResult reports the bounded resource refusal before Update
// creates or mutates transaction state.
func resourceDeniedResult(request Request) Result {
	return Result{Outcome: "resource-denied", State: "release-accepted", Generation: request.Generation,
		StagingPresent: false, SafeNotice: "update resources unavailable"}
}

// activationUnsupportedResult reports that the admitted Update root cannot
// support the selected atomic-activation operation.
func activationUnsupportedResult(request Request) Result {
	return Result{Outcome: "activation-unsupported", State: "release-accepted", Generation: request.Generation,
		StagingPresent: false, SafeNotice: "update storage unsupported"}
}
