//go:build live && !linux

package network_test

import "errors"

func finalCampaignHostAllocation([]finalRunnerObservedHost) ([]finalRunnerObservedHost, error) {
	return nil, errors.New("final campaign host allocation requires Linux")
}
