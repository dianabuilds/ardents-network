package route

func closedControlPurpose(purpose ClosedPurpose) bool {
	switch purpose {
	case ClosedPurposeIssuer, ClosedPurposeReachability, ClosedPurposeIntroduction, ClosedPurposeSubmission:
		return true
	}
	return false
}
