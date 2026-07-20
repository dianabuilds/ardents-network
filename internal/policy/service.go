package policy

import (
	"sync"
	"time"

	dataapi "ardents/internal/data/api"
	hostingservice "ardents/internal/hosting/service"
	identityapi "ardents/internal/identity/api"
	transport "ardents/internal/network/api"
	nodeapi "ardents/internal/node/api"
	policyapi "ardents/internal/policy/api"
	"ardents/internal/policy/decision"
	"ardents/internal/policy/enforcement"
	"ardents/internal/policy/evaluation"
	"ardents/internal/workload/observedstate"
	domainworkload "ardents/internal/workload/workload"
)

var _ policyapi.Service = (*Service)(nil)

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

func (s *Service) AdmitWorkload(spec domainworkload.Spec, existing []observedstate.Status) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := evaluation.CheckWorkload(evaluation.WorkloadConfig{
		MaxWorkloads:       s.cfg.MaxWorkloads,
		AllowedPolicyRefs:  s.cfg.AllowedPolicyRefs,
		DeniedCapabilities: s.cfg.DeniedCapabilities,
	}, spec, existing)
	return s.applyDecisionLocked(result)
}

func (s *Service) AllowServicePublication(spec hostingservice.Spec) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := evaluation.CheckServicePublication(evaluation.ServicePublicationConfig{
		DisableServicePublication:       s.cfg.DisableServicePublication,
		DisableNetworkPublishedServices: s.cfg.DisableNetworkPublishedServices,
		DeniedServiceTypes:              s.cfg.DeniedServiceTypes,
	}, spec)
	return s.applyDecisionLocked(result)
}

func (s *Service) AllowBlobRetention(blob dataapi.BlobSnapshot, relay bool, expiresAt time.Time, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := evaluation.CheckRetention(evaluation.RetentionConfig{
		DisableLocalBlobRetention: s.cfg.DisableLocalBlobRetention,
		DisableRelayBlobRetention: s.cfg.DisableRelayBlobRetention,
		MaxLocalRetentionTTL:      s.cfg.MaxLocalRetentionTTL,
		MaxRelayRetentionTTL:      s.cfg.MaxRelayRetentionTTL,
	}, blob, relay, expiresAt, now)
	return s.applyDecisionLocked(result)
}

func (s *Service) AllowBlobPin(blob dataapi.BlobSnapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := evaluation.CheckPin(evaluation.RetentionConfig{
		DisableBlobPinning:         s.cfg.DisableBlobPinning,
		AllowPinRelayRetainedBlobs: s.cfg.AllowPinRelayRetainedBlobs,
	}, blob)
	return s.applyDecisionLocked(result)
}

func (s *Service) AllowPeerBlobReserving(blob dataapi.BlobSnapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := evaluation.CheckPeerReserving(evaluation.RetentionConfig{
		DisablePeerBlobReserving: s.cfg.DisablePeerBlobReserving,
		AllowReservingRelayBlobs: s.cfg.AllowReservingRelayBlobs,
	}, blob)
	return s.applyDecisionLocked(result)
}

func (s *Service) AllowReplicaBlobServing(blob dataapi.BlobSnapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := evaluation.CheckPeerReserving(evaluation.RetentionConfig{
		DisablePeerBlobReserving: s.cfg.DisablePeerBlobReserving,
		AllowReservingRelayBlobs: true,
	}, blob)
	return s.applyDecisionLocked(result)
}

func (s *Service) AllowRouteUse(candidate transport.Candidate) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := evaluation.CheckRouteUse(evaluation.RouteConfig{
		DisableUntrustedRouteUse: s.cfg.DisableUntrustedRouteUse,
		DeniedRouteSchemes:       s.cfg.DeniedRouteSchemes,
	}, candidate)
	return s.applyDecisionLocked(result)
}

func (s *Service) AllowCapabilityUse(use identityapi.CapabilityUse) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := evaluation.CheckCapabilityUse(evaluation.CapabilityConfig{
		DisablePrivateCapabilityUse: s.cfg.DisablePrivateCapabilityUse,
		DeniedCapabilityScopes:      s.cfg.DeniedCapabilityScopes,
	}, use)
	return s.applyDecisionLocked(result)
}

func (s *Service) Snapshot() nodeapi.PartSnapshot {
	return enforcement.Snapshot(s.State(), s.Reason())
}

func (s *Service) applyDecisionLocked(result decision.Result) error {
	if result.Allowed {
		return nil
	}
	s.state = "enforced"
	s.reason = result.Reason.Message
	return result.Error()
}
