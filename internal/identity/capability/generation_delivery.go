package capability

import (
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/hpke"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	identityapi "ardents/internal/identity"
	"ardents/internal/storage"
)

const (
	DeliveryPhaseInstalled         = "installed"
	DeliveryPhaseActive            = "active"
	MaximumGenerationEnvelopeBytes = 256 << 10
	maxGenerationSenderGrants      = 256
	maxGenerationRevocations       = 256
	maxGenerationArtifactLife      = 30 * 24 * time.Hour
	generationBundleDomain         = "ardents:generation-bundle:v1\x00"
	generationEnvelopeInfoDomain   = "ardents:generation-delivery:v1\x00"
	generationReceiptDomain        = "ardents:generation-receipt:v1\x00"
)

var (
	generationRealmPattern     = regexp.MustCompile(`^r1_[0-9a-f]{32}$`)
	generationOperationPattern = regexp.MustCompile(`^rao1_[0-9a-f]{32}$`)
	generationDeliveryPattern  = regexp.MustCompile(`^rad1_[0-9a-f]{32}$`)
	generationDigestPattern    = regexp.MustCompile(`^ade1_[0-9a-f]{64}$`)
	generationKeyDigestPattern = regexp.MustCompile(`^adk1_[0-9a-f]{64}$`)
)

type GenerationBundle struct {
	Version            uint32
	RealmID            string
	AuthorityPrincipal string
	AuthorityEpoch     uint64
	AuthoritySequence  uint64
	OperationID        string
	DeliveryID         string
	ChannelID          [16]byte
	ChannelClass       identityapi.CapabilityScope
	Generation         uint32
	RecipientPrincipal string
	DeliveryKeyDigest  string
	SubjectGrant       identityapi.CapabilityGrant
	SenderGrants       []identityapi.CapabilityGrant
	Revocations        []identityapi.CapabilityRevocation
	ActivationPhase    string
	DrainDeadline      time.Time
	ExpiresAt          time.Time
	ReceiptKey         []byte
	AuthoritySignature []byte
}

func (GenerationBundle) String() string   { return "generation-bundle[redacted]" }
func (GenerationBundle) GoString() string { return "generation-bundle[redacted]" }
func (GenerationBundle) MarshalJSON() ([]byte, error) {
	return []byte(`{"redacted":"generation-bundle"}`), nil
}

type GenerationDeliveryBinding struct {
	Version            uint32                      `json:"version"`
	RealmID            string                      `json:"realm_id"`
	AuthorityPrincipal string                      `json:"authority_principal"`
	AuthorityEpoch     uint64                      `json:"authority_epoch"`
	AuthoritySequence  uint64                      `json:"authority_sequence"`
	OperationID        string                      `json:"operation_id"`
	DeliveryID         string                      `json:"delivery_id"`
	ChannelID          [16]byte                    `json:"channel_id"`
	ChannelClass       identityapi.CapabilityScope `json:"channel_class"`
	Generation         uint32                      `json:"generation"`
	RecipientPrincipal string                      `json:"recipient_principal"`
	DeliveryKeyDigest  string                      `json:"delivery_key_digest"`
	ExpiresAt          time.Time                   `json:"expires_at"`
}

type SealedGenerationDelivery struct {
	Binding        GenerationDeliveryBinding `json:"binding"`
	Envelope       []byte                    `json:"envelope"`
	EnvelopeDigest string                    `json:"envelope_digest"`
}

func (SealedGenerationDelivery) String() string   { return "sealed-generation-delivery[redacted]" }
func (SealedGenerationDelivery) GoString() string { return "sealed-generation-delivery[redacted]" }

type GenerationDeliveryReceipt struct {
	Version            uint32                      `json:"version"`
	RealmID            string                      `json:"realm_id"`
	AuthorityPrincipal string                      `json:"authority_principal"`
	AuthorityEpoch     uint64                      `json:"authority_epoch"`
	OperationID        string                      `json:"operation_id"`
	DeliveryID         string                      `json:"delivery_id"`
	EnvelopeDigest     string                      `json:"envelope_digest"`
	AuthoritySequence  uint64                      `json:"authority_sequence"`
	ChannelID          [16]byte                    `json:"channel_id"`
	ChannelClass       identityapi.CapabilityScope `json:"channel_class"`
	Generation         uint32                      `json:"generation"`
	RecipientPrincipal string                      `json:"recipient_principal"`
	DeliveryKeyDigest  string                      `json:"delivery_key_digest"`
	Phase              string                      `json:"phase"`
	CreatedAt          time.Time                   `json:"created_at"`
	MAC                []byte                      `json:"mac"`
}

