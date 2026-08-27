package credential

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"net/url"

	"github.com/openpcc/ohttp"
)

// HTTPClient creates the one-purpose HTTPS client for a State-selected issuer
// duty. The caller supplies its identity from State; ambient roots, proxy,
// compression, keep-alives, and HTTP/2 are unavailable.
func HTTPClient(expected [32]byte, certificate tls.Certificate) (*http.Client, error) {
	if expected == [32]byte{} || certificate.PrivateKey == nil || certificate.Leaf == nil {
		return nil, errors.New("transit issuance issuer key is missing")
	}
	transport := &http.Transport{Proxy: nil, DisableCompression: true, DisableKeepAlives: true, ForceAttemptHTTP2: false,
		MaxConnsPerHost: 1, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, Certificates: []tls.Certificate{certificate},
			InsecureSkipVerify: true, SessionTicketsDisabled: true, NextProtos: []string{"http/1.1"}, VerifyConnection: func(state tls.ConnectionState) error {
				if len(state.PeerCertificates) != 1 {
					return errors.New("transit issuance issuer certificate is unavailable")
				}
				public, ok := state.PeerCertificates[0].PublicKey.(ed25519.PublicKey)
				if !ok || len(public) != len(expected) || string(public) != string(expected[:]) {
					return errors.New("transit issuance issuer certificate does not match State")
				}
				return nil
			}}}
	return &http.Client{Transport: transport}, nil
}

// ForwardOHTTP exchanges one opaque envelope with one State-selected issuer.
// It accepts no caller headers, method, target, stream, retry, or fallback.
func ForwardOHTTP(ctx context.Context, issuerURL string, client *http.Client, envelope []byte) ([]byte, error) {
	parsed, err := url.Parse(issuerURL)
	if ctx == nil || err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.Path != "" || parsed.User != nil || parsed.RawQuery != "" ||
		parsed.Fragment != "" || client == nil || len(envelope) == 0 || len(envelope) > maximumEnvelopeSize {
		return nil, errors.New("transit issuance OHTTP forward is invalid")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, issuerURL, bytes.NewReader(envelope))
	if err != nil {
		return nil, errors.New("transit issuance OHTTP forward is invalid")
	}
	request.Header.Set("Content-Type", ohttp.RequestMediaType)
	response, err := client.Do(request)
	if err != nil {
		return nil, errors.New("transit issuance issuer is unavailable")
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maximumEnvelopeSize+1))
	if err != nil || response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != ohttp.ResponseMediaType ||
		len(body) == 0 || len(body) > maximumEnvelopeSize {
		return nil, errors.New("transit issuance issuer response is invalid")
	}
	return body, nil
}
