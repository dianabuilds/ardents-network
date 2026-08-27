package private

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/url"
)

// NewRelay creates an alpha private Relay that forwards only one opaque OHTTP
// envelope to its configured HTTPS Gateway.
func NewRelay(gatewayURL string, client *http.Client) (*relay, error) {
	parsed, err := url.Parse(gatewayURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || client == nil {
		return nil, errors.New("alpha private Relay requires one HTTPS Gateway")
	}
	return &relay{gateway: gatewayURL, client: client}, nil
}

// Handler returns the Relay's opaque forwarding adapter.
func (relay *relay) Handler() http.Handler { return http.HandlerFunc(relay.forward) }

func (relay *relay) forward(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost || request.URL.Path != "/ohttp" || request.Header.Get("Content-Type") != requestMediaType {
		http.Error(writer, "invalid opaque alpha request", http.StatusBadRequest)
		return
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, maxEnvelopeSize+1))
	if err != nil || len(body) == 0 || len(body) > maxEnvelopeSize {
		http.Error(writer, "invalid opaque alpha request", http.StatusBadRequest)
		return
	}
	forward, err := http.NewRequestWithContext(request.Context(), http.MethodPost, relay.gateway, bytes.NewReader(body))
	if err != nil {
		http.Error(writer, "alpha Gateway unavailable", http.StatusBadGateway)
		return
	}
	forward.Header.Set("Content-Type", requestMediaType)
	response, err := relay.client.Do(forward)
	if err != nil {
		http.Error(writer, "alpha Gateway unavailable", http.StatusBadGateway)
		return
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxEnvelopeSize+1))
	if err != nil || len(responseBody) == 0 || len(responseBody) > maxEnvelopeSize {
		http.Error(writer, "alpha Gateway response invalid", http.StatusBadGateway)
		return
	}
	writer.Header().Set("Content-Type", response.Header.Get("Content-Type"))
	writer.WriteHeader(response.StatusCode)
	_, _ = writer.Write(responseBody)
}
