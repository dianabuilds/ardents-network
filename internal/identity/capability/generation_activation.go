package capability

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	identityapi "ardents/internal/identity"
)

const generationActivationDomain = "ardents:generation-activation:v1\x00"
const maximumRetainedActivationOperations = 4096

type GenerationActivation struct {
	Version            uint32                      `json:"version"`
	RealmID            string                      `json:"realm_id"`
	AuthorityPrincipal string                      `json:"authority_principal"`
	AuthorityEpoch     uint64                      `json:"authority_epoch"`
	AuthoritySequence  uint64                      `json:"authority_sequence"`
	OperationID        string                      `json:"operation_id"`
	ChannelID          [16]byte                    `json:"channel_id"`
	ChannelClass       identityapi.CapabilityScope `json:"channel_class"`
	PreviousGeneration uint32                      `json:"previous_generation"`
	Generation         uint32                      `json:"generation"`
	EffectiveAt        time.Time                   `json:"effective_at"`
	DrainDeadline      time.Time                   `json:"drain_deadline"`
	CheckpointDigest   string                      `json:"checkpoint_digest"`
	Signature          []byte                      `json:"signature"`
}

type GenerationReadiness struct {
	ChannelID          [16]byte
	CurrentGeneration  uint32
	PendingGeneration  uint32
	PreviousGeneration uint32
	PreviousDrainUntil time.Time
	CheckpointDigest   string
	Ready              bool
}

func SignGenerationActivationWith(
	activation GenerationActivation,
	sign func([]byte) ([]byte, error),
) (GenerationActivation, error) {
	if sign == nil {
		return GenerationActivation{}, fmt.Errorf("generation activation signer is required")
	}
	if err := validateGenerationActivation(activation, false); err != nil {
		return GenerationActivation{}, err
	}
	digest, err := generationActivationDigest(activation)
	if err != nil {
		return GenerationActivation{}, err
	}
	signature, err := sign(digest)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return GenerationActivation{}, fmt.Errorf("generation activation signing failed")
	}
	activation.Signature = append([]byte(nil), signature...)
	return activation, nil
}

func VerifyGenerationActivation(
	activation GenerationActivation,
	authorityPublic ed25519.PublicKey,
) error {
	if err := validateGenerationActivation(activation, true); err != nil {
		return err
	}
	digest, err := generationActivationDigest(activation)
	if err != nil || len(authorityPublic) != ed25519.PublicKeySize ||
		!ed25519.Verify(authorityPublic, digest, activation.Signature) {
		return fmt.Errorf("generation activation signature is invalid")
	}
	return nil
}

