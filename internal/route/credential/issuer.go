package credential

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/route"
)

type issuerConfig struct {
	NetworkID, NodeID  [32]byte
	GrantSigner        ed25519.PrivateKey
	InitiatorNodeID    [32]byte
	InitiatorPublicKey [32]byte
	CurrentDuty        CurrentDuty
	Clock              func() time.Time
}

// Issuer is the isolated OHTTP handler for one State-selected transit-issuance
// duty. It retains no endpoint, name, or unbounded issued-grant history; its
// finite duty root stores only stable OHTTP material and bounded idempotency.
type Issuer struct {
	config        issuerConfig
	nodePublic    [32]byte
	profile       Profile
	profileDigest [32]byte
	handler       http.Handler
	scope         duty.TransitGrantIssuerScope
	find          func(duty.TransitGrantIssuerScope, [32]byte, [32]byte) ([32]byte, bool, error)
	reserve       func(duty.TransitGrantIssuerScope, [32]byte, [32]byte, [32]byte) ([32]byte, bool, error)
	withdraw      func(duty.TransitGrantIssuerScope) error
	close         func() error
}

// Handler returns the issuer's OHTTP-only HTTP handler.
func (issuer *Issuer) Handler() http.Handler { return issuer.handler }

// Profile returns a defensive copy of the issuer's signed common profile.
func (issuer *Issuer) Profile() Profile {
	if issuer == nil {
		return Profile{}
	}
	profile := issuer.profile
	profile.KeyConfig = append([]byte(nil), issuer.profile.KeyConfig...)
	profile.Signature = append([]byte(nil), issuer.profile.Signature...)
	return profile
}

// Close releases the issuer duty's exclusive durable root lease.
func (issuer *Issuer) Close() error {
	if issuer == nil || issuer.close == nil {
		return nil
	}
	close := issuer.close
	issuer.close = nil
	return close()
}

// TLSConfig returns the required mutual-TLS policy for this issuer's HTTPS
// listener. The caller owns the listener certificate; this operation permits
// only the current State-selected Initiator's certified Node key.
func (issuer *Issuer) TLSConfig(certificate tls.Certificate) (*tls.Config, error) {
	if issuer == nil || certificate.PrivateKey == nil || certificate.Leaf == nil || certificateKey(certificate.Leaf) != issuer.nodePublic {
		return nil, errors.New("transit issuance issuer TLS certificate is invalid")
	}
	return &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, Certificates: []tls.Certificate{certificate},
		ClientAuth: tls.RequireAnyClientCert, NextProtos: []string{"http/1.1"}}, nil
}

func (issuer *Issuer) serve(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost || request.URL.Path != "/issue" {
		issuer.writeResult(writer, Result{Outcome: Unavailable})
		return
	}
	fixed, err := io.ReadAll(io.LimitReader(request.Body, messageSize+1))
	payload, requestErr := unpad(fixed)
	decoded, decodeErr := decodeRequest(payload)
	result := Result{Outcome: Unavailable}
	now := issuer.config.Clock().UTC()
	current, available := issuer.config.CurrentDuty()
	profileCurrent := issuer.profileDigest == [32]byte{} || current.ProfileDigest == issuer.profileDigest
	if err == nil && requestErr == nil && decodeErr == nil && available && profileCurrent && validStateDuty(current, issuer.config, issuer.nodePublic, now) &&
		decoded.NetworkID == issuer.config.NetworkID && decoded.Digest == current.Digest && decoded.Epoch == current.Epoch &&
		now.Before(decoded.NotAfter) && !decoded.NotAfter.After(now.Add(15*time.Second)) && !decoded.NotAfter.After(current.NotAfter) {
		result = issuer.issue(decoded, payload, current, now)
	}
	issuer.writeResult(writer, result)
}

