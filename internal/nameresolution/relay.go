package nameresolution

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/url"
)

const maxOHTTPEnvelope = 8 << 10

// NewRelay creates a header-stripping Relay bound to one HTTPS Gateway.
func NewRelay(gatewayURL string, client *http.Client) (*relay, error) {
	parsed, err := url.Parse(gatewayURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || client == nil {
		return nil, errors.New("private resolution Relay requires one HTTPS Gateway")
	}
	return &relay{gateway: gatewayURL, client: client}, nil
}

// Handler returns the endpoint-adjacent opaque forwarding Adapter.
func (relay *relay) Handler() http.Handler { return http.HandlerFunc(relay.forward) }

// Observation returns only bounded observer-safe Relay counters.
func (relay *relay) Observation() (requests uint32, requestBytes, responseBytes uint64) {
	relay.mu.Lock()
	defer relay.mu.Unlock()
	return relay.observation.Requests, relay.observation.RequestBytes, relay.observation.ResponseBytes
}

func (relay *relay) forward(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost || request.URL.Path != "/ohttp" || request.Header.Get("Content-Type") != ohttpRequestType {
		http.Error(writer, "invalid opaque request", http.StatusBadRequest)
		return
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, maxOHTTPEnvelope+1))
	if err != nil || len(body) == 0 || len(body) > maxOHTTPEnvelope {
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
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxOHTTPEnvelope+1))
	if err != nil || len(responseBody) == 0 || len(responseBody) > maxOHTTPEnvelope {
		http.Error(writer, "Gateway response invalid", http.StatusBadGateway)
		return
	}
	writer.Header().Set("Content-Type", response.Header.Get("Content-Type"))
	writer.WriteHeader(response.StatusCode)
	_, _ = writer.Write(responseBody)
	relay.mu.Lock()
	relay.observation.Requests++
	relay.observation.RequestBytes += uint64(len(body))
	relay.observation.ResponseBytes += uint64(len(responseBody))
	relay.mu.Unlock()
}
