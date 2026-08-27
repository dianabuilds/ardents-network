package main

import (
	"errors"
	"io"
	"net/http"
	"os"
	"time"

	endpointapi "github.com/dianabuilds/ardents-network/internal/endpoint"
)

// waitForDynamicPublisherWithdrawal proves that closing the local Publisher
// Application terminates the selected Service Connection. A subsequent name
// request must fail at the local Browser Entry; it must not retain a usable
// route or select another destination.
func waitForDynamicPublisherWithdrawal(deadline time.Time, site *endpointapi.UserReferenceSite, client *http.Client, origin, browserEntryStatePath string) (string, error) {
	class, err := waitForUserReferenceOutcome(deadline, site)
	if err != nil {
		return "", err
	}
	if browserEntryStatePath != "" {
		if _, stateErr := os.Stat(browserEntryStatePath); !errors.Is(stateErr, os.ErrNotExist) {
			return "", errors.New("dynamic Browser Entry retained its withdrawn alpha proxy")
		}
		return class, nil
	}
	if client == nil {
		return "", errors.New("dynamic fixture browser client is unavailable")
	}
	response, requestErr := client.Get(origin)
	if requestErr != nil {
		return class, nil
	}
	_, _ = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()
	if response.StatusCode < http.StatusBadRequest {
		return "", errors.New("dynamic Publisher remained reachable after local Application closed")
	}
	return class, nil
}
