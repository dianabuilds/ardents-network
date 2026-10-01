package route

import "github.com/dianabuilds/ardents-network/internal/route/ardp"

// ClosedControlPurpose reports whether one ARDP purpose draws from the
// finite control reserve. Outgoing child-capacity accounting shares this
// single definition with the receiving forwarding channel.
func ClosedControlPurpose(purpose ardp.Purpose) bool {
	switch purpose {
	case ardp.PurposeIssuer, ardp.PurposeReachability, ardp.PurposeIntroduction, ardp.PurposeSubmission:
		return true
	}
	return false
}
