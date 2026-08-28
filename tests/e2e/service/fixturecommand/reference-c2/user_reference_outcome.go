package main

import (
	"errors"
	"time"

	endpointapi "github.com/dianabuilds/ardents-network/internal/endpoint"
)

// waitForUserReferenceOutcome observes only the classified terminal result of
// the exact C-2 Service Connection. It does not choose a retry or destination.
func waitForUserReferenceOutcome(deadline time.Time, site *endpointapi.UserReferenceSite) (string, error) {
	result, err := waitForUserReferenceRuntimeOutcome(deadline, site)
	return result.Class, err
}

func waitForUserReferenceRuntimeOutcome(deadline time.Time, site *endpointapi.UserReferenceSite) (endpointapi.RuntimeResult, error) {
	if site == nil {
		return endpointapi.RuntimeResult{}, errors.New("user C2 fixture Reference Site is unavailable")
	}
	select {
	case outcome := <-site.Done():
		if outcome.Result.Class == "" {
			return endpointapi.RuntimeResult{}, errors.New("user C2 fixture did not receive a classified Service Connection result")
		}
		return outcome.Result, nil
	case <-time.After(time.Until(deadline)):
		return endpointapi.RuntimeResult{}, errors.New("user C2 fixture did not receive a terminal result")
	}
}
