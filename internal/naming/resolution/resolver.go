package resolution

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/openpcc/ohttp"
)

const ohttpRequestType = ohttp.RequestMediaType

// Open selects authenticated roles, binds one local Isolation Context, and
// constructs a single-use OHTTP Adapter atomically. A retry must call Open
// again and therefore receives a fresh Adapter and transport.
func Open(view state.Snapshot, selection Selection, profile GatewayProfile, isolation [32]byte,
	base *http.Transport) (*resolver, error) {
	resolutionView, viewErr := view.Resolution()
	if viewErr != nil {
		return nil, errors.New("private resolution Network State view is invalid")
	}
	plan, err := selectPlan(resolutionView, selection, profile)
	if err != nil || isolation == [32]byte{} || base == nil {
		return nil, errors.New("private resolution client configuration is invalid")
	}
	if !plan.AdmissionChallenge.BindsIsolation(isolation) {
		return nil, errors.New("private resolution admission does not bind the Isolation Context")
	}
	var key ohttp.KeyConfig
	if err := key.UnmarshalBinary(plan.GatewayKeyConfig); err != nil {
		return nil, errors.New("private resolution Gateway key is invalid")
	}
	relayURL := "https://" + plan.Relay.Endpoint + "/ohttp"
	client := isolatedHTTPClient(base)
	transport, err := ohttp.NewTransport(key, relayURL, ohttp.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}
	role := resolverRoleEvidence{Operation: "resolve", Isolation: isolation, Network: plan.NetworkID,
		Relay: plan.Relay.NodeID, Gateway: plan.Gateway.NodeID,
		Rendezvous: plan.ConnectionRendezvous.NodeID, Deadline: plan.Deadline}
	return &resolver{plan: clonePlan(plan), client: client, transport: transport, roleEvidence: role}, nil
}

// Resolve performs one fresh, fixed-size private lookup and independently
// authenticates the complete child-to-root Record chain before returning Target.
func (resolver *resolver) Resolve(ctx context.Context, serviceName string, at time.Time) (result, error) {
	if !resolver.begin() {
		return resolver.failure(policyDenialClass, errors.New("private resolution Adapter is single-use"))
	}
	if ctx == nil || at.IsZero() || at.UnixNano() != resolver.plan.SelectionAt {
		return resolver.failure(policyDenialClass, errors.New("local resolution input is invalid"))
	}
	deadline := time.Unix(0, resolver.plan.Deadline)
	attempt, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return resolver.failure(resolutionUnavailableClass, err)
	}
	resolver.mu.Lock()
	resolver.roleEvidence.Name, resolver.roleEvidence.Nonce = serviceName, nonce
	resolver.mu.Unlock()
	operationDigest, err := resolutionAdmissionDigest(resolver.plan.NetworkID, serviceName, resolver.plan.Deadline)
	if err != nil || operationDigest != resolver.plan.AdmissionChallenge.OperationDigest {
		return resolver.failure(policyDenialClass, errors.New("resolution admission operation is invalid"))
	}
	admissionProof, _ := resolver.plan.AdmissionChallenge.Solve()
	payload, err := encodeRequest(resolutionRequest{network: resolver.plan.NetworkID, nonce: nonce,
		deadline: resolver.plan.Deadline, name: serviceName, admission: admissionProof})
	if err == nil {
		payload, err = padMessage(payload)
	}
	if err != nil {
		return resolver.failure(policyDenialClass, err)
	}
	plainRequest, err := http.NewRequestWithContext(attempt, http.MethodPost, "http://ohttp.invalid/resolve", bytes.NewReader(payload))
	if err != nil {
		return resolver.failure(policyDenialClass, err)
	}
	encapsulated, decapsulator, err := resolver.transport.Encapsulate(plainRequest)
	if err != nil {
		return resolver.failure(resolutionUnavailableClass, err)
	}
	outerResponse, err := resolver.client.Do(encapsulated)
	defer resolver.client.CloseIdleConnections()
	if err != nil {
		return resolver.failure(resolutionUnavailableClass, err)
	}
	defer outerResponse.Body.Close()
	plainResponse, err := decapsulator.Decapsulate(attempt, outerResponse)
	if err != nil {
		return resolver.failure(invalidEvidenceClass, err)
	}
	defer plainResponse.Body.Close()
	fixed, err := io.ReadAll(io.LimitReader(plainResponse.Body, fixedMessageSize+1))
	if err != nil {
		return resolver.failure(invalidEvidenceClass, err)
	}
	responsePayload, err := unpadMessage(fixed)
	if err != nil {
		return resolver.failure("invalid naming evidence", err)
	}
	response, err := decodeResponse(responsePayload)
	if err != nil || response.network != resolver.plan.NetworkID || response.nonce != nonce ||
		response.deadline != resolver.plan.Deadline || response.name != serviceName {
		return resolver.failure(invalidEvidenceClass, errors.New("resolution response binding is invalid"))
	}
	if response.result != resultResolved {
		return resolver.failure(resolutionUnavailableClass, errors.New("name is unavailable"))
	}
	binding, warning, err := resolver.plan.NamespaceVerifier.Verify(response.proof, at.UnixMilli())
	if err != nil || binding.Name != serviceName {
		return resolver.failure(invalidEvidenceClass, errors.New("resolution returned the wrong name"))
	}
	if binding.Generation != response.generation || binding.Revision != response.revision {
		return resolver.failure(invalidEvidenceClass, errors.New("resolution response Record version is inconsistent"))
	}
	resolver.mu.Lock()
	resolver.observation.Resolved++
	resolver.roleEvidence.Result = resolvedClass
	resolver.roleEvidence.Target = binding.Target
	resolver.roleEvidence.Generation = binding.Generation
	resolver.roleEvidence.Revision = binding.Revision
	resolver.mu.Unlock()
	return result{Class: resolvedClass, Warning: warning, Binding: binding}, nil
}

func isolatedHTTPClient(base *http.Transport) *http.Client {
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

// Observation returns only bounded local counts with no query-derived identifier.
func (resolver *resolver) Observation() (requests, resolved, failed uint32) {
	resolver.mu.Lock()
	defer resolver.mu.Unlock()
	return resolver.observation.Requests, resolver.observation.Resolved, resolver.observation.Failed
}

// ConnectionExclusions returns the resolution identities/families that a
// subsequent connection plan must exclude, plus its distinct Rendezvous.
func (resolver *resolver) ConnectionExclusions() ([][32]byte, []string, [32]byte) {
	return append([][32]byte(nil), resolver.plan.ExcludedIdentities...),
		append([]string(nil), resolver.plan.ExcludedFamilies...), resolver.plan.ConnectionRendezvous.NodeID
}

func (resolver *resolver) failure(class string, _ error) (result, error) {
	resolver.mu.Lock()
	resolver.observation.Failed++
	resolver.roleEvidence.Result = class
	resolver.mu.Unlock()
	return result{Class: class}, errors.New(class)
}

func (resolver *resolver) begin() bool {
	resolver.mu.Lock()
	defer resolver.mu.Unlock()
	resolver.observation.Requests++
	if resolver.used {
		return false
	}
	resolver.used = true
	return true
}

func clonePlan(plan plan) plan {
	plan.GatewayKeyConfig = append([]byte(nil), plan.GatewayKeyConfig...)
	plan.ExcludedIdentities = append([][32]byte(nil), plan.ExcludedIdentities...)
	plan.ExcludedFamilies = append([]string(nil), plan.ExcludedFamilies...)
	return plan
}
