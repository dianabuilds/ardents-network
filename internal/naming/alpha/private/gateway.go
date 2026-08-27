package private

import (
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/cloudflare/circl/hpke"
	"github.com/openpcc/ohttp"
)

// NewGateway creates one alpha-only OHTTP Gateway. Its complete signed corpus
// is supplied at startup, never assembled by a request or an untrusted relay.
func NewGateway(config GatewayConfig) (*gateway, error) {
	if config.Corpus == nil || config.Corpus.Network() == [32]byte{} || config.Corpus.Cohort() == "" ||
		config.NodeID == [32]byte{} || config.Family == "" || config.AssignmentNotAfter.IsZero() || config.Clock == nil ||
		len(config.IdentityKey) != ed25519.PrivateKeySize {
		return nil, errors.New("alpha private Gateway configuration is invalid")
	}
	kem := hpke.KEM_P256_HKDF_SHA256
	public, secret, err := kem.Scheme().GenerateKeyPair()
	if err != nil {
		return nil, err
	}
	keyConfig := ohttp.KeyConfig{KeyID: 1, KemID: kem, PublicKey: public,
		SymmetricAlgorithms: []ohttp.SymmetricAlgorithm{{KDFID: hpke.KDF_HKDF_SHA256, AEADID: hpke.AEAD_AES128GCM}}}
	keyBytes, err := keyConfig.MarshalBinary()
	if err != nil {
		return nil, err
	}
	adapter, err := ohttp.NewGateway(ohttp.KeyPair{SecretKey: secret, KeyConfig: keyConfig})
	if err != nil {
		return nil, err
	}
	profile := GatewayProfile{NetworkID: config.Corpus.Network(), Cohort: config.Corpus.Cohort(), NodeID: config.NodeID,
		Family: config.Family, KeyConfig: keyBytes, KeyConfigDigest: sha256.Sum256(keyBytes), AssignmentNotAfter: config.AssignmentNotAfter}
	profile.Signature = signProfile(profile, config.IdentityKey)
	config.IdentityKey = nil
	gateway := &gateway{config: config, profile: profile}
	middleware := ohttp.Middleware(adapter, http.HandlerFunc(gateway.serve))
	gateway.handler = http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !config.Clock().Before(config.AssignmentNotAfter) {
			http.Error(writer, "unavailable", http.StatusServiceUnavailable)
			return
		}
		middleware.ServeHTTP(writer, request)
	})
	return gateway, nil
}

// Handler returns the Gateway's role-local OHTTP HTTP adapter.
func (gateway *gateway) Handler() http.Handler { return gateway.handler }

// Profile returns an authenticated copy of the public common OHTTP profile.
func (gateway *gateway) Profile() GatewayProfile {
	profile := gateway.profile
	profile.KeyConfig = append([]byte(nil), gateway.profile.KeyConfig...)
	profile.Signature = append([]byte(nil), gateway.profile.Signature...)
	return profile
}

func (gateway *gateway) serve(writer http.ResponseWriter, requestHTTP *http.Request) {
	if requestHTTP.Method != http.MethodPost || requestHTTP.URL.Path != "/resolve" {
		http.Error(writer, "invalid alpha private request", http.StatusBadRequest)
		return
	}
	fixed, err := io.ReadAll(io.LimitReader(requestHTTP.Body, fixedMessageSize+1))
	if err != nil {
		http.Error(writer, "invalid alpha private request", http.StatusBadRequest)
		return
	}
	payload, err := unpad(fixed)
	if err != nil {
		http.Error(writer, "invalid alpha private request", http.StatusBadRequest)
		return
	}
	query, err := decodeRequest(payload)
	now := gateway.config.Clock()
	if err != nil || query.deadline <= now.UnixNano() || query.deadline > now.Add(15*time.Second).UnixNano() {
		http.Error(writer, "invalid alpha private request", http.StatusBadRequest)
		return
	}
	response, err := encodeResponse(response{nonce: query.nonce, deadline: query.deadline, link: query.link, corpus: gateway.config.Corpus.Bytes()})
	if err == nil {
		response, err = pad(response)
	}
	if err != nil {
		http.Error(writer, "alpha private response unavailable", http.StatusServiceUnavailable)
		return
	}
	writer.Header().Set("Content-Type", "application/octet-stream")
	_, _ = writer.Write(response)
}
