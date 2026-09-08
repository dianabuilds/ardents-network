package credential

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"errors"
	"path/filepath"
	"sync"
	"time"

	"github.com/cloudflare/circl/blindsign/blindrsa"
	"github.com/dianabuilds/ardents-network/internal/network/state"
)

// ClosedTokenIssuerConfig opens one isolated issuer root only while the
// caller supplies its current accepted State projection. It accepts no
// caller-selected admission authority, Node, key, or validity callback.
type ClosedTokenIssuerConfig struct {
	Root           string
	NetworkID      [32]byte
	CurrentProfile func() (state.ClosedProfileView, bool)
	Clock          func() time.Time
}

// ClosedTokenIssuer owns the finite class/window RSA material and the
// exclusive durable reservation ledger for one closed State profile.
type ClosedTokenIssuer struct {
	mu       sync.Mutex
	root     string
	lease    issuerRootLease
	network  [32]byte
	profile  state.ClosedProfileView
	material closedIssuerMaterial
	ledger   *closedTokenIssuerLedger
	current  func() (state.ClosedProfileView, bool)
	clock    func() time.Time
	closed   bool
}

// OpenClosedTokenIssuer opens an initialized closed issuer root. A root can
// bind to only one accepted profile digest and cannot be silently reused for
// a successor duty or State epoch.
func OpenClosedTokenIssuer(config ClosedTokenIssuerConfig) (*ClosedTokenIssuer, error) {
	if config.Root == "" || config.NetworkID == [32]byte{} || config.CurrentProfile == nil || config.Clock == nil {
		return nil, errors.New("closed token issuer configuration is invalid")
	}
	now := config.Clock().UTC()
	profile, available := config.CurrentProfile()
	if !available || !validClosedTokenIssuerProfile(profile, now) {
		return nil, errors.New("closed token issuer State profile is unavailable")
	}
	root, lease, err := openClosedIssuerRoot(config.Root)
	if err != nil {
		return nil, err
	}
	fail := func(cause error) (*ClosedTokenIssuer, error) { return nil, errors.Join(cause, lease.release()) }
	raw, err := readIssuerFile(filepath.Join(root, closedIssuerMaterialName), maximumClosedIssuerMaterial)
	if err != nil {
		return fail(err)
	}
	material, err := decodeClosedIssuerMaterial(raw)
	clear(raw)
	if err != nil || material.network != config.NetworkID || material.node != profile.IssuerNodeID || !now.Before(material.notAfter) {
		return fail(errors.New("closed token issuer root does not match State"))
	}
	if !closedTokenIssuerKeysMatch(material, profile) {
		return fail(errors.New("closed token issuer keys do not match State"))
	}
	ledger, err := openClosedTokenIssuerLedger(root, config.NetworkID, profile.IssuerNodeID, profile.Digest)
	if err != nil {
		return fail(err)
	}
	return &ClosedTokenIssuer{root: root, lease: lease, network: config.NetworkID, profile: profile, material: material, ledger: ledger,
		current: config.CurrentProfile, clock: config.Clock}, nil
}

