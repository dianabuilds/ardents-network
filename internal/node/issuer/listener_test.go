package issuer

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node/authority"
	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestStartRejectsMissingBorrowedDependencies(t *testing.T) {
	for _, test := range []struct {
		name string
		omit func(*Config)
	}{
		{"current route", func(config *Config) { config.Authority.CurrentRoute = nil }},
		{"current profile", func(config *Config) { config.Authority.CurrentProfile = nil }},
		{"current duty", func(config *Config) { config.CurrentDuty = nil }},
		{"admission", func(config *Config) { config.VerifyAdmission = nil }},
		{"clock", func(config *Config) { config.Now = nil }},
	} {
		t.Run(test.name, func(t *testing.T) {
			config := Config{Authority: authority.Source{
				CurrentRoute:   func() (state.ClosedRouteView, error) { return state.ClosedRouteView{}, nil },
				CurrentProfile: func() (state.ClosedProfileView, bool) { return state.ClosedProfileView{}, false },
			}, CurrentDuty: func() (state.NodeDuty, error) { return state.NodeDuty{}, nil },
				VerifyAdmission: func(route.ClosedRoleReceiver) route.ClosedAdmissionVerifier { return nil }, Now: time.Now}
			test.omit(&config)
			running, err := Start(config)
			if running != nil || err == nil || !strings.Contains(err.Error(), "dependencies are incomplete") {
				t.Fatalf("Start = %v, %v", running, err)
			}
		})
	}
}

func TestReleaseErrorsRetainFirstUnexpectedFailure(t *testing.T) {
	first := errors.New("first cleanup failure")
	second := errors.New("second cleanup failure")
	var releases releaseErrors
	releases.record(errors.Join(net.ErrClosed, first))
	releases.record(second)
	if err := releases.result(); !errors.Is(err, first) || errors.Is(err, second) || errors.Is(err, net.ErrClosed) {
		t.Fatalf("cleanup result = %v, want only first unexpected failure", err)
	}
}

func TestReleaseErrorsIgnoreWrappedClosed(t *testing.T) {
	var releases releaseErrors
	releases.record(fmt.Errorf("released: %w", net.ErrClosed))
	if err := releases.result(); err != nil {
		t.Fatalf("cleanup result = %v, want nil", err)
	}
}