type generationBundleRecord struct {
	Version            uint32                      `json:"version"`
	RealmID            string                      `json:"realm_id"`
	AuthorityPrincipal string                      `json:"authority_principal"`
	AuthorityEpoch     uint64                      `json:"authority_epoch"`
	AuthoritySequence  uint64                      `json:"authority_sequence"`
	OperationID        string                      `json:"operation_id"`
	DeliveryID         string                      `json:"delivery_id"`
	ChannelID          [16]byte                    `json:"channel_id"`
	ChannelClass       identityapi.CapabilityScope `json:"channel_class"`
	Generation         uint32                      `json:"generation"`
	RecipientPrincipal string                      `json:"recipient_principal"`
	DeliveryKeyDigest  string                      `json:"delivery_key_digest"`
	SubjectGrant       persistedGrant              `json:"subject_grant"`
	SenderGrants       []persistedGrant            `json:"sender_grants"`
	Revocations        []persistedRevocation       `json:"revocations"`
	ActivationPhase    string                      `json:"activation_phase"`
	DrainDeadline      time.Time                   `json:"drain_deadline"`
	ExpiresAt          time.Time                   `json:"expires_at"`
	ReceiptKey         []byte                      `json:"receipt_key"`
	AuthoritySignature []byte                      `json:"authority_signature,omitempty"`
}

func SealGenerationBundleForRecipient(
	bundle GenerationBundle,
	attestation identityapi.CapabilityDeliveryAttestation,
	at time.Time,
	sign func([]byte) ([]byte, error),
) (SealedGenerationDelivery, error) {
	at = at.UTC().Truncate(time.Second)
	if sign == nil {
		return SealedGenerationDelivery{}, fmt.Errorf("generation bundle signer is required")
	}
	if err := VerifyDeliveryAttestation(attestation, at); err != nil {
		return SealedGenerationDelivery{}, err
	}
	if attestation.SubjectPrincipal != bundle.RecipientPrincipal {
		return SealedGenerationDelivery{}, fmt.Errorf("generation delivery recipient mismatch")
	}
	record := persistGenerationBundle(bundle)
	if record.DeliveryKeyDigest != DeliveryPublicKeyDigest(attestation.DeliveryPublicKey) {
		return SealedGenerationDelivery{}, fmt.Errorf("generation delivery key binding mismatch")
	}
	defer clear(record.ReceiptKey)
	if err := validateGenerationBundleRecord(record, at, false); err != nil {
		return SealedGenerationDelivery{}, err
	}
	digest, err := generationBundleDigest(record)
	if err != nil {
		return SealedGenerationDelivery{}, err
	}
	signature, err := sign(digest)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return SealedGenerationDelivery{}, fmt.Errorf("generation bundle signing failed")
	}
	record.AuthoritySignature = append([]byte(nil), signature...)
	plain, err := json.Marshal(record)
	if err != nil {
		return SealedGenerationDelivery{}, err
	}
	defer clear(plain)
	kem := hpke.DHKEM(ecdh.X25519())
	public, err := kem.NewPublicKey(attestation.DeliveryPublicKey)
	if err != nil {
		return SealedGenerationDelivery{}, fmt.Errorf("generation delivery public key is invalid")
	}
	binding := bindingFromRecord(record)
	info, err := generationDeliveryInfo(binding)
	if err != nil {
		return SealedGenerationDelivery{}, err
	}
	envelope, err := hpke.Seal(
		public, hpke.HKDFSHA256(), hpke.ChaCha20Poly1305(), info, plain,
	)
	if err != nil {
		return SealedGenerationDelivery{}, fmt.Errorf("generation delivery sealing failed")
	}
	if len(envelope) > MaximumGenerationEnvelopeBytes {
		return SealedGenerationDelivery{}, fmt.Errorf("generation delivery envelope is too large")
	}
	return SealedGenerationDelivery{
		Binding: binding, Envelope: envelope, EnvelopeDigest: generationEnvelopeDigest(envelope),
	}, nil
}