// Issue verifies cheap request facts before the durable whole-batch debit,
// then signs the already-reserved blinded elements. The deterministic selected
// blind RSA primitive lets an exact post-crash retry recompute its result.
func (issuer *ClosedTokenIssuer) Issue(raw []byte) ClosedTokenBatchResult {
	if issuer == nil {
		return ClosedTokenBatchResult{Status: ClosedTokenUnavailable}
	}
	issuer.mu.Lock()
	defer issuer.mu.Unlock()
	request, err := DecodeClosedTokenBatch(raw)
	if err != nil || issuer.closed || !issuer.profileCurrent() {
		return ClosedTokenBatchResult{Status: ClosedTokenUnavailable}
	}
	now := issuer.clock().UTC()
	if VerifyPermission(request.Permission, ed25519.PublicKey(issuer.profile.IssuanceAuthorityKey[:]), issuer.network,
		issuer.profile.IssuerNodeID, issuer.profile.IssuerDutyGeneration, now) != nil || request.WindowStart != request.Permission.NotBefore {
		return ClosedTokenBatchResult{Status: ClosedTokenUnavailable}
	}
	private, err := issuer.privateKey(request)
	if err != nil {
		return ClosedTokenBatchResult{Status: ClosedTokenUnavailable}
	}
	digest := sha256.Sum256(raw)
	if _, found, err := issuer.ledger.find(request.RequestID, digest); err != nil {
		return ClosedTokenBatchResult{Status: ClosedTokenUnavailable}
	} else if !found {
		reserved, reserveErr := issuer.ledger.reserve(request, digest)
		if reserveErr != nil {
			return ClosedTokenBatchResult{Status: ClosedTokenUnavailable}
		}
		if !reserved {
			return ClosedTokenBatchResult{Status: ClosedTokenExhausted}
		}
	}
	signer := blindrsa.NewSigner(private)
	result := ClosedTokenBatchResult{Status: ClosedTokenIssued, Signatures: make([][]byte, 0, len(request.BlindedRequests))}
	for _, blinded := range request.BlindedRequests {
		signature, err := signer.BlindSign(blinded[3:])
		if err != nil || len(signature) != closedTokenBlindElementSize {
			return ClosedTokenBatchResult{Status: ClosedTokenUnavailable}
		}
		result.Signatures = append(result.Signatures, signature)
	}
	return result
}

// Close releases the exclusive root lease and removes in-memory private-key
// references. The append-only reservation ledger remains the authority after
// a restart.
func (issuer *ClosedTokenIssuer) Close() error {
	if issuer == nil {
		return nil
	}
	issuer.mu.Lock()
	defer issuer.mu.Unlock()
	if issuer.closed {
		return nil
	}
	issuer.closed = true
	for index := range issuer.material.keys {
		clear(issuer.material.keys[index].der)
	}
	issuer.material.keys = nil
	return issuer.lease.release()
}

func (issuer *ClosedTokenIssuer) profileCurrent() bool {
	profile, available := issuer.current()
	now := issuer.clock().UTC()
	return available && profile.Digest == issuer.profile.Digest && profile.IssuerNodeID == issuer.profile.IssuerNodeID &&
		profile.IssuerDutyGeneration == issuer.profile.IssuerDutyGeneration && validClosedTokenIssuerProfile(profile, now)
}

func validClosedTokenIssuerProfile(profile state.ClosedProfileView, now time.Time) bool {
	return profile.Digest != [32]byte{} && profile.IssuanceAuthorityKey != [32]byte{} && profile.IssuerNodeID != [32]byte{} &&
		profile.IssuerDutyGeneration != 0 && profile.TokenKeyCount > 0 && int(profile.TokenKeyCount) <= len(profile.TokenKeys) &&
		!profile.NotBefore.IsZero() && !profile.NotAfter.IsZero() && !now.Before(profile.NotBefore) && now.Before(profile.NotAfter)
}

func closedTokenIssuerKeysMatch(material closedIssuerMaterial, profile state.ClosedProfileView) bool {
	keys, err := closedIssuerPublicKeys(material)
	if err != nil {
		return false
	}
	for index := 0; index < int(profile.TokenKeyCount); index++ {
		matched := false
		for _, key := range keys {
			if key.WindowStart == profile.TokenKeys[index].WindowStart && key.Class == byte(profile.TokenKeys[index].Class) &&
				bytes.Equal(key.SPKI, profile.TokenKeys[index].SPKI[:]) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func (issuer *ClosedTokenIssuer) privateKey(request ClosedTokenBatchRequest) (*rsa.PrivateKey, error) {
	for _, key := range issuer.material.keys {
		if key.window != request.WindowStart || key.class != request.Class {
			continue
		}
		private, err := x509.ParsePKCS1PrivateKey(key.der)
		if err != nil || private.Validate() != nil {
			return nil, errors.New("closed token issuer private key is invalid")
		}
		spki, err := encodeClosedIssuerSPKI(&private.PublicKey)
		if err != nil || !bytes.Equal(spki, request.SPKI[:]) {
			return nil, errors.New("closed token issuer requested key is unavailable")
		}
		return private, nil
	}
	return nil, errors.New("closed token issuer requested window is unavailable")
}
