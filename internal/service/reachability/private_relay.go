package reachability

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"

	"github.com/openpcc/ohttp"
)

const maximumOHTTPEnvelope = 8 << 10

// Relay is the endpoint-adjacent opaque OHTTP forwarder for one selected
// Gateway. It parses neither the Target nor a Reachability Descriptor.
type Relay struct {
	gateway string
	client  *http.Client
}

// OHTTPResponse is the bounded opaque Gateway result. Chunked selects one of
// the two RFC 9458 response framings; it is not an application-controlled HTTP
// header.
type OHTTPResponse struct {
	Envelope []byte
	Chunked  bool
}

// NewRelay creates a header-stripping Relay bound to one HTTPS Gateway URL.
func NewRelay(gatewayURL string, client *http.Client) (*Relay, error) {
	parsed, err := url.Parse(gatewayURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || client == nil {
		return nil, errors.New("private reachability Relay requires one HTTPS Gateway")
	}
	return &Relay{gateway: gatewayURL, client: client}, nil
}

// Handler returns the opaque forwarding endpoint.
func (relay *Relay) Handler() http.Handler { return http.HandlerFunc(relay.forward) }

func (relay *Relay) forward(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost || request.URL.Path != "/ohttp" || request.Header.Get("Content-Type") != ohttpRequestType {
		http.Error(writer, "invalid opaque request", http.StatusBadRequest)
		return
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, maximumOHTTPEnvelope+1))
	if err != nil || len(body) == 0 || len(body) > maximumOHTTPEnvelope {
		http.Error(writer, "invalid opaque request", http.StatusBadRequest)
		return
	}
	response, err := ForwardOHTTP(request.Context(), relay.gateway, relay.client, body)
	if err != nil {
		http.Error(writer, "Gateway response invalid", http.StatusBadGateway)
		return
	}
	contentType := ohttp.ResponseMediaType
	if response.Chunked {
		contentType = ohttp.ChunkedResponseMediaType
	}
	writer.Header().Set("Content-Type", contentType)
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(response.Envelope)
}

// ForwardOHTTP exchanges exactly one opaque OHTTP envelope with one selected
// Gateway. It accepts no caller headers, method, path, proxy target, or
// streaming body, so callers cannot turn it into a generic HTTP forwarder.
func ForwardOHTTP(ctx context.Context, gatewayURL string, client *http.Client, envelope []byte) (OHTTPResponse, error) {
	parsed, err := url.Parse(gatewayURL)
	if ctx == nil || err != nil || parsed.Scheme != "https" || parsed.Host == "" || client == nil ||
		len(envelope) == 0 || len(envelope) > maximumOHTTPEnvelope {
		return OHTTPResponse{}, errors.New("private reachability OHTTP forward is invalid")
	}
	forward, err := http.NewRequestWithContext(ctx, http.MethodPost, gatewayURL, bytes.NewReader(envelope))
	if err != nil {
		return OHTTPResponse{}, errors.New("private reachability OHTTP forward is invalid")
	}
	forward.Header.Set("Content-Type", ohttpRequestType)
	response, err := client.Do(forward)
	if err != nil {
		return OHTTPResponse{}, errors.New("private reachability Gateway is unavailable")
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maximumOHTTPEnvelope+1))
	contentType := response.Header.Get("Content-Type")
	if err != nil || response.StatusCode != http.StatusOK || (contentType != ohttp.ResponseMediaType && contentType != ohttp.ChunkedResponseMediaType) ||
		len(responseBody) == 0 || len(responseBody) > maximumOHTTPEnvelope {
		return OHTTPResponse{}, errors.New("private reachability Gateway response is invalid")
	}
	return OHTTPResponse{Envelope: responseBody, Chunked: contentType == ohttp.ChunkedResponseMediaType}, nil
}
