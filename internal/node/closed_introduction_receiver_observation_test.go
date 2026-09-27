//go:build linux

package node

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/node/introduction"
)

// Keep actual role construction while observing the child owner through its
// read-only qualification view. Node placement and State remain fixtures.
func observedIntroductionStart(observed **closedIntroductionServer) func(Config) (func() error, error) {
	return func(config Config) (func() error, error) {
		resolved, err := resolveConfig(config)
		if err != nil {
			return nil, err
		}
		if err := resolved.openClosedHosting(); err != nil {
			return nil, err
		}
		snapshot, err := currentFacts(resolved)
		if err != nil {
			return nil, errors.Join(err, resolved.host.Close())
		}
		server, err := newClosedIntroductionServer(resolved, snapshot)
		if err != nil {
			return nil, errors.Join(err, resolved.host.Close())
		}
		*observed = server
		return func() error {
			stopErr := server.Stop()
			select {
			case <-server.Joined():
				return errors.Join(stopErr, server.Drain(context.Background(), time.Second), resolved.host.Close())
			case <-time.After(testLifecycleWait):
				return errors.New("observed Introduction receiver did not drain")
			}
		}, nil
	}
}

type closedIntroductionServer = introduction.Server
type introductionReceiverObservation = introduction.Observation

func observeIntroductionReceiver(t *testing.T, server *closedIntroductionServer, phase string) introductionReceiverObservation {
	t.Helper()
	snapshot, err := server.Observe(phase)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}
