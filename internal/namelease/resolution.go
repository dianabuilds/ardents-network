package namelease

func canResolve(current Record, now int64, parents []Record) (bool, string) {
	_, warning, err := ResolveBinding(current, now, parents)
	if err != nil {
		return false, err.Error()
	}
	return true, warning
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
