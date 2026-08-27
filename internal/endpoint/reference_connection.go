package endpoint

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"

	"github.com/dianabuilds/ardents-network/internal/endpoint/reference"
	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
)

// ReferenceConnectionRequest binds one exact Target Link and its already
// selected outbound Route attachment to the bounded static Reference Site
// Browser Adapter. Application is supplied by this composition, never by the
// caller.
type ReferenceConnectionRequest struct {
	TargetLink string
	Routes     map[string]string
	Connection OutboundConnectionRequest
	Browser    ReferenceBrowser
}

// AlphaReferenceConnectionRequest carries one already-verified bounded alpha
// binding into the same authenticated Service Connection flow as a Target Link.
// It does not resolve an alpha Service Link, choose a fallback Target, or turn
// the alpha name into a canonical Namespace claim.
type AlphaReferenceConnectionRequest struct {
	Binding    alpha.Binding
	Routes     map[string]string
	Connection OutboundConnectionRequest
	Browser    ReferenceBrowser
}

// AlphaTransparentConnectionRequest carries an already verified alpha binding
// into one payload-neutral HTTP/1.1 Service bridge. It deliberately has no
// content, CMS, route-map, or alternate-destination input.
type AlphaTransparentConnectionRequest struct {
	Binding    alpha.Binding
	Connection OutboundConnectionRequest
	Browser    ReferenceBrowser
}

// ReferenceReady reports the participant-visible browser URL only after
// Endpoint authenticated the exact Target and created the scoped local origin.
// AlphaProxyURL is an Endpoint-local Browser Entry input, never a visible
// Service address; it is empty for ordinary Target-Link presentation.
type ReferenceReady struct {
	URL                 string
	AlphaProxyURL       string
	AuthenticatedTarget [32]byte
}

// ReferenceBrowser receives only an already-authenticated, newly-created
// Reference Site URL. It cannot select a Target, alter the Service Connection,
// or ask Endpoint to open an arbitrary URL. A provisional alpha URL requires a
// separately installed local Browser Entry to use AlphaProxyURL; a platform
// adapter may then open the participant's selected browser.
type ReferenceBrowser interface {
	OpenReference(context.Context, ReferenceReady) error
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
	closed chan struct{}

	mu           sync.Mutex
	presentation io.Closer
	release      func()
	once         sync.Once
	readyOnce    sync.Once
}

// StartReferenceConnection validates the Target Link before spending any
// local Connection capability, then starts the native Connection in the
// background. When Browser is supplied, it receives the exact local URL only
// after target authentication and origin creation succeed. Otherwise the
// caller waits on Ready before explicitly opening that URL in a supported
// existing browser.
func (endpoint *endpoint) StartReferenceConnection(ctx context.Context, input ReferenceConnectionRequest) (*ReferenceConnection, error) {
	if endpoint == nil || ctx == nil {
		return nil, errors.New("reference Connection input is incomplete or attempts to supply an Application path")
	}
	target, err := endpoint.TargetFromLink(input.TargetLink)
	if err != nil {
		return nil, err
	}
	return endpoint.startReferenceConnection(ctx, input, target, "")
}

// StartAlphaReferenceConnection starts a Reference Site only after the caller
// has obtained a Binding from a verified active alpha Corpus. Its browser
// presentation is one provisional HTTP-only `.ard` browser name, carried only
// through Endpoint's loopback alpha proxy. It is neither HTTPS, public DNS,
// nor a canonical Namespace name.
func (endpoint *endpoint) StartAlphaReferenceConnection(ctx context.Context, input AlphaReferenceConnectionRequest) (*ReferenceConnection, error) {
	if endpoint == nil || input.Binding.Network() != endpoint.network {
		return nil, ErrAlphaBindingNetwork
	}
	return endpoint.startReferenceConnection(ctx, ReferenceConnectionRequest{Routes: input.Routes,
		Connection: input.Connection, Browser: input.Browser}, input.Binding.Target(), alphaBrowserHostname(input.Binding))
}