func (s *Service) InstallGenerationDelivery(sealed SealedGenerationDelivery) (GenerationDeliveryReceipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := validateSealedGenerationDelivery(sealed); err != nil {
		return GenerationDeliveryReceipt{}, capabilityError(CodeInvalid, err.Error())
	}
	now := s.clock().UTC().Truncate(time.Second)
	if !now.Before(sealed.Binding.ExpiresAt) {
		return GenerationDeliveryReceipt{}, capabilityError(CodeExpired, "generation delivery is expired")
	}
	if retained, ok := s.ledger.InstalledDeliveries[sealed.Binding.DeliveryID]; ok {
		receipt := retained.restore()
		if !receiptMatchesSealed(receipt, retained.ExpiresAt, sealed) {
			return GenerationDeliveryReceipt{}, capabilityError(CodeInvalid, "delivery identity conflicts with retained envelope")
		}
		return receipt, nil
	}
	private, err := s.deliveryPrivateKeyLocked()
	if err != nil {
		return GenerationDeliveryReceipt{}, err
	}
	info, err := generationDeliveryInfo(sealed.Binding)
	if err != nil {
		return GenerationDeliveryReceipt{}, capabilityError(CodeInvalid, err.Error())
	}
	plain, err := hpke.Open(
		private, hpke.HKDFSHA256(), hpke.ChaCha20Poly1305(), info, sealed.Envelope,
	)
	if err != nil {
		return GenerationDeliveryReceipt{}, capabilityError(CodeInvalid, "generation delivery authentication failed")
	}
	defer clear(plain)
	var record generationBundleRecord
	if err := storage.DecodeJSONStrict(plain, &record); err != nil {
		return GenerationDeliveryReceipt{}, capabilityError(CodeInvalid, "generation delivery decode failed")
	}
	if bindingFromRecord(record) != sealed.Binding {
		return GenerationDeliveryReceipt{}, capabilityError(CodeInvalid, "generation delivery binding mismatch")
	}
	if err := s.validateGenerationBundle(record, now); err != nil {
		return GenerationDeliveryReceipt{}, capabilityError(CodeInvalid, err.Error())
	}
	defer clear(record.ReceiptKey)
	receipt := GenerationDeliveryReceipt{
		Version: 1, RealmID: record.RealmID, AuthorityPrincipal: record.AuthorityPrincipal,
		AuthorityEpoch: record.AuthorityEpoch, OperationID: record.OperationID,
		DeliveryID: record.DeliveryID, EnvelopeDigest: sealed.EnvelopeDigest,
		AuthoritySequence: record.AuthoritySequence, ChannelID: record.ChannelID,
		ChannelClass: record.ChannelClass, Generation: record.Generation,
		RecipientPrincipal: record.RecipientPrincipal, DeliveryKeyDigest: record.DeliveryKeyDigest,
		Phase: DeliveryPhaseInstalled, CreatedAt: now,
	}
	receipt, err = AuthenticateGenerationDeliveryReceipt(receipt, record.ReceiptKey)
	if err != nil {
		return GenerationDeliveryReceipt{}, capabilityError(CodeInvalid, "generation delivery receipt creation failed")
	}
	next := cloneLedger(s.ledger)
	ref := s.reference(mustRestoreGrant(record.SubjectGrant))
	next.Grants[string(ref)] = record.SubjectGrant
	for _, sender := range record.SenderGrants {
		next.SenderGrants[grantIDKey(sender.GrantID)] = sender
	}
	for _, revocation := range record.Revocations {
		next.Revocations[grantIDKey(revocation.GrantID)] = revocation
	}
	next.InstalledDeliveries[record.DeliveryID] = persistDeliveryReceipt(receipt, record.ExpiresAt)
	if err := s.store.save(next); err != nil {
		return GenerationDeliveryReceipt{}, err
	}
	if s.installAfterCommit != nil {
		if err := s.installAfterCommit(); err != nil {
			return GenerationDeliveryReceipt{}, capabilityError(CodeUnavailable, "generation install interrupted after commit")
		}
	}
	s.ledger = next
	return receipt, nil
}

