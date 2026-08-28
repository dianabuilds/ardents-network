package main

import (
	"errors"
	"fmt"
	"time"

	endpointapi "github.com/dianabuilds/ardents-network/internal/endpoint"
)

func userReferenceTerminalSnapshot(site *endpointapi.UserReferenceSite) string {
	if site == nil {
		return "site-unavailable"
	}
	select {
	case outcome := <-site.Done():
		return fmt.Sprintf("class=%q reason=%q error=%v", outcome.Result.Class, outcome.Result.Reason, outcome.Err)
	case <-time.After(250 * time.Millisecond):
		return "pending-after-250ms"
	}
}

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
