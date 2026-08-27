package endpoint

import (
	"crypto/tls"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// validOptionalTransitClientCertificate permits the retained fresh-key path
// for lower-level component tests. A non-zero pre-enrolled certificate must
// contain both its private key and parsed leaf before it reaches Route.
func validOptionalTransitClientCertificate(value tls.Certificate) bool {
	return value.PrivateKey == nil && value.Leaf == nil || value.PrivateKey != nil && value.Leaf != nil
}

func cloneTransitClientCertificates(input map[[32]byte]tls.Certificate) (map[[32]byte]tls.Certificate, error) {
	if len(input) == 0 {
		return nil, nil
	}
	result := make(map[[32]byte]tls.Certificate, len(input))
	for grantID, certificate := range input {
		if grantID == [32]byte{} || !validOptionalTransitClientCertificate(certificate) || certificate.PrivateKey == nil {
			return nil, errors.New("endpoint transit credential enrollment is invalid")
		}
		result[grantID] = certificate
	}
	return result, nil
}

// transitClientCertificate resolves a grant-bound identity only from this
// Endpoint's local enrollment. Legacy opaque authorizations retain the fresh
// per-attempt key path used by narrow component tests; a decodable Transit
// Grant is never silently attempted with a fresh or caller-substituted key.
func (endpoint *endpoint) transitClientCertificate(authorization []byte, supplied tls.Certificate) (tls.Certificate, error) {
	grant, err := route.DecodeTransitGrant(authorization)
	if err != nil {
		return supplied, nil
	}
	if endpoint == nil {
		return tls.Certificate{}, errors.New("transit grant has no Endpoint-local credential")
	}
	endpoint.transitMu.Lock()
	certificate, ok := endpoint.transitClients[grant.GrantID]
	endpoint.transitMu.Unlock()
	if !ok || !validOptionalTransitClientCertificate(certificate) || certificate.PrivateKey == nil {
		return tls.Certificate{}, errors.New("transit grant credential is not enrolled")
	}
	digest, err := route.ClientTLSKeyDigest(certificate.Leaf)
	if err != nil || digest != grant.ClientKeyDigest {
		return tls.Certificate{}, errors.New("transit grant credential does not match its key binding")
	}
	if supplied.PrivateKey != nil {
		suppliedDigest, suppliedErr := route.ClientTLSKeyDigest(supplied.Leaf)
		if suppliedErr != nil || suppliedDigest != digest {
			return tls.Certificate{}, errors.New("caller supplied a transit credential outside local enrollment")
		}
	}
	return certificate, nil
}

func (endpoint *endpoint) enrollTransitClient(authorization []byte, certificate tls.Certificate) (func(), error) {
	grant, err := route.DecodeTransitGrant(authorization)
	if err != nil || grant.GrantID == [32]byte{} || !validOptionalTransitClientCertificate(certificate) || certificate.PrivateKey == nil {
		return nil, errors.New("transit credential enrollment is invalid")
	}
	digest, err := route.ClientTLSKeyDigest(certificate.Leaf)
	if err != nil || digest != grant.ClientKeyDigest {
		return nil, errors.New("transit credential enrollment does not match its Grant")
	}
	endpoint.transitMu.Lock()
	if endpoint.transitClients == nil {
		endpoint.transitClients = make(map[[32]byte]tls.Certificate)
	}
	if _, exists := endpoint.transitClients[grant.GrantID]; exists {
		endpoint.transitMu.Unlock()
		return nil, errors.New("transit credential Grant is already enrolled")
	}
	endpoint.transitClients[grant.GrantID] = certificate
	endpoint.transitMu.Unlock()
	return func() {
		endpoint.transitMu.Lock()
		delete(endpoint.transitClients, grant.GrantID)
		endpoint.transitMu.Unlock()
	}, nil
}
