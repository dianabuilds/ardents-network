//go:build referencec2

package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"

	endpointapi "github.com/dianabuilds/ardents-network/internal/endpoint"
)

type alphaUserReferenceStarter interface {
	StartAlphaUserReferenceSite(context.Context, endpointapi.AlphaUserReferenceSiteRequest) *endpointapi.UserReferenceSession
}

// runOfflineAlphaUser proves that the verified alpha route does not make a
// missing Publisher slot appear as a browser-ready service.
func runOfflineAlphaUser(user alphaUserReferenceStarter, request endpointapi.AlphaUserReferenceSiteRequest) error {
	return runOfflineSession(user.StartAlphaUserReferenceSite(context.Background(), request))
}

func runOfflineSession(session *endpointapi.UserReferenceSession) error {
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
