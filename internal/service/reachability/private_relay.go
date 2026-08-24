package reachability

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/url"
)

const maximumOHTTPEnvelope = 8 << 10

// Relay is the endpoint-adjacent opaque OHTTP forwarder for one selected
// Gateway. It parses neither the Target nor a Reachability Descriptor.
type Relay struct {
	gateway string
	client  *http.Client
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
	forward, err := http.NewRequestWithContext(request.Context(), http.MethodPost, relay.gateway, bytes.NewReader(body))
	if err != nil {
		http.Error(writer, "Gateway unavailable", http.StatusBadGateway)
		return
	}
	forward.Header.Set("Content-Type", ohttpRequestType)
	response, err := relay.client.Do(forward)
	if err != nil {
		http.Error(writer, "Gateway unavailable", http.StatusBadGateway)
		return
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maximumOHTTPEnvelope+1))
	if err != nil || len(responseBody) == 0 || len(responseBody) > maximumOHTTPEnvelope {
		http.Error(writer, "Gateway response invalid", http.StatusBadGateway)
		return
	}
	writer.Header().Set("Content-Type", response.Header.Get("Content-Type"))
	writer.WriteHeader(response.StatusCode)
	_, _ = writer.Write(responseBody)
}
