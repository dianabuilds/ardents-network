package introduction

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission/spending"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node/authority"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/carrier"
)

func TestStartRejectsMissingBorrowedDependencies(t *testing.T) {
	for _, test := range []struct {
		name string
		omit func(*Config)
	}{
		{"current route", func(config *Config) { config.Authority.CurrentRoute = nil }},
		{"current duty", func(config *Config) { config.CurrentDuty = nil }},
		{"admission", func(config *Config) { config.VerifyAdmission = nil }},
		{"clock", func(config *Config) { config.Now = nil }},
	} {
		t.Run(test.name, func(t *testing.T) {
			config := Config{Authority: authority.Source{CurrentRoute: func() (state.ClosedRouteView, error) {
				return state.ClosedRouteView{}, nil
			}}, CurrentDuty: func() (state.NodeDuty, error) { return state.NodeDuty{}, nil },
				VerifyAdmission: func(route.ClosedRoleReceiver) route.ClosedAdmissionVerifier { return nil }, Now: time.Now}
			test.omit(&config)
			running, err := Start(config)
			if running != nil || err == nil || !strings.Contains(err.Error(), "dependencies are incomplete") {
				t.Fatalf("Start = %v, %v", running, err)
			}
		})
	}
}

func TestClosedIntroductionDrainRetainsAcceptedCarrierCloseFailure(t *testing.T) {
	for _, test := range []struct {
		name     string
		capacity bool
		kind     carrier.ClosedSharedCarrierKind
	}{
		{name: "capacity refusal", capacity: true, kind: carrier.ClosedSharedNode},
		{name: "direct refusal", kind: carrier.ClosedSharedDirect},
		{name: "accepted Node carrier", kind: carrier.ClosedSharedNode},
	} {
		t.Run(test.name, func(t *testing.T) {
			first, second := errors.New("first physical close failure"), errors.New("second physical close failure")
			for _, outcome := range []struct {
				name   string
				result error
				want   []error
			}{
				{"plain failure", first, []error{first}},
				{"already closed", net.ErrClosed, nil},
				{"wrapped already closed", &net.OpError{Op: "close", Net: "test", Err: net.ErrClosed}, nil},
				{"compound", errors.Join(net.ErrClosed, first, second), []error{first, second}},
				{"wrapped compound", &net.OpError{Op: "close", Net: "test", Err: errors.Join(net.ErrClosed, first, second)}, []error{first, second}},
			} {
				t.Run(outcome.name, func(t *testing.T) {
					checkIntroductionAcceptedCloseFailure(t, test.capacity, test.kind, outcome.result, outcome.want)
				})
			}
		})
	}
}

func checkIntroductionAcceptedCloseFailure(t *testing.T, capacity bool, kind carrier.ClosedSharedCarrierKind, closeErr error, expected []error) {
	t.Helper()
	local, peer := net.Pipe()
	defer peer.Close()
	connection := &acceptedCloseFailureConn{Conn: local, closed: make(chan struct{}), err: closeErr}
	listener := &oneAcceptedCarrierListener{ready: make(chan struct{}), connection: connection, kind: kind}
	spends, err := spending.Open(t.TempDir(), spending.Binding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 1})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	server := &Server{config: Config{CurrentDuty: func() (state.NodeDuty, error) {
		return state.NodeDuty{}, errors.New("current State is unavailable")
	}}, listener: listener, spends: spends, capacity: make(chan struct{}, 1),
		cancel: cancel, done: make(chan error, 1), drained: make(chan struct{})}
	if capacity {
		server.capacity <- struct{}{} // Force capacity refusal after the first authenticated accept.
	}
	go server.run(ctx)
	t.Cleanup(func() {
		_ = server.stop()
		select {
		case <-server.drained:
		case <-time.After(time.Second):
			t.Error("introduction server did not drain")
		}
	})
	close(listener.ready)
	select {
	case <-connection.closed:
	case <-time.After(time.Second):
		t.Fatal("introduction did not close accepted Carrier")
	}
	_ = server.stop()
	select {
	case <-server.drained:
	case <-time.After(time.Second):
		t.Fatal("introduction server did not drain")
	}
	if len(expected) == 0 && server.drainErr != nil {
		t.Fatalf("benign close became drain failure: %v", server.drainErr)
	}
	for _, cause := range expected {
		if !errors.Is(server.drainErr, cause) {
			t.Errorf("joined drain lost %v: %v", cause, server.drainErr)
		}
	}
}

