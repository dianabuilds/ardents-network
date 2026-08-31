package browseradapter

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"

	applicationconnection "github.com/dianabuilds/ardents-network/internal/application/interfacev1/connection"
	"github.com/dianabuilds/ardents-network/internal/browser/entry"
	reference "github.com/dianabuilds/ardents-network/internal/browser/reference"
)

type applicationStream interface {
	io.ReadWriteCloser
	Done() <-chan applicationconnection.Outcome
}

type applicationDial func(context.Context, string, string) (applicationStream, error)

// Config contains only Adapter-owned local paths. ApplicationSocket names the
// Endpoint Connection Interface; BrowserEntryStatePath is the native host's
// replaceable presentation handoff.
type Config struct {
	ApplicationSocket     string
	BrowserEntryStatePath string
}

// Runtime owns one Browser Adapter process lifecycle independently of the
// Endpoint it calls.
type Runtime struct {
	ctx         context.Context
	cancel      context.CancelFunc
	application string
	entry       *browserentry.Publisher
	proxy       *reference.AlphaProxy
	dial        applicationDial

	mu     sync.Mutex
	sites  map[string]*site
	closed bool
	once   sync.Once
	err    error
}

type site struct {
	application  applicationStream
	presentation *reference.TransparentServer
	route        *reference.AlphaRoute
	once         sync.Once
	err          error
}

// Open starts one Browser Adapter without starting, stopping, or configuring
// an Endpoint or browser executable.
func Open(ctx context.Context, config Config) (*Runtime, error) {
	return open(ctx, config, func(dialCtx context.Context, path, serviceLink string) (applicationStream, error) {
		return applicationconnection.Dial(dialCtx, path, serviceLink)
	})
}

func open(ctx context.Context, config Config, dial applicationDial) (*Runtime, error) {
	if ctx == nil || config.ApplicationSocket == "" || config.BrowserEntryStatePath == "" || dial == nil {
		return nil, errors.New("browser Adapter configuration is incomplete")
	}
	entry, err := browserentry.OpenPublisher(config.BrowserEntryStatePath)
	if err != nil {
		return nil, err
	}
	proxy, err := reference.OpenAlphaProxyForBrowserEntry(entry.Capability(), entry.ProxyCredential())
	if err != nil {
		_ = entry.Close()
		return nil, err
	}
	lifetime, cancel := context.WithCancel(ctx)
	runtime := &Runtime{ctx: lifetime, cancel: cancel, application: config.ApplicationSocket, entry: entry, proxy: proxy,
		dial: dial, sites: make(map[string]*site)}
	if err := proxy.SetRouteOpener(runtime.openName); err != nil {
		_ = runtime.Close()
		return nil, err
	}
	port, err := proxy.BrowserEntryPort()
	if err != nil || entry.Publish(port) != nil {
		_ = runtime.Close()
		return nil, errors.New("browser Adapter Entry state is unavailable")
	}
	return runtime, nil
}

func (runtime *Runtime) openName(ctx context.Context, hostname string) error {
	if runtime == nil || ctx == nil || !strings.HasSuffix(hostname, ".ard") || len(hostname) <= len(".ard") {
		return errors.New("browser Adapter Service Name is invalid")
	}
	link, err := serviceLinkForHostname(hostname)
	if err != nil {
		return err
	}
	application, err := runtime.dial(ctx, runtime.application, link)
	if err != nil {
		return err
	}
	presentation, err := reference.OpenTransparent(reference.TransparentConfig{Hostname: hostname, Connection: application})
	if err != nil {
		_ = application.Close()
		return err
	}
	route, err := runtime.proxy.RegisterTransparent(hostname, presentation)
	if err != nil {
		_ = presentation.Close()
		return err
	}
	opened := &site{application: application, presentation: presentation, route: route}
	runtime.mu.Lock()
	if runtime.closed || runtime.sites[hostname] != nil {
		runtime.mu.Unlock()
		_ = opened.Close()
		return errors.New("browser Adapter Service presentation is unavailable")
	}
	runtime.sites[hostname] = opened
	runtime.mu.Unlock()
	go runtime.watch(hostname, opened)
	return nil
}

func (runtime *Runtime) watch(hostname string, opened *site) {
	if opened == nil {
		return
	}
	select {
	case <-opened.application.Done():
	case <-runtime.ctx.Done():
	}
	runtime.mu.Lock()
	if runtime.sites[hostname] == opened {
		delete(runtime.sites, hostname)
	}
	runtime.mu.Unlock()
	_ = opened.Close()
}

// Close withdraws Browser Entry state, local HTTP origins, and the proxy. It
// never closes or mutates the Endpoint process behind ApplicationSocket.
func (runtime *Runtime) Close() error {
	if runtime == nil {
		return nil
	}
	runtime.once.Do(func() {
		runtime.cancel()
		runtime.mu.Lock()
		runtime.closed = true
		sites := make([]*site, 0, len(runtime.sites))
		for _, opened := range runtime.sites {
			sites = append(sites, opened)
		}
		clear(runtime.sites)
		runtime.mu.Unlock()
		for _, opened := range sites {
			runtime.err = errors.Join(runtime.err, opened.Close())
		}
		runtime.err = errors.Join(runtime.err, runtime.entry.Clear(), runtime.proxy.Close(), runtime.entry.Close())
	})
	return runtime.err
}

func (opened *site) Close() error {
	if opened == nil {
		return nil
	}
	opened.once.Do(func() {
		opened.err = errors.Join(opened.route.Close(), opened.presentation.Close(), opened.application.Close())
	})
	return opened.err
}
