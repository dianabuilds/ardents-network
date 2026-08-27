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

	"github.com/cloudflare/circl/hpke"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/openpcc/ohttp"
)

// Issuer is the isolated OHTTP handler for one State-selected transit-issuance
// duty. It retains no endpoint, name, or issued-grant history.
type Issuer struct {
	config     IssuerConfig
	nodePublic [32]byte
	profile    Profile
	handler    http.Handler
}

// NewIssuer opens one bounded project-operated issuer duty.
func NewIssuer(config IssuerConfig) (*Issuer, error) {
	if config.NetworkID == [32]byte{} || config.NodeID == [32]byte{} || len(config.IdentityKey) != ed25519.PrivateKeySize ||
		len(config.GrantSigner) != ed25519.PrivateKeySize || config.InitiatorNodeID == [32]byte{} || config.InitiatorPublicKey == [32]byte{} ||
		config.CurrentDuty == nil || config.Clock == nil || config.Authorize == nil {
		return nil, errors.New("transit issuance issuer configuration is invalid")
	}
	nodePublic := publicKey(config.IdentityKey)
	duty, available := config.CurrentDuty()
	if !available || !validStateDuty(duty, config, nodePublic, config.Clock().UTC()) {
		return nil, errors.New("transit issuance issuer assignment is expired")
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
	grantPublic := config.GrantSigner.Public().(ed25519.PublicKey)
	profile := Profile{NetworkID: config.NetworkID, NodeID: config.NodeID, GrantAuthorityID: sha256.Sum256(grantPublic),
		KeyConfig: encoded, KeyConfigDigest: sha256.Sum256(encoded), AssignmentNotAfter: duty.NotAfter.UTC()}
	profile.Signature = ed25519.Sign(config.IdentityKey, profileTranscript(profile))
	issuer := &Issuer{config: config, nodePublic: nodePublic, profile: profile}
	issuer.config.IdentityKey = nil
	adapter, err := ohttp.NewGateway(ohttp.KeyPair{SecretKey: secret, KeyConfig: keyConfig})
	if err != nil {
		return nil, err
	}
	middleware := ohttp.Middleware(adapter, http.HandlerFunc(issuer.serve))
	issuer.handler = http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !issuer.config.Clock().Before(issuer.profile.AssignmentNotAfter) || !issuer.acceptsInitiator(request.TLS) {
			http.Error(writer, "unavailable", http.StatusServiceUnavailable)
			return
		}
		middleware.ServeHTTP(writer, request)
	})
	return issuer, nil
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
		http.Error(writer, "invalid transit issuance request", http.StatusBadRequest)
		return
	}
	fixed, err := io.ReadAll(io.LimitReader(request.Body, messageSize+1))
	payload, requestErr := unpad(fixed)
	decoded, decodeErr := decodeRequest(payload)
	now := issuer.config.Clock().UTC()
	duty, available := issuer.config.CurrentDuty()
	if err != nil || requestErr != nil || decodeErr != nil || !available || !validStateDuty(duty, issuer.config, issuer.nodePublic, now) ||
		decoded.NetworkID != issuer.config.NetworkID || decoded.Digest != duty.Digest || decoded.Epoch != duty.Epoch ||
		!now.Before(decoded.NotAfter) || decoded.NotAfter.After(now.Add(15*time.Second)) || decoded.NotAfter.After(duty.NotAfter) ||
		!issuer.config.Authorize(decoded, now) {
		http.Error(writer, "unavailable", http.StatusServiceUnavailable)
		return
	}
	var grantID [32]byte
	if _, err := rand.Read(grantID[:]); err != nil || grantID == [32]byte{} {
		http.Error(writer, "unavailable", http.StatusServiceUnavailable)
		return
	}
	grantPublic := issuer.config.GrantSigner.Public().(ed25519.PublicKey)
	grant, err := route.IssueTransitGrant(route.TransitGrant{IssuerID: sha256.Sum256(grantPublic), GrantID: grantID,
		NetworkID: decoded.NetworkID, Digest: decoded.Digest, AttachmentID: decoded.AttachmentID, TransitNodeID: decoded.IntroductionNodeID,
		ClientKeyDigest: decoded.ClientKeyDigest, Epoch: decoded.Epoch, TransitRole: route.IntroductionRole, NotAfter: decoded.NotAfter}, issuer.config.GrantSigner)
	if err == nil {
		var response []byte
		response, err = encodeResponse(grant)
		if err == nil {
			response, err = pad(response)
		}
		if err == nil {
			writer.Header().Set("Content-Type", "application/octet-stream")
			_, err = writer.Write(response)
		}
	}
	if err != nil {
		http.Error(writer, "unavailable", http.StatusServiceUnavailable)
	}
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

func validStateDuty(duty StateDuty, config IssuerConfig, nodePublic [32]byte, now time.Time) bool {
	grantPublic := config.GrantSigner.Public().(ed25519.PublicKey)
	return duty.NetworkID == config.NetworkID && duty.IssuerNodeID == config.NodeID && duty.IssuerPublicKey == nodePublic &&
		duty.InitiatorNodeID == config.InitiatorNodeID && duty.InitiatorPublicKey == config.InitiatorPublicKey &&
		duty.GrantAuthorityID == sha256.Sum256(grantPublic) && duty.Digest != [32]byte{} && duty.Epoch != 0 &&
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