func (s *Service) validateGenerationBundle(record generationBundleRecord, at time.Time) error {
	if err := validateGenerationBundleRecord(record, at, true); err != nil {
		return err
	}
	issuerPublic, ok := s.trustedIssuer(record.AuthorityPrincipal)
	if !ok {
		return fmt.Errorf("generation bundle issuer is not trusted")
	}
	digest, err := generationBundleDigest(record)
	if err != nil || !ed25519.Verify(issuerPublic, digest, record.AuthoritySignature) {
		return fmt.Errorf("generation bundle signature is invalid")
	}
	subject, err := record.SubjectGrant.restore()
	if err != nil || validateGrant(subject, issuerPublic) != nil ||
		subject.SubjectPrincipal != s.localPrincipal ||
		subject.IssuerPrincipal != record.AuthorityPrincipal ||
		subject.ChannelID != record.ChannelID || subject.Generation != record.Generation ||
		subject.Scope != record.ChannelClass {
		return fmt.Errorf("generation subject grant is invalid")
	}
	seen := make(map[[16]byte]struct{}, len(record.SenderGrants))
	subjectPresent := false
	for _, stored := range record.SenderGrants {
		grant, err := stored.restore()
		if err != nil || validateGrant(grant, issuerPublic) != nil ||
			grant.IssuerPrincipal != record.AuthorityPrincipal ||
			grant.ChannelID != record.ChannelID || grant.Generation != record.Generation ||
			grant.Scope != record.ChannelClass {
			return fmt.Errorf("generation sender snapshot is invalid")
		}
		if _, duplicate := seen[grant.GrantID]; duplicate {
			return fmt.Errorf("generation sender snapshot is duplicated")
		}
		seen[grant.GrantID] = struct{}{}
		subjectPresent = subjectPresent || grant.GrantID == subject.GrantID
		if err := s.validateSenderGrantConflict(grant); err != nil {
			return err
		}
	}
	if !subjectPresent {
		return fmt.Errorf("generation sender snapshot omits subject")
	}
	if err := s.rejectConflictingGrant(s.reference(subject), subject); err != nil {
		return err
	}
	for _, stored := range record.Revocations {
		revocation := stored.restore()
		if validateRevocation(revocation, issuerPublic) != nil ||
			revocation.IssuerPrincipal != record.AuthorityPrincipal {
			return fmt.Errorf("generation revocation snapshot is invalid")
		}
		if err := s.checkRevocationIssuer(revocation); err != nil {
			return err
		}
	}
	return nil
}

func validateGenerationBundleRecord(record generationBundleRecord, at time.Time, requireSignature bool) error {
	if record.Version != 1 || !generationRealmPattern.MatchString(record.RealmID) ||
		!validPrincipal(record.AuthorityPrincipal) || record.AuthorityEpoch == 0 ||
		record.AuthoritySequence < 2 || !generationOperationPattern.MatchString(record.OperationID) ||
		!generationDeliveryPattern.MatchString(record.DeliveryID) ||
		zeroID(record.ChannelID) || !knownScope(record.ChannelClass) || record.Generation == 0 ||
		!validPrincipal(record.RecipientPrincipal) ||
		!generationKeyDigestPattern.MatchString(record.DeliveryKeyDigest) ||
		record.ActivationPhase != DeliveryPhaseInstalled ||
		len(record.ReceiptKey) != sha256.Size ||
		len(record.SenderGrants) == 0 || len(record.SenderGrants) > maxGenerationSenderGrants ||
		len(record.Revocations) > maxGenerationRevocations ||
		!canonicalDeliverySecond(record.DrainDeadline) ||
		!canonicalDeliverySecond(record.ExpiresAt) || !at.Before(record.ExpiresAt) ||
		record.ExpiresAt.Sub(at) > maxGenerationArtifactLife {
		return fmt.Errorf("generation bundle contract is invalid")
	}
	if requireSignature && len(record.AuthoritySignature) != ed25519.SignatureSize {
		return fmt.Errorf("generation bundle signature is invalid")
	}
	return nil
}

