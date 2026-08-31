package browserreference

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/browser/entry"
)

// AlphaProxy owns one loopback-only HTTP proxy for a finite set of active
// alpha browser names. It forwards a request only to the exact local Reference
// origin registered after Endpoint authentication. It does not resolve names,
// tunnel HTTPS, use an upstream proxy, or forward arbitrary Internet traffic.
type AlphaProxy struct {
	listener net.Listener
	server   *http.Server

	mu          sync.Mutex
	routes      map[string]alphaRouteOrigin
	opening     map[string]*alphaRouteOpening
	routeOpener AlphaRouteOpener
	work        sync.WaitGroup
	once        sync.Once
	closed      bool
	err         error

	browserEntryProbeCapability [32]byte
	browserEntryProxyCredential [32]byte
}

// AlphaRouteOpener opens one exact `.ard` origin after AlphaProxy has
// authenticated a browser request for that host. The opener must register its
// resulting origin on this proxy before it returns. It receives no ordinary
// Internet hostname because AlphaProxy validates the suffix first.
type AlphaRouteOpener func(context.Context, string) error

type alphaRouteOpening struct {
	done chan struct{}
	err  error
}

// AlphaRoute is one active exact-name route in an AlphaProxy. Closing it
// withdraws only that name; a later connection must explicitly register again.
type AlphaRoute struct {
	proxy  *AlphaProxy
	host   string
	origin alphaRouteOrigin
	once   sync.Once
}

// OpenAlphaProxy binds a fresh loopback-only alpha browser proxy. It initially
// has no route, so every browser name fails locally until Endpoint registers an
// authenticated Reference origin.
func OpenAlphaProxy() (*AlphaProxy, error) {
	return openAlphaProxy([32]byte{}, [32]byte{})
}

// OpenAlphaProxyForBrowserEntry binds one proxy that accepts a native-host
// liveness probe carrying the supplied Endpoint-local capability and requires
// the separate supplied credential for normal proxy requests. Normal browser
// requests still receive only exact registered alpha names.
func OpenAlphaProxyForBrowserEntry(probeCapability, proxyCredential [32]byte) (*AlphaProxy, error) {
	if probeCapability == [32]byte{} || proxyCredential == [32]byte{} {
		return nil, errors.New("alpha browser Entry credentials are invalid")
	}
	return openAlphaProxy(probeCapability, proxyCredential)
}

func openAlphaProxy(probeCapability, proxyCredential [32]byte) (*AlphaProxy, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	proxy := &AlphaProxy{listener: listener, routes: make(map[string]alphaRouteOrigin), opening: make(map[string]*alphaRouteOpening), browserEntryProbeCapability: probeCapability, browserEntryProxyCredential: proxyCredential}
	proxy.server = &http.Server{Handler: http.HandlerFunc(proxy.serve), ReadHeaderTimeout: time.Second,
		IdleTimeout: 5 * time.Second, MaxHeaderBytes: 16 << 10}
	proxy.work.Add(1)
	go func() {
		defer proxy.work.Done()
		_ = proxy.server.Serve(listener)
	}()
	return proxy, nil
}

// SetRouteOpener enables demand-opened exact alpha routes. It is a one-time
// Endpoint composition decision: callers cannot replace an opener while a
// browser request may be using it.
func (proxy *AlphaProxy) SetRouteOpener(opener AlphaRouteOpener) error {
	if proxy == nil || opener == nil {
		return errors.New("alpha browser route opener is invalid")
	}
	proxy.mu.Lock()
	defer proxy.mu.Unlock()
	if proxy.closed || proxy.routeOpener != nil {
		return errors.New("alpha browser route opener is unavailable")
	}
	proxy.routeOpener = opener
	return nil
}

// URL returns the loopback proxy URL for the local native Browser Entry. It is
// not a participant-facing Service URL and has no route until Register succeeds.
func (proxy *AlphaProxy) URL() string {
	if proxy == nil || proxy.listener == nil {
		return ""
	}
	return "http://" + proxy.listener.Addr().String()
}

// BrowserEntryPort is the exact unprivileged loopback port that a verified
// native host may hand to Firefox. It is not a participant-visible address.
func (proxy *AlphaProxy) BrowserEntryPort() (uint16, error) {
	if proxy == nil || proxy.listener == nil {
		return 0, errors.New("alpha browser proxy is unavailable")
	}
	tcp, ok := proxy.listener.Addr().(*net.TCPAddr)
	if !ok || !tcp.IP.Equal(net.IPv4(127, 0, 0, 1)) || tcp.Port < 1024 || tcp.Port > 65535 {
		return 0, errors.New("alpha browser proxy port is invalid")
	}
	return uint16(tcp.Port), nil
}

// Register adds one exact visible alpha HTTP name for a fresh authenticated
// Reference origin. It never replaces an existing mapping, even when both
// callers present the same local origin.
func (proxy *AlphaProxy) Register(hostname string, origin *Server) (*AlphaRoute, error) {
	return proxy.register(hostname, origin)
}

