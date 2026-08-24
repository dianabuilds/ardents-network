package reachability

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/openpcc/ohttp"
)

// ClientConfig is one already-selected Relay/Gateway pair. Production callers
// must derive its public fields from State; this adapter does not discover one.
type ClientConfig struct {
	NetworkID     [32]byte
	GatewayPublic [32]byte
	Profile       GatewayProfile
	RelayURL      string
	BaseTransport *http.Transport
	At            time.Time
	Deadline      time.Time
}

// Client performs one single-use private Target lookup over its selected Relay.
type Client struct {
	network, gateway [32]byte
	at, deadline     time.Time
	client           *http.Client
	transport        *ohttp.Transport

	mu   sync.Mutex
	used bool
}

// OpenClient authenticates the Gateway profile and constructs a one-use OHTTP
// client. It has no HTTP/plaintext fallback.
func OpenClient(config ClientConfig) (*Client, error) {
	parsed, err := url.Parse(config.RelayURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || config.NetworkID == [32]byte{} ||
		config.GatewayPublic == [32]byte{} || config.BaseTransport == nil || config.At.IsZero() ||
		!config.At.Before(config.Deadline) || config.Deadline.After(config.At.Add(15*time.Second)) ||
		config.Profile.NetworkID != config.NetworkID || config.Profile.AssignmentNotAfter.Before(config.Deadline) ||
		!validGatewayProfile(config.Profile, config.GatewayPublic) {
		return nil, errors.New("private reachability client configuration is invalid")
	}
	var key ohttp.KeyConfig
	if err := key.UnmarshalBinary(config.Profile.KeyConfig); err != nil {
		return nil, errors.New("private reachability Gateway key configuration is invalid")
	}
	client := isolatedPrivateClient(config.BaseTransport)
	transport, err := ohttp.NewTransport(key, config.RelayURL, ohttp.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}
	return &Client{network: config.NetworkID, gateway: config.Profile.NodeID, at: config.At, deadline: config.Deadline,
		client: client, transport: transport}, nil
}

// Resolve returns exactly one verified descriptor payload or a closed class.
// Descriptor signature verification remains Endpoint-owned because only it
// binds the final Target Link to C-2 Route composition.
func (client *Client) Resolve(ctx context.Context, target [32]byte) ([]byte, StoreClass, error) {
	if client == nil || ctx == nil || target == [32]byte{} || !client.begin() {
		return nil, StoreInvalid, errors.New("private reachability client is unavailable")
	}
	attempt, cancel := context.WithDeadline(ctx, client.deadline)
	defer cancel()
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, StoreInvalid, err
	}
	payload, err := encodePrivateRequest(privateRequest{network: client.network, target: target, nonce: nonce, deadline: client.deadline.UnixNano()})
	if err == nil {
		payload, err = padPrivateMessage(payload)
	}
	if err != nil {
		return nil, StoreInvalid, err
	}
	request, err := http.NewRequestWithContext(attempt, http.MethodPost, "http://ohttp.invalid/resolve", bytes.NewReader(payload))
	if err != nil {
		return nil, StoreInvalid, err
	}
	encapsulated, decapsulator, err := client.transport.Encapsulate(request)
	if err != nil {
		return nil, StoreInvalid, err
	}
	response, err := client.client.Do(encapsulated)
	defer client.client.CloseIdleConnections()
	if err != nil {
		return nil, StoreStale, errors.New("private reachability unavailable")
	}
	defer response.Body.Close()
	plain, err := decapsulator.Decapsulate(attempt, response)
	if err != nil {
		return nil, StoreInvalid, errors.New("private reachability evidence is invalid")
	}
	defer plain.Body.Close()
	fixed, err := io.ReadAll(io.LimitReader(plain.Body, privateMessageSize+1))
	if err != nil {
		return nil, StoreInvalid, errors.New("private reachability evidence is invalid")
	}
	payload, err = unpadPrivateMessage(fixed)
	if err != nil {
		return nil, StoreInvalid, errors.New("private reachability evidence is invalid")
	}
	decoded, err := decodePrivateResponse(payload)
	if err != nil || decoded.network != client.network || decoded.target != target || decoded.nonce != nonce || decoded.deadline != client.deadline.UnixNano() {
		return nil, StoreInvalid, errors.New("private reachability response binding is invalid")
	}
	switch decoded.class {
	case privateResolved:
		return append([]byte(nil), decoded.descriptor...), StoreAlreadyCurrent, nil
	case privateConflicting:
		return nil, StoreConflicting, errors.New("private reachability Target conflicts")
	case privateUnavailable:
		return nil, StoreStale, errors.New("private reachability Target is unavailable")
	default:
		return nil, StoreInvalid, errors.New("private reachability evidence is invalid")
	}
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

func isolatedPrivateClient(base *http.Transport) *http.Client {
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
	return &http.Client{Transport: transport}
}