func (s *Service) ActivateGeneration(
	activation GenerationActivation,
) (GenerationDeliveryReceipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clock().UTC().Truncate(time.Second)
	if err := validateGenerationActivation(activation, true); err != nil {
		return GenerationDeliveryReceipt{}, capabilityError(CodeInvalid, err.Error())
	}
	issuer, ok := s.trustedIssuer(activation.AuthorityPrincipal)
	if !ok || VerifyGenerationActivation(activation, issuer) != nil {
		return GenerationDeliveryReceipt{}, capabilityError(CodeIssuerUntrusted, "generation activation issuer is not trusted")
	}
	channelKey := generationChannelKey(activation.ChannelID)
	if retained, exists := s.ledger.ActivatedOperations[activation.OperationID]; exists {
		if !activationsEqual(retained.Activation, activation) {
			return GenerationDeliveryReceipt{}, capabilityError(CodeInvalid, "generation activation conflicts with retained checkpoint")
		}
		return retained.Receipt.restore(), nil
	}
	if latest, exists := s.ledger.ActivatedGenerations[channelKey]; exists &&
		activation.Generation <= latest.Activation.Generation {
		return GenerationDeliveryReceipt{}, capabilityError(CodeInvalid, "generation activation is not monotonic")
	}
	if len(s.ledger.ActivatedOperations) >= maximumRetainedActivationOperations {
		return GenerationDeliveryReceipt{}, capabilityError(CodeUnavailable, "generation activation history is full")
	}
	pending, exists := s.ledger.PendingGenerations[channelKey]
	if !exists || pending.ChannelID != activation.ChannelID ||
		pending.ChannelClass != activation.ChannelClass ||
		pending.RealmID != activation.RealmID ||
		pending.AuthorityPrincipal != activation.AuthorityPrincipal ||
		pending.AuthorityEpoch != activation.AuthorityEpoch ||
		pending.OperationID != activation.OperationID ||
		pending.Generation != activation.Generation ||
		activation.PreviousGeneration+1 != pending.Generation ||
		activation.AuthoritySequence <= pending.AuthoritySequence ||
		!activation.DrainDeadline.Equal(pending.DrainDeadline) {
		return GenerationDeliveryReceipt{}, capabilityError(CodeInvalid, "generation activation does not match pending delivery")
	}
	if now.Before(activation.EffectiveAt) {
		return GenerationDeliveryReceipt{}, capabilityError(CodeNotYetValid, "generation activation is not effective")
	}
	if !now.Before(activation.DrainDeadline) || !now.Before(pending.ExpiresAt) {
		return GenerationDeliveryReceipt{}, capabilityError(CodeExpired, "generation activation is expired")
	}
	current, ok := s.ledger.Grants[pending.CurrentReference]
	if !ok || current.ChannelID != activation.ChannelID ||
		current.Generation != activation.PreviousGeneration {
		return GenerationDeliveryReceipt{}, capabilityError(CodeInvalid, "current generation does not match activation predecessor")
	}
	installed, ok := s.ledger.InstalledDeliveries[pending.DeliveryID]
	if !ok || installed.Phase != DeliveryPhaseInstalled {
		return GenerationDeliveryReceipt{}, capabilityError(CodeInvalid, "pending generation is not durably installed")
	}
	active := installed.restore()
	active.AuthoritySequence = activation.AuthoritySequence
	active.Phase = DeliveryPhaseActive
	active.CreatedAt = now
	active, err := AuthenticateGenerationDeliveryReceipt(active, pending.ReceiptKey)
	if err != nil {
		return GenerationDeliveryReceipt{}, capabilityError(CodeInvalid, "active receipt creation failed")
	}
	next := cloneLedger(s.ledger)
	next.PreviousGenerations[channelKey] = persistedPreviousGeneration{
		Reference: pending.CurrentReference, Grant: current,
		DrainDeadline: activation.DrainDeadline,
	}
	next.Grants[pending.CurrentReference] = pending.SubjectGrant
	for _, sender := range pending.SenderGrants {
		next.SenderGrants[grantIDKey(sender.GrantID)] = sender
	}
	for _, revocation := range pending.Revocations {
		next.Revocations[grantIDKey(revocation.GrantID)] = revocation
	}
	delete(next.PendingGenerations, channelKey)
	next.ActivatedGenerations[channelKey] = persistedActivatedGeneration{
		Activation:     activation,
		Receipt:        persistDeliveryReceipt(active, pending.ExpiresAt),
		RuntimeAdopted: false,
	}
	next.ActivatedOperations[activation.OperationID] = next.ActivatedGenerations[channelKey]
	if err := s.store.save(next); err != nil {
		return GenerationDeliveryReceipt{}, err
	}
	if s.activationAfterCommit != nil {
		if err := s.activationAfterCommit(); err != nil {
			return GenerationDeliveryReceipt{}, capabilityError(CodeUnavailable, "generation activation interrupted after commit")
		}
	}
	s.ledger = next
	return active, nil
}

func (s *Service) ConfirmGenerationRuntimeAdoption(
	activation GenerationActivation,
) (GenerationDeliveryReceipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := validateGenerationActivation(activation, true); err != nil {
		return GenerationDeliveryReceipt{}, capabilityError(CodeInvalid, err.Error())
	}
	issuer, ok := s.trustedIssuer(activation.AuthorityPrincipal)
	if !ok || VerifyGenerationActivation(activation, issuer) != nil {
		return GenerationDeliveryReceipt{}, capabilityError(CodeIssuerUntrusted, "generation activation issuer is not trusted")
	}
	retained, exists := s.ledger.ActivatedOperations[activation.OperationID]
	if !exists || !activationsEqual(retained.Activation, activation) {
		return GenerationDeliveryReceipt{}, capabilityError(CodeInvalid, "generation activation is not committed")
	}
	channelKey := generationChannelKey(activation.ChannelID)
	latest, exists := s.ledger.ActivatedGenerations[channelKey]
	if !exists || !activationsEqual(latest.Activation, activation) {
		return GenerationDeliveryReceipt{}, capabilityError(CodeInvalid, "generation activation is not current")
	}
	if latest.RuntimeAdopted {
		return latest.Receipt.restore(), nil
	}
	next := cloneLedger(s.ledger)
	latest.RuntimeAdopted = true
	retained.RuntimeAdopted = true
	next.ActivatedGenerations[channelKey] = latest
	next.ActivatedOperations[activation.OperationID] = retained
	if err := s.store.save(next); err != nil {
		return GenerationDeliveryReceipt{}, capabilityError(CodeUnavailable, "generation runtime adoption commit failed")
	}
	s.ledger = next
	return latest.Receipt.restore(), nil
}

func (s *Service) GenerationReadiness(channelID [16]byte) GenerationReadiness {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clock().UTC().Truncate(time.Second)
	status := GenerationReadiness{ChannelID: channelID}
	_, current, ok := currentSubjectGrant(s.ledger, channelID, s.localPrincipal)
	if ok {
		status.CurrentGeneration = current.Generation
		status.Ready = true
	}
	channelKey := generationChannelKey(channelID)
	if pending, exists := s.ledger.PendingGenerations[channelKey]; exists {
		status.PendingGeneration = pending.Generation
		status.Ready = false
	}
	if previous, exists := s.ledger.PreviousGenerations[channelKey]; exists &&
		now.Before(previous.DrainDeadline) {
		status.PreviousGeneration = previous.Grant.Generation
		status.PreviousDrainUntil = previous.DrainDeadline
	}
	if activated, exists := s.ledger.ActivatedGenerations[channelKey]; exists {
		status.CheckpointDigest = activated.Activation.CheckpointDigest
		if !activated.RuntimeAdopted {
			status.Ready = false
		}
	}
	return status
}

