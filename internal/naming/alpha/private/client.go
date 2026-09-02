package private

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
	"github.com/openpcc/ohttp"
)

// Open verifies one alpha private-resolution role configuration and creates a
// single-use OHTTP Client. Retrying creates a fresh Client and OHTTP context.
func Open(config ClientConfig, now time.Time) (*Client, error) {
	parsed, err := url.Parse(config.RelayURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || config.RelayNodeID == [32]byte{} ||
		config.RelayFamily == "" || config.Gateway.NodeID == [32]byte{} || config.Gateway.Family == "" ||
		config.Gateway.NodeID == config.RelayNodeID || config.Gateway.Family == config.RelayFamily ||
		config.AuthorityPublic == nil || config.Cohort == "" || config.Network == [32]byte{} || config.Floor == nil ||
		config.Base == nil || now.IsZero() || !now.Before(config.Gateway.AssignmentNotAfter) ||
		config.Gateway.Cohort != config.Cohort || config.Gateway.NetworkID != config.Network ||
		!validProfile(config.Gateway, config.GatewayPublic) {
		return nil, errors.New("alpha private Client configuration is invalid")
	}
	keyConfig := ohttp.KeyConfig{}
	if err := keyConfig.UnmarshalBinary(config.Gateway.KeyConfig); err != nil {
		return nil, errors.New("alpha private Gateway key configuration is invalid")
	}
	httpClient := isolatedClient(config.Base)
	transport, err := ohttp.NewTransport(keyConfig, config.RelayURL+"/ohttp", ohttp.WithHTTPClient(httpClient))
	if err != nil {
		return nil, err
	}
	return &Client{config: config, transport: transport, http: httpClient}, nil
}

// Resolve sends exactly one padded OHTTP alpha-name request through its Relay,
// verifies the response Corpus under the pinned alpha authority, checks its
// decision-time validity before applying its supplied floor, and returns only
// the exact requested Binding.
func (client *Client) Resolve(ctx context.Context, link alpha.ServiceLink, at time.Time) (alpha.Binding, error) {
	if client == nil || ctx == nil || at.IsZero() || !client.begin() || !at.Before(client.config.Gateway.AssignmentNotAfter) {
		return alpha.Binding{}, errors.New("alpha private resolution input is invalid")
	}
	deadline := at.Add(15 * time.Second)
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return alpha.Binding{}, err
	}
	payload, err := encodeRequest(request{nonce: nonce, deadline: deadline.UnixNano(), link: link})
	if err == nil {
		payload, err = pad(payload)
	}
	if err != nil {
		return alpha.Binding{}, err
	}
	attempt, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	plainRequest, err := http.NewRequestWithContext(attempt, http.MethodPost, "http://ohttp.invalid/resolve", bytes.NewReader(payload))
	if err != nil {
		return alpha.Binding{}, err
	}
	encapsulated, decapsulator, err := client.transport.Encapsulate(plainRequest)
	if err != nil {
		return alpha.Binding{}, err
	}
	outerResponse, err := client.http.Do(encapsulated)
	client.http.CloseIdleConnections()
	if err != nil {
		return alpha.Binding{}, err
	}
	defer outerResponse.Body.Close()
	plainResponse, err := decapsulator.Decapsulate(attempt, outerResponse)
	if err != nil {
		return alpha.Binding{}, err
	}
	defer plainResponse.Body.Close()
	fixed, err := io.ReadAll(io.LimitReader(plainResponse.Body, fixedMessageSize+1))
	if err != nil {
		return alpha.Binding{}, err
	}
	responsePayload, err := unpad(fixed)
	if err != nil {
		return alpha.Binding{}, err
	}
	response, err := decodeResponse(responsePayload)
	if err != nil || response.nonce != nonce || response.deadline != deadline.UnixNano() || response.link != link {
		return alpha.Binding{}, errors.New("alpha private response binding is invalid")
	}
	corpus, err := alpha.OpenCorpus(client.config.AuthorityPublic, response.corpus)
	if err != nil || corpus.Cohort() != client.config.Cohort || corpus.Network() != client.config.Network {
		return alpha.Binding{}, errors.New("alpha private response corpus is invalid")
	}
	if err := corpus.ValidAt(at); err != nil {
		return alpha.Binding{}, err
	}
	if err := client.config.Floor.Observe(corpus); err != nil {
		return alpha.Binding{}, err
	}
	return corpus.Resolve(link, at)
}

func (client *Client) begin() bool {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.used {
		return false
	}
	client.used = true
	return true
}

func isolatedClient(base *http.Transport) *http.Client {
	transport := base.Clone()
	transport.Proxy = nil
	transport.DisableCompression = true
	transport.DisableKeepAlives = true
	transport.ForceAttemptHTTP2 = false
	transport.MaxConnsPerHost = 1
	if transport.TLSClientConfig == nil {
		transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS13}
	} else {
		transport.TLSClientConfig = transport.TLSClientConfig.Clone()
		if transport.TLSClientConfig.MinVersion < tls.VersionTLS13 {
			transport.TLSClientConfig.MinVersion = tls.VersionTLS13
		}
	}
	transport.TLSClientConfig.NextProtos = []string{"http/1.1"}
	return &http.Client{Transport: transport, CheckRedirect: rejectAlphaPrivateRedirect}
}

func rejectAlphaPrivateRedirect(_ *http.Request, _ []*http.Request) error {
	return errors.New("alpha private resolution redirects are forbidden")
}
