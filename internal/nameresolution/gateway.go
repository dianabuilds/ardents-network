package nameresolution

import (
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/cloudflare/circl/hpke"
	"github.com/dianabuilds/ardents-network/internal/namestore"
	"github.com/openpcc/ohttp"
)

// NewGateway creates one finite common OHTTP configuration and a strict
// naming-side handler. The key remains private to the returned Gateway.
func NewGateway(config GatewayConfig) (*gateway, error) {
	if config.NodeID == [32]byte{} || config.Family == "" || config.Domain != rendezvousDomain ||
		config.AssignmentNotAfter.IsZero() || config.MaximumPending == 0 || config.Clock == nil ||
		len(config.IdentityKey) != ed25519.PrivateKeySize || config.State.recordStore == nil ||
		config.State.admission == nil || config.State.epochDigest == [32]byte{} {
		return nil, errors.New("naming Gateway role is invalid")
	}
	records, err := newRecordSet(config.State.recordStore, config.State.network, config.State.minimum)
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
	state := config.State
	config.State = gatewayState{}
	gateway := &gateway{config: config, state: state, records: records, seen: make(map[[32]byte]int64), profile: profile}
	application := http.HandlerFunc(gateway.serve)
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

func (gateway *gateway) serve(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path == "/control" {
		gateway.control(writer, request)
		return
	}
	gateway.resolve(writer, request)
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
	operationDigest, digestErr := resolutionAdmissionDigest(query.network, query.name, query.deadline)
	admitted := false
	if err == nil && digestErr == nil && query.admission.Challenge.OperationDigest == operationDigest &&
		query.admission.Challenge.Node == gateway.config.NodeID {
		admitted, _ = gateway.state.admission.Verify(now.UnixMilli(), query.admission)
	}
	if err != nil || query.network != gateway.records.network || query.deadline <= now.UnixNano() ||
		query.deadline > now.Add(15*time.Second).UnixNano() || !admitted ||
		!gateway.acceptNonce(query.nonce, query.deadline, now) {
		gateway.reject(writer)
		return
	}
	proof, found := gateway.records.lookup(query.name)
	result := byte(resultUnavailable)
	generation, revision := uint64(0), uint64(0)
	target := [32]byte{}
	if found {
		record, _, _, _, verifyErr := namestore.Verify(gateway.state.policy, proof,
			gateway.state.minimum, gateway.state.epochDigest, now.Unix())
		if verifyErr != nil {
			gateway.reject(writer)
			return
		}
		result = resultResolved
		generation, revision = record.Generation, record.Revision
		target = record.Target
	}
	response, err := encodeResponse(resolutionResponse{network: query.network, nonce: query.nonce,
		deadline: query.deadline, name: query.name, generation: generation, revision: revision,
		result: result, proof: proof})
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
	role := gatewayRoleEvidence{Operation: "resolve", Name: query.name, Network: query.network,
		Nonce: query.nonce, Target: target, Deadline: query.deadline,
		Generation: generation, Revision: revision, Result: resolutionUnavailableClass}
	if found {
		gateway.observation.Resolved++
		role.Result = resolvedClass
	} else {
		gateway.observation.Rejected++
	}
	gateway.roleEvidence = append(gateway.roleEvidence, role)
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
