package nameresolution

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

// NewGateway creates one finite common OHTTP configuration and a strict
// naming-side handler. The key remains private to the returned Gateway.
func NewGateway(config GatewayConfig) (*gateway, error) {
	if config.NodeID == [32]byte{} || config.Family == "" || config.Domain != rendezvousDomain ||
		config.AssignmentNotAfter.IsZero() || config.MaximumPending == 0 || config.Clock == nil ||
		len(config.IdentityKey) != ed25519.PrivateKeySize {
		return nil, errors.New("naming Gateway role is invalid")
	}
	records, err := newRecordSet(config.NetworkID, config.SignedRecordChains)
	if err != nil {
		return nil, err
	}
	kem := hpke.KEM_P256_HKDF_SHA256
	public, secret, err := kem.Scheme().GenerateKeyPair()
	if err != nil {
		return nil, err
	}
	keyConfig := ohttp.KeyConfig{KeyID: 1, KemID: kem, PublicKey: public,
		SymmetricAlgorithms: []ohttp.SymmetricAlgorithm{{KDFID: hpke.KDF_HKDF_SHA256, AEADID: hpke.AEAD_AES128GCM}}}
	configBytes, err := keyConfig.MarshalBinary()
	if err != nil {
		return nil, err
	}
	adapter, err := ohttp.NewGateway(ohttp.KeyPair{SecretKey: secret, KeyConfig: keyConfig})
	if err != nil {
		return nil, err
	}
	profile := GatewayProfile{NetworkID: records.network, NodeID: config.NodeID,
		KeyConfig: configBytes, KeyConfigDigest: sha256.Sum256(configBytes),
		AssignmentNotAfter: config.AssignmentNotAfter}
	profile.Signature = signGatewayProfile(profile, config.IdentityKey)
	config.IdentityKey = nil
	config.SignedRecordChains = nil
	gateway := &gateway{config: config, records: records, seen: make(map[[32]byte]int64), profile: profile}
	application := http.HandlerFunc(gateway.resolve)
	middleware := ohttp.Middleware(adapter, application)
	gateway.handler = http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !config.Clock().Before(config.AssignmentNotAfter) {
			http.Error(writer, "unavailable", http.StatusServiceUnavailable)
			return
		}
		middleware.ServeHTTP(writer, request)
	})
	return gateway, nil
}

// Handler returns the role-local OHTTP server Adapter.
func (gateway *gateway) Handler() http.Handler { return gateway.handler }

// Profile returns a copy of the public configuration for authenticated provisioning.
func (gateway *gateway) Profile() GatewayProfile {
	profile := gateway.profile
	profile.KeyConfig = append([]byte(nil), profile.KeyConfig...)
	profile.Signature = append([]byte(nil), profile.Signature...)
	return profile
}

// Observation returns only bounded observer-safe Gateway counters.
func (gateway *gateway) Observation() (requests, resolved, rejected uint32) {
	gateway.mu.Lock()
	defer gateway.mu.Unlock()
	return gateway.observation.Requests, gateway.observation.Resolved, gateway.observation.Rejected
}

func (gateway *gateway) resolve(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost || request.URL.Path != "/resolve" {
		gateway.reject(writer)
		return
	}
	fixed, err := io.ReadAll(io.LimitReader(request.Body, fixedMessageSize+1))
	if err != nil {
		gateway.reject(writer)
		return
	}
	payload, err := unpadMessage(fixed)
	if err != nil {
		gateway.reject(writer)
		return
	}
	query, err := decodeRequest(payload)
	now := gateway.config.Clock()
	if err != nil || query.network != gateway.records.network || query.deadline <= now.UnixNano() ||
		query.deadline > now.Add(15*time.Second).UnixNano() || !gateway.acceptNonce(query.nonce, query.deadline, now) {
		gateway.reject(writer)
		return
	}
	chain, found := gateway.records.lookup(query.name)
	result := byte(resultUnavailable)
	generation, revision := uint64(0), uint64(0)
	if found {
		result = resultResolved
		generation, revision = chain.head.Generation, chain.head.Revision
	}
	response, err := encodeResponse(resolutionResponse{network: query.network, nonce: query.nonce,
		deadline: query.deadline, name: query.name, generation: generation, revision: revision,
		result: result, chain: chain.signed})
	if err == nil {
		response, err = padMessage(response)
	}
	if err != nil {
		gateway.reject(writer)
		return
	}
	writer.Header().Set("Content-Type", "application/octet-stream")
	_, _ = writer.Write(response)
	gateway.mu.Lock()
	gateway.observation.Requests++
	if found {
		gateway.observation.Resolved++
	} else {
		gateway.observation.Rejected++
	}
	gateway.mu.Unlock()
}

func (gateway *gateway) acceptNonce(nonce [32]byte, deadline int64, now time.Time) bool {
	if nonce == [32]byte{} {
		return false
	}
	gateway.mu.Lock()
	defer gateway.mu.Unlock()
	for key, expiry := range gateway.seen {
		if expiry <= now.UnixNano() {
			delete(gateway.seen, key)
		}
	}
	if _, duplicate := gateway.seen[nonce]; duplicate || len(gateway.seen) >= int(gateway.config.MaximumPending) {
		return false
	}
	gateway.seen[nonce] = deadline
	return true
}

func (gateway *gateway) reject(writer http.ResponseWriter) {
	gateway.mu.Lock()
	gateway.observation.Rejected++
	gateway.mu.Unlock()
	writer.Header().Set("Content-Type", "application/octet-stream")
	_, _ = writer.Write(make([]byte, fixedMessageSize))
}