// RegisterTransparent adds one exact alpha HTTP name for the selected dynamic
// Service bridge. It does not grant a browser any other destination.
func (proxy *AlphaProxy) RegisterTransparent(hostname string, origin *TransparentServer) (*AlphaRoute, error) {
	return proxy.register(hostname, origin)
}

type alphaRouteOrigin interface {
	alphaOriginAddress() string
	alphaOriginHost(string) string
	alphaOriginPath(string) string
}

func (proxy *AlphaProxy) register(hostname string, origin alphaRouteOrigin) (*AlphaRoute, error) {
	if proxy == nil || origin == nil || origin.alphaOriginAddress() == "" || origin.alphaOriginHost(hostname) == "" || !validAlphaHTTPHost(hostname) {
		return nil, errors.New("alpha browser route is invalid")
	}
	proxy.mu.Lock()
	defer proxy.mu.Unlock()
	if proxy.closed || proxy.routes[hostname] != nil {
		return nil, errors.New("alpha browser route is unavailable")
	}
	proxy.routes[hostname] = origin
	return &AlphaRoute{proxy: proxy, host: hostname, origin: origin}, nil
}

// Close stops the loopback proxy and withdraws every alpha browser name.
func (proxy *AlphaProxy) Close() error {
	if proxy == nil || proxy.server == nil {
		return nil
	}
	proxy.once.Do(func() {
		proxy.mu.Lock()
		proxy.closed = true
		clear(proxy.routes)
		proxy.mu.Unlock()
		shutdown, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := proxy.server.Shutdown(shutdown); err != nil {
			closeErr := proxy.server.Close()
			if closeErr != nil && !errors.Is(closeErr, http.ErrServerClosed) {
				proxy.err = errors.Join(err, closeErr)
			} else {
				proxy.err = err
			}
		}
		proxy.work.Wait()
	})
	return proxy.err
}

// Close withdraws this exact alpha name. It is safe to call repeatedly.
func (route *AlphaRoute) Close() error {
	if route == nil || route.proxy == nil {
		return nil
	}
	route.once.Do(func() {
		route.proxy.mu.Lock()
		if route.proxy.routes[route.host] == route.origin {
			delete(route.proxy.routes, route.host)
		}
		route.proxy.mu.Unlock()
	})
	return nil
}

func (proxy *AlphaProxy) serve(writer http.ResponseWriter, request *http.Request) {
	if proxy.serveBrowserEntryProbe(writer, request) {
		return
	}
	if !proxy.authorizesBrowserEntryRequest(request) {
		writer.Header().Set("Proxy-Authenticate", `Basic realm="Ardents Browser Entry"`)
		writer.WriteHeader(http.StatusProxyAuthRequired)
		return
	}
	if request.Method == http.MethodConnect {
		http.Error(writer, "alpha HTTPS is unavailable", http.StatusBadGateway)
		return
	}
	if request.Header.Get("Upgrade") != "" {
		http.Error(writer, "alpha HTTP upgrade is unavailable", http.StatusBadRequest)
		return
	}
	if request.URL.Scheme != "http" || request.URL.User != nil || request.URL.Host == "" || request.URL.Port() != "" ||
		!validAlphaHTTPHost(request.URL.Hostname()) || request.Host != request.URL.Hostname() {
		http.Error(writer, "invalid alpha browser request", http.StatusBadRequest)
		return
	}
	origin, err := proxy.routeFor(request.Context(), request.URL.Hostname())
	if err != nil {
		http.Error(writer, "alpha name is unavailable", alphaRouteFailureStatus(err))
		return
	}
	response, closeInbound, err := forwardAlphaRequest(request, origin)
	if err != nil {
		if closeInbound {
			writer.Header().Set("Connection", "close")
		}
		http.Error(writer, "alpha service unavailable", http.StatusBadGateway)
		return
	}
	defer response.Body.Close()
	copyForwardHeaders(writer.Header(), response.Header)
	if closeInbound {
		writer.Header().Set("Connection", "close")
	}
	writer.WriteHeader(response.StatusCode)
	_ = copyAndFlush(writer, response.Body)
}

