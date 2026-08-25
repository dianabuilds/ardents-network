package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"

	endpointapi "github.com/dianabuilds/ardents-network/internal/endpoint"
)

type userReferenceStarter interface {
	StartUserReferenceSite(context.Context, endpointapi.UserReferenceSiteRequest) *endpointapi.UserReferenceSession
}

// runOfflineUser proves that a stale-but-authentic reachability descriptor does
// not turn a missing Publisher slot into a browser origin or apparent delivery.
func runOfflineUser(user userReferenceStarter, request endpointapi.UserReferenceSiteRequest) error {
	session := user.StartUserReferenceSite(context.Background(), request)
	first, open := <-session.Events()
	if !open || first.State != endpointapi.UserReferenceStarting {
		return errors.New("offline User fixture did not begin its Reference lifecycle")
	}
	last, open := <-session.Events()
	if !open || last.State != endpointapi.UserReferenceUnavailable || last.Class != "service unavailable" || last.Ready.URL != "" {
		return errors.New("offline Publisher fixture did not produce bounded User unavailability")
	}
	if _, open := <-session.Events(); open {
		return errors.New("offline User fixture lifecycle remained open")
	}
	return json.NewEncoder(os.Stdout).Encode(result{Schema: "ardents-e2e-reference-c2-result-v1", Role: "user", Class: last.Class, Passed: true})
}
