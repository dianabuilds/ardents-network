//go:build linux

package streamqualification

import "errors"

// NetworkCondition fixes the acceptance envelope independently of workload
// direction. It cannot select a Carrier, Route, destination or executable.
type NetworkCondition byte

const (
	NormalNetwork       NetworkCondition = 1
	ImpairedLiveNetwork NetworkCondition = 2
	RecoveryNetwork     NetworkCondition = 3
)

func (condition NetworkCondition) CarrierRatioLimit() (float64, error) {
	switch condition {
	case NormalNetwork:
		return 1.5, nil
	case ImpairedLiveNetwork, RecoveryNetwork:
		return 2, nil
	default:
		return 0, errors.New("qualification network condition is invalid")
	}
}