func validateSealedGenerationDelivery(sealed SealedGenerationDelivery) error {
	if sealed.Binding.Version != 1 || !generationRealmPattern.MatchString(sealed.Binding.RealmID) ||
		!validPrincipal(sealed.Binding.AuthorityPrincipal) || sealed.Binding.AuthorityEpoch == 0 ||
		sealed.Binding.AuthoritySequence < 2 ||
		!generationOperationPattern.MatchString(sealed.Binding.OperationID) ||
		!generationDeliveryPattern.MatchString(sealed.Binding.DeliveryID) ||
		zeroID(sealed.Binding.ChannelID) || !knownScope(sealed.Binding.ChannelClass) ||
		sealed.Binding.Generation == 0 || !validPrincipal(sealed.Binding.RecipientPrincipal) ||
		!generationKeyDigestPattern.MatchString(sealed.Binding.DeliveryKeyDigest) ||
		!canonicalDeliverySecond(sealed.Binding.ExpiresAt) ||
		len(sealed.Envelope) == 0 || len(sealed.Envelope) > MaximumGenerationEnvelopeBytes ||
		!generationDigestPattern.MatchString(sealed.EnvelopeDigest) ||
		sealed.EnvelopeDigest != generationEnvelopeDigest(sealed.Envelope) {
		return fmt.Errorf("sealed generation delivery is invalid")
	}
	return nil
}

func persistGenerationBundle(bundle GenerationBundle) generationBundleRecord {
	senders := make([]persistedGrant, len(bundle.SenderGrants))
	for index, grant := range bundle.SenderGrants {
		senders[index] = persistGrant(grant)
	}
	revocations := make([]persistedRevocation, len(bundle.Revocations))
	for index, revocation := range bundle.Revocations {
		revocations[index] = persistRevocation(revocation)
	}
	return generationBundleRecord{
		Version: bundle.Version, RealmID: bundle.RealmID,
		AuthorityPrincipal: bundle.AuthorityPrincipal,
		AuthorityEpoch:     bundle.AuthorityEpoch, AuthoritySequence: bundle.AuthoritySequence,
		OperationID: bundle.OperationID, DeliveryID: bundle.DeliveryID, ChannelID: bundle.ChannelID,
		ChannelClass: bundle.ChannelClass, Generation: bundle.Generation,
		RecipientPrincipal: bundle.RecipientPrincipal,
		DeliveryKeyDigest:  bundle.DeliveryKeyDigest,
		SubjectGrant:       persistGrant(bundle.SubjectGrant), SenderGrants: senders,
		Revocations: revocations, ActivationPhase: bundle.ActivationPhase,
		DrainDeadline: bundle.DrainDeadline, ExpiresAt: bundle.ExpiresAt,
		ReceiptKey:         append([]byte(nil), bundle.ReceiptKey...),
		AuthoritySignature: append([]byte(nil), bundle.AuthoritySignature...),
	}
}

func bindingFromRecord(record generationBundleRecord) GenerationDeliveryBinding {
	return GenerationDeliveryBinding{
		Version: record.Version, RealmID: record.RealmID,
		AuthorityPrincipal: record.AuthorityPrincipal,
		AuthorityEpoch:     record.AuthorityEpoch, AuthoritySequence: record.AuthoritySequence,
		OperationID: record.OperationID, DeliveryID: record.DeliveryID, ChannelID: record.ChannelID,
		ChannelClass: record.ChannelClass, Generation: record.Generation,
		RecipientPrincipal: record.RecipientPrincipal,
		DeliveryKeyDigest:  record.DeliveryKeyDigest, ExpiresAt: record.ExpiresAt,
	}
}

func generationBundleDigest(record generationBundleRecord) ([]byte, error) {
	unsigned := record
	unsigned.AuthoritySignature = nil
	raw, err := json.Marshal(unsigned)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(append([]byte(generationBundleDomain), raw...))
	return sum[:], nil
}

func generationDeliveryInfo(binding GenerationDeliveryBinding) ([]byte, error) {
	raw, err := json.Marshal(binding)
	if err != nil {
		return nil, err
	}
	return append([]byte(generationEnvelopeInfoDomain), raw...), nil
}

