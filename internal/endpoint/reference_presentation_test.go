package endpoint

import (
	"errors"
	"net/http"
	"runtime"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/endpoint/reference"
	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

// These coordinator tests construct ReferenceConnection directly because the
// public starters intentionally expose no presentation factory; explicit
// channels make authenticated install and withdrawal interleavings deterministic.
func TestReferenceConnectionCloseReturnsPresentationWithdrawalFailure(t *testing.T) {
	withdrawalFailure := errors.New("presentation withdrawal failed")
	connection := &ReferenceConnection{cancel: func() {}, presentation: referencePresentationErrorCloser(func() error {
		return withdrawalFailure
	})}
	if err := connection.Close(); !errors.Is(err, withdrawalFailure) {
		t.Fatalf("Reference Connection Close error = %v", err)
	}
}

func TestReferenceConnectionDoneReportsPresentationWithdrawalFailure(t *testing.T) {
	connectionFailure := errors.New("Service Connection failed")
	withdrawalFailure := errors.New("presentation withdrawal failed")
	connection := &ReferenceConnection{
		ready:  make(chan ReferenceReady, 1),
		done:   make(chan ReferenceOutcome, 1),
		closed: make(chan struct{}),
		presentation: referencePresentationErrorCloser(func() error {
			return withdrawalFailure
		}),
	}
	connection.completeReferenceConnection(RuntimeResult{}, connectionFailure)
	outcome := <-connection.Done()
	if !errors.Is(outcome.Err, connectionFailure) || !errors.Is(outcome.Err, withdrawalFailure) {
		t.Fatalf("Reference Connection outcome error = %v", outcome.Err)
	}
}

func TestReferenceConnectionCloseWaitsForConcurrentPresentationWithdrawal(t *testing.T) {
	previous := runtime.GOMAXPROCS(1)
	t.Cleanup(func() { runtime.GOMAXPROCS(previous) })
	releaseStarted := make(chan struct{})
	releaseAllowed := make(chan struct{})
	terminalCloseReturned := make(chan error, 1)
	publicCloseStarted := make(chan struct{})
	publicCloseReturned := make(chan error, 1)
	withdrawalFailure := errors.New("presentation withdrawal failed")
	connection := &ReferenceConnection{
		cancel:       func() { close(publicCloseStarted) },
		presentation: referencePresentationErrorCloser(func() error { return withdrawalFailure }),
		release: func() {
			close(releaseStarted)
			<-releaseAllowed
		},
	}
	go func() {
		terminalCloseReturned <- connection.closePresentation()
	}()
	<-releaseStarted
	go func() {
		publicCloseReturned <- connection.Close()
	}()
	<-publicCloseStarted
	runtime.Gosched()
	select {
	case <-publicCloseReturned:
		close(releaseAllowed)
		<-terminalCloseReturned
		t.Fatal("concurrent Close returned before presentation withdrawal completed")
	default:
	}
	close(releaseAllowed)
	terminalErr := <-terminalCloseReturned
	publicErr := <-publicCloseReturned
	if !errors.Is(terminalErr, withdrawalFailure) || !errors.Is(publicErr, withdrawalFailure) {
		t.Fatalf("concurrent presentation withdrawal errors = terminal %v / public %v", terminalErr, publicErr)
	}
}

func TestReferenceConnectionCloseWaitsForLatePresentationCleanup(t *testing.T) {
	previous := runtime.GOMAXPROCS(1)
	t.Cleanup(func() { runtime.GOMAXPROCS(previous) })
	cancelStarted := make(chan struct{})
	connection := &ReferenceConnection{cancel: func() { close(cancelStarted) }}
	setup, accepted := connection.beginPresentation()
	if !accepted {
		t.Fatal("live Reference Connection rejected presentation setup")
	}
	closeReturned := make(chan error, 1)
	go func() {
		closeReturned <- connection.Close()
	}()
	<-cancelStarted
	for {
		connection.mu.Lock()
		closing := connection.presentationClosed
		connection.mu.Unlock()
		if closing {
			break
		}
		runtime.Gosched()
	}
	select {
	case <-closeReturned:
		t.Fatal("Close returned while authenticated presentation setup was still live")
	default:
	}
	releases, closes := 0, 0
	withdrawalFailure := errors.New("late presentation withdrawal failed")
	installed := connection.installPresentation(setup, referencePresentationErrorCloser(func() error {
		closes++
		return withdrawalFailure
	}), func() { releases++ })
	if installed {
		t.Fatal("presentation was installed after Reference Connection Close")
	}
	closeErr := <-closeReturned
	if releases != 1 || closes != 1 {
		t.Fatalf("late presentation cleanup releases=%d closes=%d, want exactly one each", releases, closes)
	}
	if !errors.Is(closeErr, withdrawalFailure) {
		t.Fatalf("late presentation Close error = %v", closeErr)
	}
}

func TestReferenceConnectionInvalidPresentationCompletesSetupBeforeClose(t *testing.T) {
	previous := runtime.GOMAXPROCS(1)
	t.Cleanup(func() { runtime.GOMAXPROCS(previous) })
	cancelStarted := make(chan struct{})
	connection := &ReferenceConnection{cancel: func() { close(cancelStarted) }}
	setup, accepted := connection.beginPresentation()
	if !accepted {
		t.Fatal("live Reference Connection rejected presentation setup")
	}
	releaseStarted := make(chan struct{})
	releaseAllowed := make(chan struct{})
	installReturned := make(chan struct{})
	releases, installed := 0, false
	go func() {
		installed = connection.installPresentation(setup, nil, func() {
			releases++
			close(releaseStarted)
			<-releaseAllowed
		})
		close(installReturned)
	}()
	<-releaseStarted
	closeReturned := make(chan struct{})
	go func() {
		_ = connection.Close()
		close(closeReturned)
	}()
	<-cancelStarted
	runtime.Gosched()
	closeReturnedBeforeRelease := false
	select {
	case <-closeReturned:
		closeReturnedBeforeRelease = true
	default:
	}
	close(releaseAllowed)
	<-installReturned
	if installed {
		t.Fatal("invalid nil presentation was installed")
	}
	connection.mu.Lock()
	retainedSetup := connection.presentationSetup == setup
	connection.mu.Unlock()
	if retainedSetup {
		connection.abandonPresentation(setup)
	}
	<-closeReturned
	if closeReturnedBeforeRelease {
		t.Fatal("Close returned before invalid presentation release completed")
	}
	if retainedSetup {
		t.Fatal("invalid presentation retained the setup barrier and blocked Close")
	}
	if releases != 1 {
		t.Fatalf("invalid presentation release calls = %d, want exactly one", releases)
	}
}

type referencePresentationErrorCloser func() error

func (close referencePresentationErrorCloser) Close() error { return close() }

func TestOpenReferencePresentationRequiresAnAuthenticatedTarget(t *testing.T) {
	if server, err := OpenReferencePresentation(ReferencePresentation{Document: reference.Resource{ContentType: "text/html", Body: []byte("page")}}); err == nil || server != nil {
		t.Fatalf("missing Target presentation result = (%v, %v)", server, err)
	}
	server, err := OpenReferencePresentation(ReferencePresentation{AuthenticatedTarget: [32]byte{1},
		Document: reference.Resource{ContentType: "text/html", Body: []byte("page")}})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	response, err := http.Get(server.URL())
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("Reference presentation status = %d", response.StatusCode)
	}
}

func TestEndpointOpenReferenceFromLinkRequiresExactAuthenticatedTarget(t *testing.T) {
	endpoint := &endpoint{network: targetLinkBytes(1)}
	selected := targetLinkBytes(33)
	text, err := targetlink.Encode(targetlink.Link{Network: endpoint.network, Target: selected})
	if err != nil {
		t.Fatal(err)
	}
	input := ReferencePresentation{AuthenticatedTarget: targetLinkBytes(34),
		Document: reference.Resource{ContentType: "text/html", Body: []byte("page")}}
	if server, err := endpoint.OpenReferenceFromLink(text, input); !errors.Is(err, ErrReferenceTargetMismatch) || server != nil {
		t.Fatalf("mismatched presentation result = (%v, %v)", server, err)
	}
	input.AuthenticatedTarget = selected
	server, err := endpoint.OpenReferenceFromLink(text, input)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	response, err := http.Get(server.URL())
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("Reference presentation status = %d", response.StatusCode)
	}
}
