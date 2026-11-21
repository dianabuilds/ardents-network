package hosting

// JointTraffic bounds both component directions and their combined traffic.
// It describes a workload envelope, not measured counters or billing authority.
type JointTraffic struct {
	Tx, Rx, Total uint64
}

func (traffic JointTraffic) cost(policy Policy) (uint64, error) {
	if traffic.Total == 0 || traffic.Tx > traffic.Total || traffic.Rx > traffic.Total ||
		traffic.Tx < traffic.Total-traffic.Rx {
		return 0, ErrInvalid
	}
	switch policy.Directions {
	case "tx":
		return traffic.Tx, nil
	case "rx":
		return traffic.Rx, nil
	case "tx+rx":
		return traffic.Total, nil
	default:
		return 0, ErrInvalid
	}
}