func TestIntroductionDrainTimeoutRetainsSpendRootUntilAcceptedWorkerJoins(t *testing.T) {
	local, peer := net.Pipe()
	defer peer.Close()
	listener := &oneAcceptedCarrierListener{ready: make(chan struct{}), connection: local, kind: carrier.ClosedSharedNode}
	spends, err := spending.Open(t.TempDir(), spending.Binding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 1})
	if err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	server := &Server{config: Config{CurrentDuty: func() (state.NodeDuty, error) {
		close(entered)
		<-release
		return state.NodeDuty{}, errors.New("current State is unavailable")
	}}, listener: listener, spends: spends, capacity: make(chan struct{}, 1),
		cancel: cancel, done: make(chan error, 1), drained: make(chan struct{})}
	go server.run(ctx)
	t.Cleanup(func() {
		_ = server.stop()
		select {
		case <-release:
		default:
			close(release)
		}
		select {
		case <-server.drained:
		case <-time.After(time.Second):
			t.Error("introduction server did not join after release")
		}
	})
	close(listener.ready)
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("accepted introduction worker did not enter State callback")
	}
	if err := server.Drain(context.Background(), 20*time.Millisecond); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("drain before worker join: %v", err)
	}
	select {
	case <-server.drained:
		t.Fatal("role joined before accepted worker")
	default:
	}
	if _, err := spends.IntroductionSlots(); err != nil {
		t.Fatalf("spend root released before accepted worker: %v", err)
	}
	close(release)
	select {
	case <-server.drained:
	case <-time.After(time.Second):
		t.Fatal("role did not join after accepted worker")
	}
	if _, err := spends.IntroductionSlots(); err == nil {
		t.Fatal("spend root remained open after joined shutdown")
	}
}

type acceptedCloseFailureConn struct {
	net.Conn
	closed chan struct{}
	err    error
}

func (connection *acceptedCloseFailureConn) Close() error {
	_ = connection.Conn.Close()
	select {
	case <-connection.closed:
	default:
		close(connection.closed)
	}
	return connection.err
}

type oneAcceptedCarrierListener struct {
	ready      chan struct{}
	connection net.Conn
	kind       carrier.ClosedSharedCarrierKind
	served     bool
}

func (listener *oneAcceptedCarrierListener) Accept(ctx context.Context, _ time.Duration) (carrier.ClosedSharedCarrier, error) {
	if !listener.served {
		select {
		case <-listener.ready:
		case <-ctx.Done():
			return carrier.ClosedSharedCarrier{}, ctx.Err()
		}
		listener.served = true
		return carrier.ClosedSharedCarrier{Kind: listener.kind, Connection: listener.connection}, nil
	}
	<-ctx.Done()
	return carrier.ClosedSharedCarrier{}, ctx.Err()
}

func (*oneAcceptedCarrierListener) Close() error { return nil }

func TestCarrierCleanupRetainsOneCompleteFailure(t *testing.T) {
	server := new(Server)
	first, second, later := errors.New("first failure"), errors.New("independent first failure"), errors.New("later failure")
	server.recordCarrierCleanup(errors.Join(net.ErrClosed, first, second))
	retained := server.cleanupErr
	for range 1000 {
		server.recordCarrierCleanup(later)
	}
	if server.cleanupErr != retained {
		t.Fatal("cleanup state grows with subsequent failed Carriers")
	}
	if !errors.Is(server.cleanupErr, first) || !errors.Is(server.cleanupErr, second) || errors.Is(server.cleanupErr, later) {
		t.Fatal("first complete cleanup result changed")
	}
}
