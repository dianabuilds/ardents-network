package reachability

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/cloudflare/circl/hpke"
	"github.com/openpcc/ohttp"
)

const (
	privateGatewayProfileDomain = "ardents-reachability-gateway-v1"
	ohttpRequestType            = ohttp.RequestMediaType
)

// GatewayProfile is the common authenticated OHTTP configuration for one
// Destination Resolution Gateway. Network State must authenticate its Node
// identity before a production Endpoint can use this public value.
type GatewayProfile struct {
	NetworkID, NodeID  [32]byte
	KeyConfig          []byte
	KeyConfigDigest    [32]byte
	AssignmentNotAfter time.Time
	Signature          []byte
}

// GatewayConfig contains only Gateway-local currentness and identity facts.
// It has no Endpoint address, Route, Service private key, or publisher origin.
type GatewayConfig struct {
	NetworkID, NodeID  [32]byte
	IdentityKey        ed25519.PrivateKey
	AssignmentNotAfter time.Time
	Store              *Store
	Clock              func() time.Time
}

// Gateway decapsulates one bounded private lookup and delegates only exact
// currentness to its Store.
type Gateway struct {
	config  GatewayConfig
	profile GatewayProfile
	handler http.Handler

	mu   sync.Mutex
	seen map[[32]byte]int64
}

// NewGateway creates a strict OHTTP Gateway whose private key remains in the
// returned handler. The caller retains Store ownership and closes it separately.
func NewGateway(config GatewayConfig) (*Gateway, error) {
	if config.NetworkID == [32]byte{} || config.NodeID == [32]byte{} || len(config.IdentityKey) != ed25519.PrivateKeySize ||
		config.AssignmentNotAfter.IsZero() || config.Store == nil || config.Clock == nil {
		return nil, errors.New("private reachability Gateway configuration is invalid")
	}
	kem := hpke.KEM_P256_HKDF_SHA256
	public, secret, err := kem.Scheme().GenerateKeyPair()
	if err != nil {
		return nil, err
	}
	keyConfig := ohttp.KeyConfig{KeyID: 1, KemID: kem, PublicKey: public,
		SymmetricAlgorithms: []ohttp.SymmetricAlgorithm{{KDFID: hpke.KDF_HKDF_SHA256, AEADID: hpke.AEAD_AES128GCM}}}
	encoded, err := keyConfig.MarshalBinary()
	if err != nil {
		return nil, err
	}
	adapter, err := ohttp.NewGateway(ohttp.KeyPair{SecretKey: secret, KeyConfig: keyConfig})
	if err != nil {
		return nil, err
	}
	profile := GatewayProfile{NetworkID: config.NetworkID, NodeID: config.NodeID, KeyConfig: encoded,
		KeyConfigDigest: sha256.Sum256(encoded), AssignmentNotAfter: config.AssignmentNotAfter}
	profile.Signature = ed25519.Sign(config.IdentityKey, gatewayProfileTranscript(profile))
	config.IdentityKey = nil
	gateway := &Gateway{config: config, profile: profile, seen: make(map[[32]byte]int64)}
	middleware := ohttp.Middleware(adapter, http.HandlerFunc(gateway.serve))
	gateway.handler = http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !gateway.config.Clock().Before(gateway.profile.AssignmentNotAfter) {
			http.Error(writer, "unavailable", http.StatusServiceUnavailable)
			return
		}
		middleware.ServeHTTP(writer, request)
	})
	return gateway, nil
}

// Handler returns the OHTTP-only Gateway adapter.
func (gateway *Gateway) Handler() http.Handler { return gateway.handler }

// Profile returns a defensive copy of the signed common configuration.
func (gateway *Gateway) Profile() GatewayProfile {
	profile := gateway.profile
	profile.KeyConfig = append([]byte(nil), profile.KeyConfig...)
	profile.Signature = append([]byte(nil), profile.Signature...)
	return profile
}

func (gateway *Gateway) serve(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost || request.URL.Path != "/resolve" {
		gateway.reject(writer)
		return
	}
	fixed, err := io.ReadAll(io.LimitReader(request.Body, privateMessageSize+1))
	if err != nil {
		gateway.reject(writer)
		return
	}
	payload, err := unpadPrivateMessage(fixed)
	if err != nil {
		gateway.reject(writer)
		return
	}
	query, err := decodePrivateRequest(payload)
	now := gateway.config.Clock()
	if err != nil || query.network != gateway.config.NetworkID || query.deadline <= now.UnixNano() ||
		query.deadline > now.Add(15*time.Second).UnixNano() || !gateway.acceptNonce(query.nonce, query.deadline, now) {
		gateway.reject(writer)
		return
	}
	raw, class, lookupErr := gateway.config.Store.Lookup(query.target, now)
	responseClass := privateUnavailable
	if lookupErr == nil && class == StoreAlreadyCurrent {
		responseClass = privateResolved
	} else if class == StoreConflicting {
		responseClass = privateConflicting
	} else if class == StoreInvalid {
		responseClass = privateInvalid
	}
	response, err := encodePrivateResponse(privateResponse{network: query.network, target: query.target, nonce: query.nonce,
		deadline: query.deadline, class: responseClass, descriptor: raw})
	if err == nil {
		response, err = padPrivateMessage(response)
	}
	if err != nil {
		gateway.reject(writer)
		return
	}
	writer.Header().Set("Content-Type", "application/octet-stream")
	_, _ = writer.Write(response)
}

func (gateway *Gateway) acceptNonce(nonce [32]byte, deadline int64, now time.Time) bool {
	gateway.mu.Lock()
	defer gateway.mu.Unlock()
	for value, expiry := range gateway.seen {
		if expiry <= now.UnixNano() {
			delete(gateway.seen, value)
		}
	}
	if nonce == [32]byte{} || gateway.seen[nonce] != 0 || len(gateway.seen) >= 128 {
		return false
	}
	gateway.seen[nonce] = deadline
	return true
}

func (gateway *Gateway) reject(writer http.ResponseWriter) {
	writer.Header().Set("Content-Type", "application/octet-stream")
	_, _ = writer.Write(make([]byte, privateMessageSize))
}

func validGatewayProfile(profile GatewayProfile, public [32]byte) bool {
	return len(profile.Signature) == ed25519.SignatureSize && len(profile.KeyConfig) > 0 &&
		profile.KeyConfigDigest == sha256.Sum256(profile.KeyConfig) && !profile.AssignmentNotAfter.IsZero() &&
		ed25519.Verify(ed25519.PublicKey(public[:]), gatewayProfileTranscript(profile), profile.Signature)
}

func gatewayProfileTranscript(profile GatewayProfile) []byte {
	out := make([]byte, 0, 2+len(privateGatewayProfileDomain)+32+32+8+4+len(profile.KeyConfig))
	out = binary.BigEndian.AppendUint16(out, uint16(len(privateGatewayProfileDomain)))
	out = append(out, privateGatewayProfileDomain...)
	out = append(out, profile.NetworkID[:]...)
	out = append(out, profile.NodeID[:]...)
	out = binary.BigEndian.AppendUint64(out, uint64(profile.AssignmentNotAfter.UnixNano()))
	out = binary.BigEndian.AppendUint32(out, uint32(len(profile.KeyConfig)))
	return append(out, profile.KeyConfig...)
}
