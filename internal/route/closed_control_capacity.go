package route

import "github.com/dianabuilds/ardents-network/internal/route/ardp"

func closedControlPurpose(purpose ardp.Purpose) bool {
	switch purpose {
	case ardp.PurposeIssuer, ardp.PurposeReachability, ardp.PurposeIntroduction, ardp.PurposeSubmission:
		return true
	}
	return false
}
