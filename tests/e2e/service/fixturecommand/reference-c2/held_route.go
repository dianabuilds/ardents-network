//go:build browsercompat

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	endpointapi "github.com/dianabuilds/ardents-network/internal/endpoint"
)

// finishHeldUserRoute publishes a User-side readiness acknowledgement only
// after its exact C-2 setup completes, then waits for the test-owned release
// and exposes the existing Route's classified terminal result without opening
// another one.
func finishHeldUserRoute(input config, deadline time.Time, site *endpointapi.UserReferenceSite) error {
	if (input.HeldRouteReady == "") != (input.HeldRouteRelease == "") ||
		(input.HeldRouteReady == "") != (input.HeldRouteUserReady == "") ||
		!validPublisherApplicationPath(input.HeldRouteReady) || !validPublisherApplicationPath(input.HeldRouteUserReady) ||
		!validPublisherApplicationPath(input.HeldRouteRelease) {
		return fmt.Errorf("user C2 fixture held-route control is invalid")
	}
	held, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	if err := waitForTransitCompletion(held, input.HeldRouteReady); err != nil {
		return fmt.Errorf("user C2 fixture held route did not become active: %w", err)
	}
	if err := writePublisherApplicationReady(input.HeldRouteUserReady); err != nil {
		return fmt.Errorf("user C2 fixture held route did not acknowledge readiness: %w", err)
	}
	if err := waitForTransitCompletion(held, input.HeldRouteRelease); err != nil {
		return fmt.Errorf("user C2 fixture held route did not receive release: %w", err)
	}
	class, err := waitForUserReferenceOutcome(deadline, site)
	if err != nil {
		return fmt.Errorf("user C2 fixture held route terminal result: %w", err)
	}
	return json.NewEncoder(os.Stdout).Encode(result{Schema: "ardents-e2e-reference-c2-result-v1", Role: "user", Class: class, Passed: true})
}
