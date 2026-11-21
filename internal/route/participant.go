package route

import "time"

// Member is the retained public identity of a selected Route participant.
type Member struct {
	NodeID, PublicKey, FamilyID, RecordDigest [32]byte
	DutyGeneration                            uint64
	Domain                                    uint8
	NotAfter                                  time.Time
}

// Conflict excludes known shared identity, transport key or Operator Family.
func Conflict(first, second Member) bool {
	return first.NodeID == second.NodeID || first.PublicKey == second.PublicKey || first.FamilyID == second.FamilyID
}

// PurposePermitsDuty applies the exact generation-three role-purpose matrix.
// It grants no Network, Admission, Hosting or next-hop authority.
func PurposePermitsDuty(purpose, domain, subrole uint8) bool {
	switch purpose {
	case 1:
		return domain == 2 && subrole == 6
	case 2, 3:
		return domain == 2 && subrole == 5
	case 4, 5:
		return domain == 4 && subrole == 3
	case 6:
		return domain == 2 && subrole == 4
	case 7:
		return (domain == 1 || domain == 3 || domain == 4) && (subrole == 1 || subrole == 2)
	}
	return false
}
