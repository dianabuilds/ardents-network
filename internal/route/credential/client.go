package credential

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/openpcc/ohttp"
)

// ErrExchangeUnavailable identifies an interruption before an authenticated
// issuer response exists. An Endpoint may reconcile only this class with the
// exact same durable Request ID and tuple.
var ErrExchangeUnavailable = errors.New("transit issuance exchange is unavailable")

// Client performs exactly one opaque membership-level Transit Grant exchange.
type Client struct {
	network, issuer [32]byte
	profile         Profile
	authority       ed25519.PublicKey
	at, deadline    time.Time
	exchange        Exchange
	transport       *ohttp.Transport

	mu   sync.Mutex
	used bool
}

// OpenClient verifies one State-selected issuer profile and creates a
// one-use OHTTP client. The caller supplies no HTTP URL or fallback.
func OpenClient(config ClientConfig) (*Client, error) {
	if config.NetworkID == [32]byte{} || config.IssuerPublic == [32]byte{} || config.Profile.Version != profileVersion ||
		config.Profile.GrantSignerPublicKey == [32]byte{} ||
		config.Exchange == nil || config.At.IsZero() || !config.At.Before(config.Deadline) || config.Deadline.After(config.At.Add(15*time.Second)) {
		return nil, errors.New("transit issuance client configuration is invalid")
	}
	if err := VerifyProfile(config.Profile, config.NetworkID, config.Profile.NodeID, config.IssuerPublic, config.At, config.Deadline); err != nil {
		return nil, errors.New("transit issuance client configuration is invalid")
	}
	if config.Profile.GrantSignerID != sha256.Sum256(config.Profile.GrantSignerPublicKey[:]) {
		return nil, errors.New("transit issuance authority does not match State profile")
	}
	var key ohttp.KeyConfig
	if err := key.UnmarshalBinary(config.Profile.KeyConfig); err != nil {
		return nil, errors.New("transit issuance OHTTP profile is invalid")
	}
	transport, err := ohttp.NewTransport(key, "https://transit-issuance.invalid/issue")
	if err != nil {
		return nil, err
	}
	return &Client{network: config.NetworkID, issuer: config.Profile.NodeID, profile: config.Profile,
		authority: append(ed25519.PublicKey(nil), config.Profile.GrantSignerPublicKey[:]...), at: config.At, deadline: config.Deadline,
		exchange: config.Exchange, transport: transport}, nil
}

// Issue returns one current, exact Grant. A malformed, withheld, or changed
// response fails locally and cannot become a retry or route fallback.
func (client *Client) Issue(ctx context.Context, input Request) (Result, error) {
	if client == nil || ctx == nil || input.NetworkID != client.network || input.NotAfter.After(client.deadline) || !client.begin() {
		return Result{}, errors.New("transit issuance client is unavailable")
	}
	payload, err := encodeRequest(input)
	if err == nil {
		payload, err = pad(payload)
	}
	if err != nil {
		return Result{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://ohttp.invalid/issue", bytes.NewReader(payload))
	if err != nil {
		return Result{}, err
	}
	encapsulated, decapsulator, err := client.transport.Encapsulate(request)
	if err != nil {
		return Result{}, err
	}
	defer encapsulated.Body.Close()
	envelope, err := io.ReadAll(io.LimitReader(encapsulated.Body, maximumEnvelopeSize+1))
	if err != nil || len(envelope) == 0 || len(envelope) > maximumEnvelopeSize {
		return Result{}, errors.New("transit issuance OHTTP envelope is invalid")
	}
	responseEnvelope, err := client.exchange(ctx, envelope)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrExchangeUnavailable, err)
	}
	if len(responseEnvelope) == 0 || len(responseEnvelope) > maximumEnvelopeSize {
		return Result{}, errors.New("transit issuance response is invalid")
	}
	response := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{ohttp.ResponseMediaType}},
		Body: io.NopCloser(bytes.NewReader(responseEnvelope)), Request: encapsulated}
	plain, err := decapsulator.Decapsulate(ctx, response)
	if err != nil {
		return Result{}, errors.New("transit issuance response is invalid")
	}
	defer plain.Body.Close()
	fixed, err := io.ReadAll(io.LimitReader(plain.Body, messageSize+1))
	if err != nil {
		return Result{}, errors.New("transit issuance response is invalid")
	}
	payload, err = unpad(fixed)
	if err != nil {
		return Result{}, errors.New("transit issuance response is invalid")
	}
	result, err := decodeResponse(payload)
	if err != nil {
		return Result{}, errors.New("transit issuance response is invalid")
	}
	if result.Outcome != Issued {
		return result, nil
	}
	grant, err := route.VerifyTransitGrant(result.Grant, client.authority)
	if err != nil || grant.NetworkID != input.NetworkID || grant.Digest != input.Digest || grant.Epoch != input.Epoch ||
		grant.TransitNodeID != input.TransitNodeID || grant.TransitRole != input.TransitRole ||
		grant.AttachmentID != input.AttachmentID || grant.ClientKeyDigest != input.ClientKeyDigest || !grant.NotAfter.Equal(input.NotAfter) {
		return Result{}, errors.New("transit issuance Grant does not match the requested tuple")
	}
	result.Grant = append([]byte(nil), result.Grant...)
	return result, nil
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
