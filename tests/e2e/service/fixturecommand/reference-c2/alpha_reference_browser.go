//go:build browsercompat

package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

type alphaReferenceTransport struct {
	*http.Transport
	dialer *alphaReferenceProxyDialer
}

type alphaReferenceProxyDialer struct {
	mu                           sync.Mutex
	address                      string
	acceptedDials, rejectedDials uint32
}

// alphaReferenceClient is fixture-only browser compatibility plumbing. It
// keeps the participant-visible alpha name intact while sending only that
// request through the Endpoint-created exact-name loopback proxy.
func alphaReferenceClient(referenceURL, proxyURL string) (*http.Client, error) {
	parsed, err := url.Parse(referenceURL)
	if err != nil || parsed.Scheme != "http" || parsed.Hostname() == "" || parsed.Port() != "" ||
		!strings.HasSuffix(parsed.Hostname(), ".ard") || parsed.EscapedPath() != "/" {
		return nil, errors.New("fixture alpha Reference origin is invalid")
	}
	proxy, proxyErr := url.Parse(proxyURL)
	if proxyErr != nil || proxy.Scheme != "http" || proxy.Hostname() != "127.0.0.1" || proxy.Port() == "" {
		return nil, errors.New("fixture alpha Reference proxy is invalid")
	}
	dialer := &alphaReferenceProxyDialer{address: proxy.Host}
	transport := &http.Transport{Proxy: http.ProxyURL(proxy), DialContext: dialer.dialContext}
	return &http.Client{Transport: &alphaReferenceTransport{Transport: transport, dialer: dialer}}, nil
}

func (dialer *alphaReferenceProxyDialer) dialContext(ctx context.Context, network, address string) (net.Conn, error) {
	dialer.mu.Lock()
	if address != dialer.address {
		dialer.mu.Unlock()
		return nil, errors.New("fixture alpha Reference client refused a non-proxy dial")
	}
	if dialer.acceptedDials != 0 {
		dialer.rejectedDials++
		dialer.mu.Unlock()
		return nil, errors.New("fixture alpha Reference client refused proxy reconnect")
	}
	dialer.acceptedDials++
	dialer.mu.Unlock()
	var networkDialer net.Dialer
	return networkDialer.DialContext(ctx, network, address)
}

func alphaReferenceProxyDialCounts(client *http.Client) (uint32, uint32) {
	transport, ok := client.Transport.(*alphaReferenceTransport)
	if !ok || transport.dialer == nil {
		return 0, 0
	}
	transport.dialer.mu.Lock()
	defer transport.dialer.mu.Unlock()
	return transport.dialer.acceptedDials, transport.dialer.rejectedDials
}