func (issuer *Issuer) issue(request Request, payload []byte, current StateDuty, now time.Time) Result {
	digest := sha256.Sum256(payload)
	grantID, found, err := issuer.find(issuer.scope, request.RequestID, digest)
	if errors.Is(err, duty.ErrTransitGrantRequestConflict) {
		return Result{Outcome: Unavailable}
	}
	if err != nil {
		return Result{Outcome: Unavailable}
	}
	if current.Withdrawn {
		if err := issuer.withdraw(issuer.scope); err != nil {
			return Result{Outcome: Unavailable}
		}
		if !found {
			return Result{Outcome: Withdrawn}
		}
	}
	if !found {
		if _, err := rand.Read(grantID[:]); err != nil || grantID == [32]byte{} {
			return Result{Outcome: Unavailable}
		}
		grantID, _, err = issuer.reserve(issuer.scope, request.RequestID, digest, grantID)
		if errors.Is(err, duty.ErrTransitGrantIssuerExhausted) {
			return Result{Outcome: Exhausted}
		}
		if errors.Is(err, duty.ErrTransitGrantIssuerWithdrawn) {
			return Result{Outcome: Withdrawn}
		}
		if err != nil {
			return Result{Outcome: Unavailable}
		}
	}
	grant, err := issuer.signGrant(request, grantID)
	if err != nil {
		return Result{Outcome: Unavailable}
	}
	return Result{Outcome: Issued, Grant: grant}
}

func (issuer *Issuer) signGrant(request Request, grantID [32]byte) ([]byte, error) {
	grantPublic := issuer.config.GrantSigner.Public().(ed25519.PublicKey)
	return route.IssueTransitGrant(route.TransitGrant{IssuerID: sha256.Sum256(grantPublic), GrantID: grantID,
		NetworkID: request.NetworkID, Digest: request.Digest, AttachmentID: request.AttachmentID, TransitNodeID: request.TransitNodeID,
		ClientKeyDigest: request.ClientKeyDigest, Epoch: request.Epoch, TransitRole: request.TransitRole, NotAfter: request.NotAfter}, issuer.config.GrantSigner)
}

func (issuer *Issuer) writeResult(writer http.ResponseWriter, result Result) {
	response, err := encodeResponse(result)
	if err == nil {
		response, err = pad(response)
	}
	if err != nil {
		response, _ = encodeResponse(Result{Outcome: Unavailable})
		response, _ = pad(response)
	}
	writer.Header().Set("Content-Type", "application/octet-stream")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(response)
}

func (issuer *Issuer) acceptsInitiator(state *tls.ConnectionState) bool {
	return state != nil && len(state.PeerCertificates) == 1 && issuer.acceptsInitiatorCertificate(state.PeerCertificates[0])
}

func (issuer *Issuer) acceptsInitiatorCertificate(certificate *x509.Certificate) bool {
	if issuer == nil || certificate == nil {
		return false
	}
	public, ok := certificate.PublicKey.(ed25519.PublicKey)
	return ok && len(public) == len(issuer.config.InitiatorPublicKey) && string(public) == string(issuer.config.InitiatorPublicKey[:])
}

func validStateDuty(duty StateDuty, config issuerConfig, nodePublic [32]byte, now time.Time) bool {
	return duty.NetworkID == config.NetworkID && duty.IssuerNodeID == config.NodeID && duty.IssuerPublicKey == nodePublic &&
		duty.InitiatorNodeID == config.InitiatorNodeID && duty.InitiatorPublicKey == config.InitiatorPublicKey &&
		duty.GrantSignerPublicKey == publicKey(config.GrantSigner) && duty.Digest != [32]byte{} && duty.Epoch != 0 &&
		!duty.NotAfter.IsZero() && duty.NotAfter.Equal(duty.NotAfter.UTC().Truncate(time.Second)) && now.Before(duty.NotAfter)
}

func publicKey(private ed25519.PrivateKey) [32]byte {
	var result [32]byte
	copy(result[:], private.Public().(ed25519.PublicKey))
	return result
}

func certificateKey(certificate *x509.Certificate) [32]byte {
	if certificate == nil {
		return [32]byte{}
	}
	public, ok := certificate.PublicKey.(ed25519.PublicKey)
	if !ok {
		return [32]byte{}
	}
	var result [32]byte
	copy(result[:], public)
	return result
}
