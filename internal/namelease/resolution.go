package namelease

// CanResolve returns whether a Record and its immediate-parent-to-root lineage
// permit successful resolution at now.
func CanResolve(current Record, now int64, parents []Record) (bool, string) {
	if err := validateRecord(current); err != nil {
		return false, "name record is invalid"
	}
	if ok, reason := liveLease(current, now); !ok {
		return false, reason
	}
	if current.Target == "" {
		return false, "name has no current Service Target binding"
	}
	parent, err := validateParents(current.Name, parents, now)
	if err != nil || !sameParent(&current, parent) {
		return false, "parent lineage is missing or stale"
	}
	return true, leaseWarning(current)
}

func liveLease(current Record, now int64) (bool, string) {
	if !validStates(current) {
		return false, "invalid naming state"
	}
	if current.Consistency != consistencyCurrent {
		return false, "name consistency is not current"
	}
	if current.Recovery != recoveryStable {
		return false, "recovery is pending"
	}
	switch current.Lease {
	case leaseActive:
		if now <= current.LeaseExpiresAt {
			return true, ""
		}
		return false, "lease has expired"
	case leaseGrace:
		if now <= current.GraceExpiresAt {
			return true, ""
		}
		return false, "grace period has expired"
	case leaseReleased:
		return false, "name is released"
	default:
		return false, "invalid Lease state"
	}
}

func leaseWarning(current Record) string {
	if current.Lease == leaseGrace {
		return "name is in grace and should be treated as volatile"
	}
	return ""
}