func generationEnvelopeDigest(envelope []byte) string {
	sum := sha256.Sum256(envelope)
	return "ade1_" + hex.EncodeToString(sum[:])
}

func DeliveryPublicKeyDigest(publicKey []byte) string {
	sum := sha256.Sum256(publicKey)
	return "adk1_" + hex.EncodeToString(sum[:])
}

func generationReceiptMAC(receipt GenerationDeliveryReceipt, key []byte) []byte {
	unsigned := receipt
	unsigned.MAC = nil
	raw, _ := json.Marshal(unsigned)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(generationReceiptDomain))
	mac.Write(raw)
	return mac.Sum(nil)
}

func VerifyGenerationDeliveryReceipt(receipt GenerationDeliveryReceipt, key []byte) error {
	if receipt.Version != 1 || !generationRealmPattern.MatchString(receipt.RealmID) ||
		!validPrincipal(receipt.AuthorityPrincipal) || receipt.AuthorityEpoch == 0 ||
		!generationOperationPattern.MatchString(receipt.OperationID) ||
		!generationDeliveryPattern.MatchString(receipt.DeliveryID) ||
		!generationDigestPattern.MatchString(receipt.EnvelopeDigest) ||
		receipt.AuthoritySequence < 2 || zeroID(receipt.ChannelID) ||
		!knownScope(receipt.ChannelClass) || receipt.Generation == 0 ||
		!validPrincipal(receipt.RecipientPrincipal) ||
		!generationKeyDigestPattern.MatchString(receipt.DeliveryKeyDigest) ||
		(receipt.Phase != DeliveryPhaseInstalled && receipt.Phase != DeliveryPhaseActive) ||
		!canonicalDeliverySecond(receipt.CreatedAt) ||
		len(receipt.MAC) != sha256.Size || len(key) != sha256.Size ||
		!hmac.Equal(receipt.MAC, generationReceiptMAC(receipt, key)) {
		return fmt.Errorf("generation delivery receipt is invalid")
	}
	return nil
}

// AuthenticateGenerationDeliveryReceipt computes the holder assertion used by
// the member installer. Receipt-key possession is not proof of honest storage.
func AuthenticateGenerationDeliveryReceipt(
	receipt GenerationDeliveryReceipt,
	key []byte,
) (GenerationDeliveryReceipt, error) {
	if len(key) != sha256.Size {
		return GenerationDeliveryReceipt{}, fmt.Errorf("generation delivery receipt key is invalid")
	}
	receipt.MAC = generationReceiptMAC(receipt, key)
	if err := VerifyGenerationDeliveryReceipt(receipt, key); err != nil {
		return GenerationDeliveryReceipt{}, err
	}
	return receipt, nil
}

func receiptMatchesSealed(
	receipt GenerationDeliveryReceipt,
	expiresAt time.Time,
	sealed SealedGenerationDelivery,
) bool {
	binding := sealed.Binding
	return receipt.RealmID == binding.RealmID &&
		receipt.AuthorityPrincipal == binding.AuthorityPrincipal &&
		receipt.AuthorityEpoch == binding.AuthorityEpoch &&
		receipt.AuthoritySequence == binding.AuthoritySequence &&
		receipt.OperationID == binding.OperationID &&
		receipt.DeliveryID == binding.DeliveryID &&
		receipt.EnvelopeDigest == sealed.EnvelopeDigest &&
		receipt.ChannelID == binding.ChannelID &&
		receipt.ChannelClass == binding.ChannelClass &&
		receipt.Generation == binding.Generation &&
		receipt.RecipientPrincipal == binding.RecipientPrincipal &&
		receipt.DeliveryKeyDigest == binding.DeliveryKeyDigest &&
		expiresAt.Equal(binding.ExpiresAt)
}

func canonicalDeliverySecond(value time.Time) bool {
	return !value.IsZero() && value.Location() == time.UTC && value.Nanosecond() == 0
}

func mustRestoreGrant(stored persistedGrant) identityapi.CapabilityGrant {
	grant, _ := stored.restore()
	return grant
}
