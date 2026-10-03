package join

import (
	"context"
	"crypto/tls"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission/spending"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node/authority"
	"github.com/dianabuilds/ardents-network/internal/resource"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/carrier"
)

type monitoredHost struct {
	sample func(context.Context) (resource.HostingSample, error)
	closed chan struct{}
}

func (host *monitoredHost) Sample(ctx context.Context, _ time.Duration) (resource.HostingSample, error) {
	return host.sample(ctx)
}
func (*monitoredHost) AdmissionVerifier(route.ClosedRoleReceiver) route.ClosedAdmissionVerifier {
	return nil
}
func (*monitoredHost) Replenisher(route.ClosedRoleReceiver, *spending.Ledger) route.ClosedForwardingReplenisher {
	return nil
}
func (host *monitoredHost) Close() error {
	if host.closed != nil {
		close(host.closed)
	}
	return nil
}

type monitoredListener struct {
	closed chan struct{}
}

func (*monitoredListener) Accept(ctx context.Context, _ time.Duration) (carrier.ClosedSharedCarrier, error) {
	<-ctx.Done()
	return carrier.ClosedSharedCarrier{}, ctx.Err()
}

func (listener *monitoredListener) Close() error {
	close(listener.closed)
	return nil
}

func TestJoinHostMonitorIgnoresSampleCanceledByExplicitStop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	listener := &monitoredListener{closed: make(chan struct{})}
	server := &closedDataJoinServer{host: &monitoredHost{sample: func(ctx context.Context) (resource.HostingSample, error) {
		close(started)
		<-ctx.Done()
		return resource.HostingSample{Observation: resource.HostingObservation{Drain: true}}, errors.New("hosting operation is unavailable")
	}}, listener: listener, cancel: cancel}
	ticks := make(chan time.Time, 1)
	ticks <- time.Now()
	result := make(chan error, 1)
	go func() { result <- server.monitorHost(ctx, ticks) }()
	select {
	case <-started:
	case <-time.After(time.Second):
		cancel()
		t.Fatal("host monitor did not start sampling")
	}
	if err := server.stop(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("explicit stop became host drain: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("host monitor did not join")
	}
	select {
	case <-listener.closed:
	default:
		t.Fatal("explicit stop did not close listener")
	}
}

func TestJoinHostMonitorPreservesSampleFailureBeforeStop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sampleErr := errors.New("host sample failed")
	listener := &monitoredListener{closed: make(chan struct{})}
	server := &closedDataJoinServer{host: &monitoredHost{sample: func(context.Context) (resource.HostingSample, error) {
		return resource.HostingSample{}, sampleErr
	}}, listener: listener, cancel: cancel}
	ticks := make(chan time.Time, 1)
	ticks <- time.Now()
	if err := server.monitorHost(ctx, ticks); !errors.Is(err, sampleErr) || !strings.Contains(err.Error(), "JOIN host allowance requires drain") {
		t.Fatalf("host failure was not retained: %v", err)
	}
	select {
	case <-listener.closed:
	default:
		t.Fatal("host failure did not stop listener")
	}
}

func TestJoinHostMonitorPreservesObservedDrainBeforeStop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	listener := &monitoredListener{closed: make(chan struct{})}
	server := &closedDataJoinServer{host: &monitoredHost{sample: func(context.Context) (resource.HostingSample, error) {
		return resource.HostingSample{Observation: resource.HostingObservation{Drain: true}}, nil
	}}, listener: listener, cancel: cancel}
	ticks := make(chan time.Time, 1)
	ticks <- time.Now()
	if err := server.monitorHost(ctx, ticks); err == nil || !strings.Contains(err.Error(), "JOIN host allowance requires drain") {
		t.Fatalf("observed drain was not retained: %v", err)
	}
	select {
	case <-listener.closed:
	default:
		t.Fatal("observed drain did not stop listener")
	}
}

func TestJoinExplicitStopJoinsCanceledSampleAndOwnedResources(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	host := &monitoredHost{closed: make(chan struct{}), sample: func(ctx context.Context) (resource.HostingSample, error) {
		close(started)
		<-ctx.Done()
		return resource.HostingSample{Observation: resource.HostingObservation{Drain: true}}, errors.New("hosting operation is unavailable")
	}}
	listener := &monitoredListener{closed: make(chan struct{})}
	server := &closedDataJoinServer{host: host, listener: listener, cancel: cancel,
		done: make(chan error, 1), drained: make(chan struct{})}
	defer server.stop()
	go server.run(ctx)
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("host monitor did not start sampling")
	}
	if err := server.stop(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-server.drained:
	case <-time.After(time.Second):
		t.Fatal("JOIN shutdown did not join")
	}
	if err := <-server.done; err != nil {
		t.Fatalf("explicit stop failed accept: %v", err)
	}
	if server.drainErr != nil {
		t.Fatalf("explicit stop failed drain: %v", server.drainErr)
	}
	select {
	case <-host.closed:
	default:
		t.Fatal("JOIN did not close owned Hosting handle")
	}
	select {
	case <-listener.closed:
	default:
		t.Fatal("JOIN did not close listener")
	}
}

func TestStartRejectsMissingBorrowedDependencies(t *testing.T) {
	for _, test := range []struct {
		name string
		omit func(*Config)
	}{
		{"current route", func(config *Config) { config.Authority.CurrentRoute = nil }},
		{"current duty", func(config *Config) { config.CurrentDuty = nil }},
		{"open host", func(config *Config) { config.OpenHost = nil }},
		{"clock", func(config *Config) { config.Now = nil }},
	} {
		t.Run(test.name, func(t *testing.T) {
			config := Config{Authority: authority.Source{CurrentRoute: func() (state.ClosedRouteView, error) {
				return state.ClosedRouteView{}, nil
			}}, CurrentDuty: func() (state.NodeDuty, error) { return state.NodeDuty{}, nil },
				OpenHost: func(string) (Host, error) { return nil, nil }, Now: time.Now}
			test.omit(&config)
			running, err := Start(config)
			if running != nil || err == nil || !strings.Contains(err.Error(), "dependencies are incomplete") {
				t.Fatalf("Start = %v, %v", running, err)
			}
		})
	}
}

func TestValidateRefusesUnavailableCurrentReceiver(t *testing.T) {
	profile := Profile{HostingRoot: t.TempDir(), AdmissionRoot: t.TempDir(), Certificate: tls.Certificate{PrivateKey: new(int)},
		ConnectionLimit: 1, DrainTimeout: time.Second}
	snapshot := state.NodeDuty{ProbeEndpoint: "127.0.0.1:41000", CarrierProfile: string(carrier.ClosedCarrierTCP)}
	err := Validate(profile, authority.Source{}, snapshot, time.Now(), true)
	if err == nil || !strings.Contains(err.Error(), "State projection is unavailable") {
		t.Fatalf("unavailable current JOIN receiver: %v", err)
	}
}