// routeFor returns a registered exact origin or permits one serialized
// Endpoint-owned attempt to open it. Concurrent browser subrequests for one
// name wait for that same attempt; none of them can obtain a second Target.
func (proxy *AlphaProxy) routeFor(ctx context.Context, hostname string) (alphaRouteOrigin, error) {
	if proxy == nil || ctx == nil {
		return nil, errors.New("alpha browser route is unavailable")
	}
	proxy.mu.Lock()
	if proxy.closed {
		proxy.mu.Unlock()
		return nil, errors.New("alpha browser route is unavailable")
	}
	if origin := proxy.routes[hostname]; origin != nil {
		proxy.mu.Unlock()
		return origin, nil
	}
	opener := proxy.routeOpener
	if opener == nil {
		proxy.mu.Unlock()
		return nil, errors.New("alpha browser name is unregistered")
	}
	if pending := proxy.opening[hostname]; pending != nil {
		proxy.mu.Unlock()
		select {
		case <-pending.done:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		if pending.err != nil {
			return nil, pending.err
		}
		proxy.mu.Lock()
		defer proxy.mu.Unlock()
		if proxy.closed || proxy.routes[hostname] == nil {
			return nil, errors.New("alpha browser name is unavailable")
		}
		return proxy.routes[hostname], nil
	}
	pending := &alphaRouteOpening{done: make(chan struct{})}
	proxy.opening[hostname] = pending
	proxy.mu.Unlock()

	err := opener(ctx, hostname)
	proxy.mu.Lock()
	delete(proxy.opening, hostname)
	pending.err = err
	close(pending.done)
	origin := proxy.routes[hostname]
	closed := proxy.closed
	proxy.mu.Unlock()
	if err != nil {
		return nil, err
	}
	if closed || origin == nil {
		return nil, errors.New("alpha browser name is unavailable")
	}
	return origin, nil
}

// alphaRouteFailureStatus accepts only the Endpoint-owned failure projection.
// AlphaProxy remains unaware of corpus, resolver, and Target semantics.
func alphaRouteFailureStatus(err error) int {
	var classified interface{ AlphaRouteHTTPStatus() int }
	if errors.As(err, &classified) {
		if status := classified.AlphaRouteHTTPStatus(); status >= 400 && status <= 599 {
			return status
		}
	}
	return http.StatusNotFound
}

func (proxy *AlphaProxy) serveBrowserEntryProbe(writer http.ResponseWriter, request *http.Request) bool {
	if request.URL.Path != browserentry.ProbePath || request.URL.RawQuery != "" || request.URL.IsAbs() {
		return false
	}
	if proxy.browserEntryProbeCapability == [32]byte{} {
		http.Error(writer, "browser Entry probe is unavailable", http.StatusNotFound)
		return true
	}
	host, port, err := net.SplitHostPort(request.Host)
	proxyPort, portErr := proxy.BrowserEntryPort()
	if request.Method != http.MethodGet || err != nil || host != "127.0.0.1" || portErr != nil || port != fmt.Sprintf("%d", proxyPort) ||
		request.Header.Get(browserentry.ProbeHeader) != hex.EncodeToString(proxy.browserEntryProbeCapability[:]) {
		http.Error(writer, "browser Entry probe is invalid", http.StatusBadRequest)
		return true
	}
	// Echo the capability only after this exact proxy has authenticated the
	// request. The native host requires this response proof so a stale state
	// file cannot mistake an unrelated recycled loopback port for this proxy.
	writer.Header().Set(browserentry.ProbeHeader, hex.EncodeToString(proxy.browserEntryProbeCapability[:]))
	writer.WriteHeader(http.StatusNoContent)
	return true
}

func (proxy *AlphaProxy) authorizesBrowserEntryRequest(request *http.Request) bool {
	if proxy.browserEntryProxyCredential == [32]byte{} {
		return true
	}
	expected := "Basic " + base64.StdEncoding.EncodeToString([]byte(browserentry.ProxyUsername+":"+hex.EncodeToString(proxy.browserEntryProxyCredential[:])))
	return request.Header.Get("Proxy-Authorization") == expected
}

func (server *Server) alphaOriginAddress() string {
	if server == nil || server.listener == nil {
		return ""
	}
	return server.listener.Addr().String()
}

func (server *Server) alphaOriginHost(string) string {
	if server == nil {
		return ""
	}
	return server.originHost
}

func (server *Server) alphaOriginPath(path string) string {
	if server == nil {
		return ""
	}
	return server.basePath + strings.TrimPrefix(path, "/")
}

func cloneForwardHeaders(source http.Header) http.Header {
	copy := source.Clone()
	removeForwardHopHeaders(copy)
	return copy
}

func copyForwardHeaders(destination, source http.Header) {
	copy := source.Clone()
	removeForwardHopHeaders(copy)
	for key, values := range copy {
		destination[key] = append([]string(nil), values...)
	}
}

func removeForwardHopHeaders(headers http.Header) {
	for _, value := range headers.Values("Connection") {
		for _, token := range strings.Split(value, ",") {
			headers.Del(strings.TrimSpace(token))
		}
	}
	for _, name := range []string{"Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "Proxy-Connection", "TE", "Trailer", "Transfer-Encoding", "Upgrade"} {
		headers.Del(name)
	}
}

func validAlphaHTTPHost(hostname string) bool {
	const suffix = ".ard"
	return strings.HasSuffix(hostname, suffix) && len(hostname) > len(suffix) && validReferenceName(strings.TrimSuffix(hostname, suffix))
}

func validReferenceName(value string) bool {
	if len(value) > 249 || strings.HasPrefix(value, ".") || strings.Contains(value, "..") {
		return false
	}
	for _, label := range strings.Split(value, ".") {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
		for _, character := range label {
			if !((character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '-') {
				return false
			}
		}
	}
	return true
}
