package main

import (
	"errors"
	"time"

	endpointapi "github.com/dianabuilds/ardents-network/internal/endpoint"
)

// waitForUserReferenceOutcome observes only the classified terminal result of
// the exact C-2 Service Connection. It does not choose a retry or destination.
func waitForUserReferenceOutcome(deadline time.Time, site *endpointapi.UserReferenceSite) (string, error) {
	if site == nil {
		return "", errors.New("user C2 fixture Reference Site is unavailable")
	}
	select {
	case outcome := <-site.Done():
		if outcome.Result.Class == "" {
			return "", errors.New("user C2 fixture did not receive a classified Service Connection result")
		}
		return outcome.Result.Class, nil
	case <-time.After(time.Until(deadline)):
		return "", errors.New("user C2 fixture did not receive a terminal result")
	}
}