// StartAlphaTransparentConnection starts one alpha HTTP origin only after its
// exact alpha binding and Service Target have been authenticated. It carries
// ordinary HTTP request/response semantics over the selected Service
// Connection; it does not make the Endpoint an Internet proxy or a CMS
// adapter.
func (endpoint *endpoint) StartAlphaTransparentConnection(ctx context.Context, input AlphaTransparentConnectionRequest) (*ReferenceConnection, error) {
	if endpoint == nil || ctx == nil || input.Binding.Network() != endpoint.network || input.Connection.Application != nil || input.Connection.OnAuthenticated != nil {
		return nil, errors.New("transparent alpha Service input is incomplete or attempts to supply an Application path")
	}
	target := input.Binding.Target()
	if target == [32]byte{} || input.Connection.Target != target {
		return nil, ErrReferenceTargetMismatch
	}
	application, browser := net.Pipe()
	lifetime, cancel := context.WithCancel(ctx)
	running := &ReferenceConnection{cancel: cancel, ready: make(chan ReferenceReady, 1), done: make(chan ReferenceOutcome, 1), closed: make(chan struct{})}
	request := input.Connection
	request.Application = application
	// HTTP/1.1 uses a Publisher application's EOF as the end of this selected
	// persistent origin. The native stream otherwise preserves ordinary
	// bidirectional half-close behavior for non-browser applications.
	request.closeApplicationOnRemoteTerminal = true
	request.OnAuthenticated = func(authenticated [32]byte) error {
		if authenticated != target {
			return ErrReferenceTargetMismatch
		}
		hostname := alphaBrowserHostname(input.Binding)
		presentation, err := reference.OpenTransparent(reference.TransparentConfig{Target: authenticated, Hostname: hostname, Connection: browser})
		if err != nil {
			return err
		}
		proxyURL, release, err := endpoint.openAlphaTransparentBrowserRoute(hostname, presentation)
		if err != nil {
			_ = presentation.Close()
			return err
		}
		running.mu.Lock()
		running.presentation = presentation
		running.release = release
		running.mu.Unlock()
		ready := ReferenceReady{URL: "http://" + hostname + "/", AlphaProxyURL: proxyURL, AuthenticatedTarget: authenticated}
		if input.Browser != nil {
			if err := input.Browser.OpenReference(lifetime, ready); err != nil {
				return errors.Join(errors.New("selected Reference browser did not open the authenticated origin"), err)
			}
		}
		running.publishReady(ready)
		return nil
	}
	go func() {
		result, runErr := endpoint.Connect(lifetime, request)
		_ = application.Close()
		running.closePresentation()
		running.closeReady()
		running.done <- ReferenceOutcome{Result: result, Err: runErr}
		close(running.done)
		close(running.closed)
	}()
	return running, nil
}

func (endpoint *endpoint) startReferenceConnection(ctx context.Context, input ReferenceConnectionRequest, target [32]byte, alphaHostname string) (*ReferenceConnection, error) {
	if endpoint == nil || ctx == nil || target == [32]byte{} || input.Connection.Application != nil || input.Connection.OnAuthenticated != nil {
		return nil, errors.New("reference Connection input is incomplete or attempts to supply an Application path")
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
	running := &ReferenceConnection{cancel: cancel, ready: make(chan ReferenceReady, 1), done: make(chan ReferenceOutcome, 1), closed: make(chan struct{})}
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
		readyURL, proxyURL := server.URL(), ""
		var release func()
		if alphaHostname != "" {
			proxyURL, release, openErr = endpoint.openAlphaBrowserRoute(alphaHostname, server)
			if openErr != nil {
				_ = server.Close()
				return openErr
			}
			readyURL = "http://" + alphaHostname + "/"
		}
		running.mu.Lock()
		running.presentation = server
		running.release = release
		running.mu.Unlock()
		ready := ReferenceReady{URL: readyURL, AlphaProxyURL: proxyURL, AuthenticatedTarget: authenticated}
		if input.Browser != nil {
			if openErr := input.Browser.OpenReference(lifetime, ready); openErr != nil {
				return errors.Join(errors.New("selected Reference browser did not open the authenticated origin"), openErr)
			}
		}
		running.publishReady(ready)
		return nil
	}
	go func() {
		result, runErr := endpoint.Connect(lifetime, request)
		_ = application.Close()
		running.closePresentation()
		running.closeReady()
		running.done <- ReferenceOutcome{Result: result, Err: runErr}
		close(running.done)
		close(running.closed)
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

// terminated is an internal broadcast signal for Endpoint-owned cleanup. It
// intentionally does not consume the caller's classified Done outcome.
func (connection *ReferenceConnection) terminated() <-chan struct{} {
	if connection == nil {
		return nil
	}
	return connection.closed
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
	presentation := connection.presentation
	release := connection.release
	connection.presentation = nil
	connection.release = nil
	connection.mu.Unlock()
	if release != nil {
		release()
	}
	if presentation != nil {
		_ = presentation.Close()
	}
}

func alphaBrowserHostname(binding alpha.Binding) string {
	return string(binding.Link().Name()) + ".ard"
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
