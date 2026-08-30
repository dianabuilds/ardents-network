package credential

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"net/http"

	"github.com/cloudflare/circl/hpke"
	"github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/openpcc/ohttp"
)

// OpenIssuerFromRoot opens one bootstrapped generation only when current
// authenticated State binds its exact public profile and permitted Initiator.
func OpenIssuerFromRoot(config RootIssuerConfig) (*Issuer, error) {
	if config.Root == "" || config.NetworkID == [32]byte{} || config.NodeID == [32]byte{} ||
		len(config.IdentityKey) != ed25519.PrivateKeySize || config.CurrentDuty == nil || config.Clock == nil {
		return nil, errors.New("root-backed transit issuer configuration is invalid")
	}
	ledger, err := duty.Open(duty.Config{Root: config.Root, Clock: config.Clock})
	if err != nil {
		return nil, err
	}
	closeOnError := func(cause error) (*Issuer, error) { return nil, errors.Join(cause, ledger.Close()) }
	privateMaterial, rawProfile, err := ledger.TransitGrantIssuerRoot()
	if err != nil {
		return closeOnError(err)
	}
	grantPrivate, ohttpMaterial, err := decodeIssuerMaterial(privateMaterial)
	if err != nil {
		return closeOnError(err)
	}
	profile, err := DecodeProfile(rawProfile)
	if err != nil || profile.Version != profileVersion || profile.NetworkID != config.NetworkID || profile.NodeID != config.NodeID {
		return closeOnError(errors.New("root-backed transit issuer profile is invalid"))
	}
	rebuilt, err := issuerProfile(IssuerRootConfig{NetworkID: config.NetworkID, NodeID: config.NodeID, IdentityKey: config.IdentityKey,
		InitiatorNodeID: profile.InitiatorNodeID, InitiatorPublicKey: profile.InitiatorPublicKey,
		AssignmentNotAfter: profile.AssignmentNotAfter}, grantPrivate.Public().(ed25519.PublicKey), ohttpMaterial)
	if err != nil {
		return closeOnError(err)
	}
	rebuiltRaw, err := EncodeProfile(rebuilt)
	if err != nil || !bytes.Equal(rebuiltRaw, rawProfile) {
		return closeOnError(errors.New("root-backed transit issuer public binding changed"))
	}
	now := config.Clock().UTC()
	current, available := config.CurrentDuty()
	issuerConfig := issuerConfig{NetworkID: config.NetworkID, NodeID: config.NodeID,
		GrantSigner: grantPrivate, InitiatorNodeID: profile.InitiatorNodeID, InitiatorPublicKey: profile.InitiatorPublicKey,
		CurrentDuty: config.CurrentDuty, Clock: config.Clock}
	nodePublic := publicKey(config.IdentityKey)
	profileDigest := sha256.Sum256(rawProfile)
	if !available || current.ProfileDigest != profileDigest || !validStateDuty(current, issuerConfig, nodePublic, now) ||
		!current.NotAfter.Equal(profile.AssignmentNotAfter) {
		return closeOnError(errors.New("root-backed transit issuer assignment is unavailable"))
	}
	scope := duty.TransitGrantIssuerScope{NetworkID: current.NetworkID, Digest: current.Digest, IssuerNodeID: current.IssuerNodeID,
		GrantSignerID: profile.GrantSignerID, Epoch: current.Epoch, NotAfter: current.NotAfter}
	if err := ledger.BindTransitGrantIssuer(scope); err != nil {
		return closeOnError(err)
	}
	secret, err := hpke.KEM_P256_HKDF_SHA256.Scheme().UnmarshalBinaryPrivateKey(ohttpMaterial)
	if err != nil {
		return closeOnError(errors.New("root-backed transit issuer OHTTP material is invalid"))
	}
	keyConfig := ohttp.KeyConfig{KeyID: 1, KemID: hpke.KEM_P256_HKDF_SHA256, PublicKey: secret.Public(),
		SymmetricAlgorithms: []ohttp.SymmetricAlgorithm{{KDFID: hpke.KDF_HKDF_SHA256, AEADID: hpke.AEAD_AES128GCM}}}
	adapter, err := ohttp.NewGateway(ohttp.KeyPair{SecretKey: secret, KeyConfig: keyConfig})
	if err != nil {
		return closeOnError(err)
	}
	issuer := &Issuer{config: issuerConfig, nodePublic: nodePublic, profile: profile, profileDigest: profileDigest, scope: scope,
		find: ledger.FindTransitGrantReservation, reserve: ledger.ReserveTransitGrant,
		withdraw: ledger.WithdrawTransitGrantIssuer, close: ledger.Close}
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
