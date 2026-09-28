//go:build linux

package endpoint

import (
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

// Exclude a public member whose key or known family bridges a different live
// assignment. Apply before Entry/Interior selection and again in admission's
// current selection check; failure never resamples a retained set.
func textRoleSeparated(member textRoleMember, view state.ClosedRouteView, snapshot state.Snapshot, now time.Time) bool {
	for _, role := range view.Nodes[:view.NodeCount] {
		if role.RoleDomain == member.Domain && role.Subrole == member.subrole {
			continue
		}
		for _, candidate := range snapshot.Candidates[:snapshot.CandidateCount] {
			if candidate.NodeID != role.NodeID || candidate.RecordDigest != role.RecordDigest ||
				now.Before(candidate.ValidFrom) || !now.Before(candidate.ValidUntil) || !now.Before(candidate.AssignmentNotAfter) {
				continue
			}
			if candidate.NodeID == member.NodeID || candidate.PublicKey == member.PublicKey || candidate.FamilyID == member.FamilyID {
				return false
			}
		}
	}
	return true
}
