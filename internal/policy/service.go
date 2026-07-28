package policy

import (
	"ardents/internal/content"
	"sync"
	"time"

	identityapi "ardents/internal/identity"
	transport "ardents/internal/network"
	"ardents/internal/workload/execution"
	"ardents/internal/workload/registry"
)

var _ Policy = (*Service)(nil)

type Service struct {
	mu     sync.Mutex
	cfg    Config
	state  string
	reason string
}

func New(cfg Config) *Service {
	return &Service{cfg: normalizeConfig(cfg), state: "ready"}
}

func (s *Service) Reconfigure(cfg Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg = normalizeConfig(cfg)
	s.state = "ready"
	s.reason = ""
}

func (s *Service) State() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

func (s *Service) Reason() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reason
}

func (s *Service) AdmitWorkload(spec registry.Spec, existing []execution.Status) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := CheckWorkload(WorkloadConfig{
		MaxWorkloads:               s.cfg.MaxWorkloads,
		AllowedPolicyRefs:          s.cfg.AllowedPolicyRefs,
		DeniedWorkloadRequirements: s.cfg.DeniedWorkloadRequirements,
	}, spec, existing)
	return s.applyDecisionLocked(result)
}

func (s *Service) AllowServicePublication(spec registry.ServiceSpec) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := CheckServicePublication(ServicePublicationConfig{
		DisableServicePublication:       s.cfg.DisableServicePublication,
		DisableNetworkPublishedServices: s.cfg.DisableNetworkPublishedServices,
		DeniedServiceTypes:              s.cfg.DeniedServiceTypes,
	}, spec)
	return s.applyDecisionLocked(result)
}

func (s *Service) AllowBlobRetention(blob content.BlobPolicyView, relay bool, expiresAt time.Time, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := CheckRetention(RetentionConfig{
		DisableLocalBlobRetention: s.cfg.DisableLocalBlobRetention,
		DisableRelayBlobRetention: s.cfg.DisableRelayBlobRetention,
		MaxLocalRetentionTTL:      s.cfg.MaxLocalRetentionTTL,
		MaxRelayRetentionTTL:      s.cfg.MaxRelayRetentionTTL,
	}, retentionBlobView(blob), relay, expiresAt, now)
	return s.applyDecisionLocked(result)
}

func (s *Service) AllowBlobPin(blob content.BlobPolicyView) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := CheckPin(RetentionConfig{
		DisableBlobPinning:         s.cfg.DisableBlobPinning,
		AllowPinRelayRetainedBlobs: s.cfg.AllowPinRelayRetainedBlobs,
	}, retentionBlobView(blob))
	return s.applyDecisionLocked(result)
}

func (s *Service) AllowPeerBlobReserving(blob content.BlobPolicyView) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := CheckPeerReserving(RetentionConfig{
		DisablePeerBlobReserving: s.cfg.DisablePeerBlobReserving,
		AllowReservingRelayBlobs: s.cfg.AllowReservingRelayBlobs,
	}, retentionBlobView(blob))
	return s.applyDecisionLocked(result)
}

func (s *Service) AllowReplicaBlobServing(blob content.BlobPolicyView) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := CheckPeerReserving(RetentionConfig{
		DisablePeerBlobReserving: s.cfg.DisablePeerBlobReserving,
		AllowReservingRelayBlobs: true,
	}, retentionBlobView(blob))
	return s.applyDecisionLocked(result)
}

func retentionBlobView(blob content.BlobPolicyView) content.BlobPolicyView {
	return content.BlobPolicyView{State: blob.State, Retention: blob.Retention, Encrypted: blob.Encrypted}
}

func (s *Service) AllowRouteUse(candidate transport.Candidate) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := CheckRouteUse(RouteConfig{
		DisableUntrustedRouteUse: s.cfg.DisableUntrustedRouteUse,
		DeniedRouteSchemes:       s.cfg.DeniedRouteSchemes,
	}, candidate)
	return s.applyDecisionLocked(result)
}

func (s *Service) AllowCapabilityUse(use identityapi.CapabilityUse) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := CheckChannelGrantUse(ChannelGrantPolicyConfig{
		DisablePrivateChannelGrantUse: s.cfg.DisablePrivateChannelGrantUse,
		DeniedChannelGrantScopes:      s.cfg.DeniedChannelGrantScopes,
	}, use)
	return s.applyDecisionLocked(result)
}

func (s *Service) AllowRealmChannelClass(scope identityapi.CapabilityScope) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !ContainsNormalized(s.cfg.DeniedChannelGrantScopes, string(scope)) {
		return nil
	}
	return s.applyDecisionLocked(Deny(
		"policy_realm_channel_class_denied",
		"Realm channel class is denied by policy",
	))
}

// AllowRealmAuthorityCreation is the Product Policy gate for the sole v1
// authority-instance genesis mutation.
func (s *Service) AllowRealmAuthorityCreation() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.cfg.DisableRealmAuthorityCreation {
		return nil
	}
	return s.applyDecisionLocked(Deny("policy_realm_authority_denied", "Realm Authority creation is disabled by policy"))
}

// AllowRealmAuthorityRecovery gates explicit recovery-only verification. The
// authority service still performs all cryptographic and persistence checks.
func (s *Service) AllowRealmAuthorityRecovery() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.cfg.DisableRealmAuthorityRecovery {
		return nil
	}
	return s.applyDecisionLocked(Deny(
		"policy_realm_authority_recovery_denied",
		"Realm Authority recovery is disabled by policy",
	))
}

// AllowRealmChannelDelivery is the Product Policy gate for recipient-bound
// generation issue and acknowledgement mutations.
func (s *Service) AllowRealmChannelDelivery() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.cfg.DisableRealmChannelDelivery {
		return nil
	}
	return s.applyDecisionLocked(Deny("policy_realm_channel_delivery_denied", "Realm channel delivery is disabled by policy"))
}

func (s *Service) AllowRealmChannelRotation() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.cfg.DisableRealmChannelRotation {
		return nil
	}
	return s.applyDecisionLocked(Deny(
		"policy_realm_channel_rotation_denied",
		"Realm channel rotation is disabled by policy",
	))
}

func (s *Service) AllowRealmChannelMembership() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.cfg.DisableRealmChannelMembership {
		return nil
	}
	return s.applyDecisionLocked(Deny(
		"policy_realm_channel_membership_denied",
		"Realm channel membership change is disabled by policy",
	))
}

func (s *Service) Snapshot() Snapshot {
	return Snapshot{State: s.State(), Reason: s.Reason()}
}

func (s *Service) applyDecisionLocked(result Result) error {
	if result.Allowed {
		return nil
	}
	s.state = "enforced"
	s.reason = result.Reason.Message
	return result.Error()
}
