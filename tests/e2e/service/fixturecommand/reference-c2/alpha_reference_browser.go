package main

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
)

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
	return &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxy)}}, nil
}
