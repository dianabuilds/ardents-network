//go:build browsercompat

package endpoint

import (
	"errors"

	"github.com/dianabuilds/ardents-network/internal/endpoint/reference"
)

// openAlphaBrowserRoute registers one exact authenticated alpha name with the
// Endpoint-owned shared loopback proxy. The returned release withdraws exactly
// that mapping and stops the proxy once it has no remaining active names.
func (endpoint *endpoint) openAlphaBrowserRoute(hostname string, origin *reference.Server) (string, func(), error) {
	if origin == nil {
		return "", nil, errors.New("alpha browser route is unavailable")
	}
	return endpoint.registerAlphaBrowserRoute(func(proxy *reference.AlphaProxy) (*reference.AlphaRoute, error) {
		return proxy.Register(hostname, origin)
	})
}

// openAlphaTransparentBrowserRoute registers one exact authenticated alpha
// name with a selected payload-neutral HTTP bridge.
func (endpoint *endpoint) openAlphaTransparentBrowserRoute(hostname string, origin *reference.TransparentServer) (string, func(), error) {
	if origin == nil {
		return "", nil, errors.New("alpha browser route is unavailable")
	}
	return endpoint.registerAlphaBrowserRoute(func(proxy *reference.AlphaProxy) (*reference.AlphaRoute, error) {
		return proxy.RegisterTransparent(hostname, origin)
	})
}

func (endpoint *endpoint) registerAlphaBrowserRoute(register func(*reference.AlphaProxy) (*reference.AlphaRoute, error)) (string, func(), error) {
	if endpoint == nil || register == nil {
		return "", nil, errors.New("alpha browser route is unavailable")
	}
	endpoint.alphaBrowserMu.Lock()
	proxy, err := endpoint.ensureAlphaBrowserProxyLocked()
	if err != nil {
		endpoint.alphaBrowserMu.Unlock()
		return "", nil, err
	}
	route, err := register(proxy)
	if err != nil {
		if endpoint.alphaBrowserRoutes == 0 && endpoint.alphaBrowserOwners == 0 && endpoint.alphaBrowserProxy == proxy {
			endpoint.alphaBrowserProxy = nil
			endpoint.alphaBrowserMu.Unlock()
			_ = proxy.Close()
			return "", nil, err
		}
		endpoint.alphaBrowserMu.Unlock()
		return "", nil, err
	}
	endpoint.alphaBrowserRoutes++
	if publishErr := endpoint.publishAlphaBrowserProxyLocked(proxy); publishErr != nil {
		endpoint.alphaBrowserRoutes--
		_ = route.Close()
		var closeProxy *reference.AlphaProxy
		if endpoint.alphaBrowserRoutes == 0 && endpoint.alphaBrowserOwners == 0 && endpoint.alphaBrowserProxy == proxy {
			closeProxy = proxy
			endpoint.alphaBrowserProxy = nil
		}
		endpoint.alphaBrowserMu.Unlock()
		if closeProxy != nil {
			_ = closeProxy.Close()
		}
		return "", nil, publishErr
	}
	proxyURL := proxy.URL()
	endpoint.alphaBrowserMu.Unlock()
	return proxyURL, func() { endpoint.closeAlphaBrowserRoute(route) }, nil
}

func (endpoint *endpoint) closeAlphaBrowserRoute(route *reference.AlphaRoute) {
	if endpoint == nil || route == nil {
		return
	}
	_ = route.Close()
	endpoint.alphaBrowserMu.Lock()
	if endpoint.alphaBrowserRoutes == 0 {
		endpoint.alphaBrowserMu.Unlock()
		return
	}
	endpoint.alphaBrowserRoutes--
	var closeProxy *reference.AlphaProxy
	if endpoint.alphaBrowserRoutes == 0 && endpoint.alphaBrowserOwners == 0 {
		if endpoint.browserEntry != nil {
			_ = endpoint.browserEntry.Clear()
		}
		closeProxy = endpoint.alphaBrowserProxy
		endpoint.alphaBrowserProxy = nil
	}
	endpoint.alphaBrowserMu.Unlock()
	if closeProxy != nil {
		_ = closeProxy.Close()
	}
}

func (endpoint *endpoint) closeAlphaBrowserProxy() {
	if endpoint == nil {
		return
	}
	endpoint.alphaBrowserMu.Lock()
	proxy := endpoint.alphaBrowserProxy
	endpoint.alphaBrowserProxy = nil
	endpoint.alphaBrowserRoutes = 0
	endpoint.alphaBrowserOwners = 0
	if endpoint.browserEntry != nil {
		_ = endpoint.browserEntry.Clear()
	}
	endpoint.alphaBrowserMu.Unlock()
	if proxy != nil {
		_ = proxy.Close()
	}
}

// ensureAlphaBrowserProxyLocked returns the one Endpoint-owned loopback proxy.
// Its caller holds alphaBrowserMu and must publish Browser Entry state only
// after it has installed any demand-open resolver or exact route.
func (endpoint *endpoint) ensureAlphaBrowserProxyLocked() (*reference.AlphaProxy, error) {
	if endpoint.alphaBrowserProxy != nil {
		return endpoint.alphaBrowserProxy, nil
	}
	var (
		proxy *reference.AlphaProxy
		err   error
	)
	if endpoint.browserEntry != nil {
		proxy, err = reference.OpenAlphaProxyForBrowserEntry(endpoint.browserEntry.Capability(), endpoint.browserEntry.ProxyCredential())
	} else {
		proxy, err = reference.OpenAlphaProxy()
	}
	if err != nil {
		return nil, err
	}
	endpoint.alphaBrowserProxy = proxy
	return proxy, nil
}

// publishAlphaBrowserProxyLocked exposes the active proxy only to the fixed
// Browser Entry state owner. It receives no Service Name or Target.
func (endpoint *endpoint) publishAlphaBrowserProxyLocked(proxy *reference.AlphaProxy) error {
	if endpoint.browserEntry == nil {
		return nil
	}
	port, err := proxy.BrowserEntryPort()
	if err != nil {
		return err
	}
	return endpoint.browserEntry.Publish(port)
}
