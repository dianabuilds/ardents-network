package state_test

import (
	"context"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

func TestAutomaticRefreshSkipsAnActiveInitialWave(t *testing.T) {
	genesis := newFixture(t)
	successor := nextFixture(t, genesis)
	config, closeSources := sourceEnvironment(t, genesis, successor, successor)
	t.Cleanup(closeSources)
	firstRelay := openGatedRelay(t, config.Source.Sources[0].Address)
	secondRelay := openGatedRelay(t, config.Source.Sources[1].Address)
	config.Source.Sources[0].Address = firstRelay.listener.Addr().String()
	config.Source.Sources[1].Address = secondRelay.listener.Addr().String()
	config.AutomaticRefreshInterval = 100 * time.Millisecond
	config.ObserveClock = func() time.Time { return time.Unix(genesis.now, 0).UTC() }
	ticks, automaticResults := make(chan time.Time), make(chan error, 2)
	endpoint, err := state.OpenWithAutomaticTicksForTest(config, ticks, automaticResults)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := endpoint.Close(); closeErr != nil {
			t.Errorf("close State: %v", closeErr)
		}
	})
	initial := make(chan error, 1)
	go func() {
		_, refreshErr := endpoint.Refresh(context.Background())
		initial <- refreshErr
	}()
	firstRelay.awaitAccepted(t)
	secondRelay.awaitAccepted(t)

	// A test-owned tick reaches the scheduler only while the observed initial
	// wave is still blocked, so its busy result proves the intended overlap.
	triggerAutomaticTick(t, ticks)
	if automaticErr := awaitAutomaticResult(t, automaticResults); automaticErr == nil {
		t.Fatal("automatic tick succeeded while the initial source wave was active")
	}
	firstRelay.release()
	secondRelay.release()
	awaitRefresh(t, initial)
	if current, currentErr := endpoint.Current(); currentErr != nil || current.Epoch != 2 {
		t.Fatalf("Current after active automatic tick = %+v, %v", current, currentErr)
	}
	triggerAutomaticTick(t, ticks)
	firstRelay.awaitAccepted(t)
	secondRelay.awaitAccepted(t)
	if automaticErr := awaitAutomaticResult(t, automaticResults); automaticErr != nil {
		t.Fatalf("automatic refresh after initial wave = %v", automaticErr)
	}
}

func TestInitialRefreshHealthyWithoutAutomaticScheduler(t *testing.T) {
	genesis := newFixture(t)
	successor := nextFixture(t, genesis)
	config, closeSources := sourceEnvironment(t, genesis, successor, successor)
	t.Cleanup(closeSources)
	firstRelay := openGatedRelay(t, config.Source.Sources[0].Address)
	secondRelay := openGatedRelay(t, config.Source.Sources[1].Address)
	config.Source.Sources[0].Address = firstRelay.listener.Addr().String()
	config.Source.Sources[1].Address = secondRelay.listener.Addr().String()
	endpoint, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := endpoint.Close(); closeErr != nil {
			t.Errorf("close State: %v", closeErr)
		}
	})
	initial := make(chan error, 1)
	go func() {
		_, refreshErr := endpoint.Refresh(context.Background())
		initial <- refreshErr
	}()
	firstRelay.awaitAccepted(t)
	secondRelay.awaitAccepted(t)
	firstRelay.release()
	secondRelay.release()
	awaitRefresh(t, initial)
	if current, currentErr := endpoint.Current(); currentErr != nil || current.Epoch != 2 {
		t.Fatalf("Current after healthy initial refresh = %+v, %v", current, currentErr)
	}
}

func triggerAutomaticTick(t *testing.T, ticks chan<- time.Time) {
	t.Helper()
	select {
	case ticks <- time.Now():
	case <-time.After(time.Second):
		t.Fatal("automatic scheduler did not receive its test tick")
	}
}

func awaitAutomaticResult(t *testing.T, results <-chan error) error {
	t.Helper()
	select {
	case result := <-results:
		return result
	case <-time.After(time.Second):
		t.Fatal("automatic scheduler did not finish its refresh")
		return nil
	}
}

func awaitRefresh(t *testing.T, results <-chan error) {
	t.Helper()
	select {
	case refreshErr := <-results:
		if refreshErr != nil {
			t.Fatalf("initial Refresh = %v", refreshErr)
		}
	case <-time.After(time.Second):
		t.Fatal("initial Refresh did not complete after relay release")
	}
}

type gatedRelay struct {
	listener    net.Listener
	target      string
	accepted    chan struct{}
	releaseC    chan struct{}
	releaseOnce sync.Once
	runDone     chan struct{}
	forwards    sync.WaitGroup
}

func openGatedRelay(t *testing.T, target string) *gatedRelay {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	relay := &gatedRelay{listener: listener, target: target, accepted: make(chan struct{}, 8), releaseC: make(chan struct{}), runDone: make(chan struct{})}
	go relay.run()
	t.Cleanup(func() {
		_ = listener.Close()
		relay.release()
		<-relay.runDone
		relay.forwards.Wait()
	})
	return relay
}

func (relay *gatedRelay) run() {
	defer close(relay.runDone)
	for {
		client, err := relay.listener.Accept()
		if err != nil {
			return
		}
		relay.accepted <- struct{}{}
		relay.forwards.Add(1)
		go relay.forward(client)
	}
}

func (relay *gatedRelay) forward(client net.Conn) {
	defer relay.forwards.Done()
	defer client.Close()
	<-relay.releaseC
	upstream, err := net.Dial("tcp", relay.target)
	if err != nil {
		return
	}
	defer upstream.Close()
	finished := make(chan struct{})
	go func() { _, _ = io.Copy(upstream, client); close(finished) }()
	_, _ = io.Copy(client, upstream)
	<-finished
}

func (relay *gatedRelay) awaitAccepted(t *testing.T) {
	t.Helper()
	select {
	case <-relay.accepted:
	case <-time.After(time.Second):
		t.Fatal("State source did not reach gated relay")
	}
}

func (relay *gatedRelay) release() {
	relay.releaseOnce.Do(func() { close(relay.releaseC) })
}
