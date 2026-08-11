package siteexperiment

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net/http"

	"github.com/cloudflare/circl/hpke"
	"github.com/openpcc/ohttp"
)

type gatewayStatusError struct {
	status int
}

func (failure *gatewayStatusError) Error() string {
	return "OHTTP Gateway returned status " + http.StatusText(failure.status)
}

const resolutionMessageSize = 4096

func newOHTTPGateway(target http.Handler) (ohttp.KeyConfig, http.Handler, error) {
	if target == nil {
		return ohttp.KeyConfig{}, nil, errors.New("OHTTP Gateway target is required")
	}
	kem := hpke.KEM_P256_HKDF_SHA256
	publicKey, secretKey, err := kem.Scheme().GenerateKeyPair()
	if err != nil {
		return ohttp.KeyConfig{}, nil, err
	}
	config := ohttp.KeyConfig{
		KeyID: 1, KemID: kem, PublicKey: publicKey,
		SymmetricAlgorithms: []ohttp.SymmetricAlgorithm{{KDFID: hpke.KDF_HKDF_SHA256, AEADID: hpke.AEAD_AES128GCM}},
	}
	gateway, err := ohttp.NewGateway(ohttp.KeyPair{SecretKey: secretKey, KeyConfig: config})
	if err != nil {
		return ohttp.KeyConfig{}, nil, err
	}
	return config, ohttp.Middleware(gateway, target), nil
}

func newOHTTPTransport(config ohttp.KeyConfig, relayURL string, client *http.Client) (http.RoundTripper, error) {
	if relayURL == "" || client == nil {
		return nil, errors.New("OHTTP Relay URL and bounded client are required")
	}
	return ohttp.NewTransport(config, relayURL, ohttp.WithHTTPClient(client))
}

func sendOHTTPMessage(ctx context.Context, transport http.RoundTripper, plaintext []byte) ([]byte, error) {
	if ctx == nil || transport == nil || len(plaintext) != resolutionMessageSize {
		return nil, errors.New("OHTTP request requires context, transport, and one fixed-size message")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://ohttp.invalid/lookup", bytes.NewReader(plaintext))
	if err != nil {
		return nil, err
	}
	response, err := (&http.Client{Transport: transport}).Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, &gatewayStatusError{status: response.StatusCode}
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, resolutionMessageSize+1))
	if err != nil {
		return nil, err
	}
	if len(body) != resolutionMessageSize {
		return nil, errors.New("OHTTP response does not have the fixed plaintext size")
	}
	return body, nil
}

func padResolutionMessage(message []byte) ([]byte, error) {
	if len(message) == 0 || len(message) > resolutionMessageSize-2 {
		return nil, errors.New("resolution message does not fit the fixed envelope")
	}
	padded := make([]byte, resolutionMessageSize)
	binary.BigEndian.PutUint16(padded[:2], uint16(len(message)))
	copy(padded[2:], message)
	return padded, nil
}

func unpadResolutionMessage(padded []byte) ([]byte, error) {
	if len(padded) != resolutionMessageSize {
		return nil, errors.New("resolution plaintext has the wrong size")
	}
	length := int(binary.BigEndian.Uint16(padded[:2]))
	if length == 0 || length > resolutionMessageSize-2 {
		return nil, errors.New("resolution plaintext has an invalid content length")
	}
	for _, value := range padded[2+length:] {
		if value != 0 {
			return nil, errors.New("resolution plaintext has non-canonical padding")
		}
	}
	return bytes.Clone(padded[2 : 2+length]), nil
}
