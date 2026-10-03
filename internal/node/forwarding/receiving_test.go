package forwarding

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission/spending"
	hostingbudget "github.com/dianabuilds/ardents-network/internal/hosting"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node/authority"
	"github.com/dianabuilds/ardents-network/internal/route"
)

type startupHost struct {
	closes int
	err    error
}

func (*startupHost) Sample(context.Context, time.Duration) (hostingbudget.Sample, error) {
	return hostingbudget.Sample{}, nil
}

func (host *startupHost) Close() error {
	host.closes++
	return host.err
}

func TestClosedForwardingStartRejectsMissingDependenciesBeforeOpeningResources(t *testing.T) {
	for _, test := range []struct {
		name string
		omit func(*Config)
	}{
		{"current route", func(config *Config) { config.Authority.CurrentRoute = nil }},
		{"current profile", func(config *Config) { config.Authority.CurrentProfile = nil }},
		{"current duty", func(config *Config) { config.CurrentDuty = nil }},
		{"admission", func(config *Config) { config.VerifyAdmission = nil }},
		{"replenishment", func(config *Config) { config.Replenish = nil }},
		{"endpoint", func(config *Config) { config.LiteralEndpoint = nil }},
		{"clock", func(config *Config) { config.Now = nil }},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "receiving")
			closeErr := errors.New("host close failed")
			host := &startupHost{err: closeErr}
			config := Config{Profile: Profile{Root: root}, Host: host,
				Authority: authority.Source{
					CurrentRoute:   func() (state.ClosedRouteView, error) { return state.ClosedRouteView{}, nil },
					CurrentProfile: func() (state.ClosedProfileView, bool) { return state.ClosedProfileView{}, false },
				},
				CurrentDuty:     func() (state.NodeDuty, error) { return state.NodeDuty{}, nil },
				VerifyAdmission: func(route.ClosedRoleReceiver) route.ClosedAdmissionVerifier { return nil },
				Replenish:       func(route.ClosedRoleReceiver, *spending.Ledger) route.ClosedForwardingReplenisher { return nil },
				LiteralEndpoint: func(string) bool { return true }, Now: time.Now}
			test.omit(&config)
			running, err := Start(config)
			if running != nil || err == nil || !strings.Contains(err.Error(), "dependencies are incomplete") || !errors.Is(err, closeErr) {
				t.Fatalf("Start = %v, %v", running, err)
			}
			if host.closes != 1 {
				t.Fatalf("host closed %d times", host.closes)
			}
			if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("receiving root created before dependency check: %v", err)
			}
		})
	}
}

func TestClosedForwardingReceivingRollbackRetainsInitializationAndCleanupFailures(t *testing.T) {
	for _, test := range []struct {
		name string
		fail func(*receivingOpeners, error)
	}{
		{
			name: "limits",
			fail: func(openers *receivingOpeners, initial error) {
				openers.newLimits = func(func() time.Time) (*route.ClosedDutyLimits, error) { return nil, initial }
			},
		},
		{
			name: "bootstrap",
			fail: func(openers *receivingOpeners, initial error) {
				openers.newBootstrap = func(func() time.Time) (*route.ClosedBootstrapController, error) { return nil, initial }
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			binding := spending.Binding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2},
				ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 4}
			initial := errors.New("fixture initialization failed")
			cleanup := errors.New("fixture spend cleanup failed")
			closes := 0
			openers := defaultReceivingOpeners()
			openers.closeSpends = func(spends *spending.Ledger) error {
				closes++
				return errors.Join(spends.Close(), cleanup)
			}
			test.fail(&openers, initial)

			resources, err := openReceivingResourcesWith(root, binding, time.Now, openers)
			if resources != nil {
				t.Cleanup(func() { _ = resources.Close() })
			}
			if resources != nil || !errors.Is(err, initial) || !errors.Is(err, cleanup) {
				t.Fatalf("rollback = resources %p error %v", resources, err)
			}
			if closes != 1 {
				t.Fatalf("spend lease closed %d times", closes)
			}
			reopened, err := spending.Open(root, binding)
			if err != nil {
				t.Fatalf("rollback retained spend root: %v", err)
			}
			if err := reopened.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestClosedForwardingReceivingTransfersOnlyCompleteResources(t *testing.T) {
	root := t.TempDir()
	binding := spending.Binding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2},
		ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 4}
	resources, err := openReceivingResources(root, binding, time.Now)
	if resources != nil {
		t.Cleanup(func() { _ = resources.Close() })
	}
	if err != nil {
		t.Fatal(err)
	}
	if resources.spends == nil || resources.limits == nil || resources.bootstrap == nil {
		t.Fatal("incomplete receiving resources transferred")
	}
	for range 2 {
		if err := resources.Close(); err != nil {
			t.Fatal(err)
		}
	}
	reopened, err := spending.Open(root, binding)
	if err != nil {
		t.Fatalf("closed owner retained spend root: %v", err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
}
