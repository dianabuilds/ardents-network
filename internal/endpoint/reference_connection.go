package endpoint

import (
	"context"
	"errors"
	"net"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/endpoint/reference"
)

// ReferenceConnectionRequest binds one exact Target Link and its already
// selected outbound Route attachment to the bounded static Reference Site
// Browser Adapter. Application is supplied by this composition, never by the
// caller.
type ReferenceConnectionRequest struct {
	TargetLink string
	Routes     map[string]string
	Connection OutboundConnectionRequest
}

// ReferenceReady reports the local browser URL only after Endpoint has
// authenticated the exact Target and created the scoped loopback origin.
type ReferenceReady struct {
	URL                 string
	AuthenticatedTarget [32]byte
}

// ReferenceOutcome is the classified terminal result of the authenticated
// Service Connection that owned one local browser origin.
type ReferenceOutcome struct {
	Result RuntimeResult
	Err    error
}

// ReferenceConnection owns one live Endpoint Service Connection and its
// browser presentation origin. Close cancels both; ordinary browser traffic is
// never changed or intercepted.
type ReferenceConnection struct {
	cancel context.CancelFunc
	ready  chan ReferenceReady
	done   chan ReferenceOutcome

	mu        sync.Mutex
	server    *reference.Server
	once      sync.Once
	readyOnce sync.Once
}

// StartReferenceConnection validates the Target Link before spending any
// local Connection capability, then starts the native Connection in the
// background. The caller waits on Ready before explicitly opening its URL in
// a supported existing browser.
func (endpoint *endpoint) StartReferenceConnection(ctx context.Context, input ReferenceConnectionRequest) (*ReferenceConnection, error) {
	if endpoint == nil || ctx == nil || input.Connection.Application != nil || input.Connection.OnAuthenticated != nil {
		return nil, errors.New("Reference Connection input is incomplete or attempts to supply an Application path")
	}
	target, err := endpoint.TargetFromLink(input.TargetLink)
	if err != nil {
		return nil, err
	}
	if input.Connection.Target != target {
		return nil, ErrReferenceTargetMismatch
	}
	application, browser := net.Pipe()
	fetcher, err := reference.NewHTTPFetcher(browser)
	if err != nil {
		_ = application.Close()
		_ = browser.Close()
		return nil, err
	}
	lifetime, cancel := context.WithCancel(ctx)
	running := &ReferenceConnection{cancel: cancel, ready: make(chan ReferenceReady, 1), done: make(chan ReferenceOutcome, 1)}
	request := input.Connection
	request.Application = application
	request.OnAuthenticated = func(authenticated [32]byte) error {
		if authenticated != target {
			return ErrReferenceTargetMismatch
		}
		server, openErr := reference.OpenLive(reference.LiveConfig{Target: authenticated, Routes: input.Routes, Fetcher: fetcher})
		if openErr != nil {
			return openErr
		}
		running.mu.Lock()
		running.server = server
		running.mu.Unlock()
		running.publishReady(ReferenceReady{URL: server.URL(), AuthenticatedTarget: authenticated})
		return nil
	}
	go func() {
		result, runErr := endpoint.Connect(lifetime, request)
		_ = application.Close()
		running.closePresentation()
		running.closeReady()
		running.done <- ReferenceOutcome{Result: result, Err: runErr}
		close(running.done)
	}()
	return running, nil
}

// Ready closes once the exact Target has either become available at URL or
// the Service Connection terminates before authentication. An empty value
// means the caller must read Done for the classified failure.
func (connection *ReferenceConnection) Ready() <-chan ReferenceReady {
	if connection == nil {
		return nil
	}
	return connection.ready
}

// Done reports one classified Endpoint terminal result after the browser
// origin is withdrawn.
func (connection *ReferenceConnection) Done() <-chan ReferenceOutcome {
	if connection == nil {
		return nil
	}
	return connection.done
}

// Close cancels the Service Connection and withdraws its browser origin.
func (connection *ReferenceConnection) Close() error {
	if connection == nil {
		return nil
	}
	connection.once.Do(func() {
		connection.cancel()
		connection.closePresentation()
	})
	return nil
}

func (connection *ReferenceConnection) closePresentation() {
	connection.mu.Lock()
	server := connection.server
	connection.server = nil
	connection.mu.Unlock()
	if server != nil {
		_ = server.Close()
	}
}

func (connection *ReferenceConnection) publishReady(value ReferenceReady) {
	connection.readyOnce.Do(func() {
		connection.ready <- value
		close(connection.ready)
	})
}

func (connection *ReferenceConnection) closeReady() {
	connection.readyOnce.Do(func() { close(connection.ready) })
}