func (s *Service) ResolveCapabilityGeneration(
	use identityapi.CapabilityUse,
	generation uint32,
) (identityapi.ResolvedCapability, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if generation == 0 {
		return identityapi.ResolvedCapability{}, capabilityError(CodeInvalid, "capability generation is invalid")
	}
	at := use.At.UTC()
	if at.IsZero() {
		at = s.clock().UTC()
	}
	if current, ok := s.ledger.Grants[string(use.Ref)]; ok &&
		current.Generation == generation {
		grant, err := current.restore()
		if err != nil {
			return identityapi.ResolvedCapability{}, capabilityError(CodeInvalid, err.Error())
		}
		if err := s.authorize(grant, use, at); err != nil {
			return identityapi.ResolvedCapability{}, err
		}
		if err := s.admission.AllowCapabilityUse(use); err != nil {
			return identityapi.ResolvedCapability{}, capabilityError(CodeScopeDenied, "capability use denied by policy")
		}
		return resolved(use.Ref, grant), nil
	}
	if use.Permission == 0 ||
		use.Permission&^(identityapi.CapabilitySubscribe|identityapi.CapabilityStoreFetch) != 0 {
		return identityapi.ResolvedCapability{}, capabilityError(CodeScopeDenied, "previous generation is receive-only")
	}
	for _, previous := range s.ledger.PreviousGenerations {
		if previous.Reference != string(use.Ref) ||
			previous.Grant.Generation != generation ||
			!at.Before(previous.DrainDeadline) {
			continue
		}
		grant, err := previous.Grant.restore()
		if err != nil {
			return identityapi.ResolvedCapability{}, capabilityError(CodeInvalid, err.Error())
		}
		if err := s.authorize(grant, use, at); err != nil {
			return identityapi.ResolvedCapability{}, err
		}
		if err := s.admission.AllowCapabilityUse(use); err != nil {
			return identityapi.ResolvedCapability{}, capabilityError(CodeScopeDenied, "capability use denied by policy")
		}
		return resolved(use.Ref, grant), nil
	}
	return identityapi.ResolvedCapability{}, capabilityError(CodeMissing, "capability generation is unavailable")
}

func (s *Service) AvailableCapabilityGenerations(
	ref identityapi.CapabilityRef,
	subject string,
	scope identityapi.CapabilityScope,
	at time.Time,
) []uint32 {
	s.mu.Lock()
	defer s.mu.Unlock()
	at = at.UTC()
	if at.IsZero() {
		at = s.clock().UTC()
	}
	generations := make([]uint32, 0, 2)
	if current, ok := s.ledger.Grants[string(ref)]; ok &&
		current.SubjectPrincipal == subject && current.Scope == scope {
		generations = append(generations, current.Generation)
	}
	for _, previous := range s.ledger.PreviousGenerations {
		if previous.Reference == string(ref) &&
			previous.Grant.SubjectPrincipal == subject &&
			previous.Grant.Scope == scope &&
			at.Before(previous.DrainDeadline) {
			generations = append(generations, previous.Grant.Generation)
		}
	}
	sort.Slice(generations, func(left, right int) bool {
		return generations[left] > generations[right]
	})
	return generations
}

func validateGenerationActivation(activation GenerationActivation, requireSignature bool) error {
	if activation.Version != 1 ||
		!generationRealmPattern.MatchString(activation.RealmID) ||
		!validPrincipal(activation.AuthorityPrincipal) ||
		activation.AuthorityEpoch == 0 || activation.AuthoritySequence < 3 ||
		!generationOperationPattern.MatchString(activation.OperationID) ||
		zeroID(activation.ChannelID) || !knownScope(activation.ChannelClass) ||
		activation.PreviousGeneration == 0 ||
		activation.Generation != activation.PreviousGeneration+1 ||
		!canonicalDeliverySecond(activation.EffectiveAt) ||
		!canonicalDeliverySecond(activation.DrainDeadline) ||
		!activation.DrainDeadline.After(activation.EffectiveAt) ||
		!generationCheckpointDigestPattern.MatchString(activation.CheckpointDigest) {
		return fmt.Errorf("generation activation contract is invalid")
	}
	if requireSignature && len(activation.Signature) != ed25519.SignatureSize {
		return fmt.Errorf("generation activation signature is invalid")
	}
	return nil
}

func generationActivationDigest(activation GenerationActivation) ([]byte, error) {
	unsigned := activation
	unsigned.Signature = nil
	raw, err := json.Marshal(unsigned)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(append([]byte(generationActivationDomain), raw...))
	return sum[:], nil
}

func activationsEqual(left, right GenerationActivation) bool {
	leftRaw, leftErr := json.Marshal(left)
	rightRaw, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftRaw, rightRaw)
}
