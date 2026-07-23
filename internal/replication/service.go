package replication

import (
	"context"
	"crypto/ed25519"
	"errors"
	"strings"
	"time"

	"ardents/internal/discovery"
	identityprincipal "ardents/internal/identity/principal"
	"ardents/internal/transfer"
)

const reasonReplicaControlRejected = "replica_control_rejected"

var ErrDependencies = errors.New("replica control dependencies are unavailable")

func safeReason(err error) string {
	if err == nil {
		return ""
	}
	switch {
	case errors.Is(err, ErrDependencies):
		return "dependencies_unavailable"
	case strings.Contains(err.Error(), "quota") || strings.Contains(err.Error(), "byte limit"):
		return "quota_refused"
	case strings.Contains(err.Error(), "untrusted") || strings.Contains(err.Error(), "not trusted"):
		return "peer_untrusted"
	case strings.Contains(err.Error(), "policy"):
		return "policy_denied"
	case strings.Contains(err.Error(), "permission"):
		return "permission_denied"
	case strings.Contains(err.Error(), "expired") || strings.Contains(err.Error(), "lease"):
		return "lease_invalid"
	case strings.Contains(err.Error(), "content identity") || strings.Contains(err.Error(), "ciphertext"):
		return "content_invalid"
	case strings.Contains(err.Error(), "replay") || strings.Contains(err.Error(), "already"):
		return "operation_replayed"
	case strings.Contains(err.Error(), "unsupported") || strings.Contains(err.Error(), "requires chunking"):
		return "transfer_unsupported"
	default:
		return reasonReplicaControlRejected
	}
}

type Config struct {
	LocalNodePrincipal   identityprincipal.ID
	Data                 DataService
	Policy               PolicyService
	Discovery            *discovery.Service
	Trust                *discovery.TrustEvaluator
	Exchange             *transfer.PrivateExchange
	RecordEvent          func(string, string, string, string, string, map[string]any)
	Identity             func() transfer.IdentitySummary
	PrivateKey           func() ed25519.PrivateKey
	Now                  func() time.Time
	RepairInterval       time.Duration
	RepairConcurrency    int
	RepairAttemptTimeout time.Duration
}

type Service struct{ cfg Config }

func New(cfg Config) *Service {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.RepairInterval <= 0 {
		cfg.RepairInterval = 5 * time.Minute
	}
	if cfg.RepairConcurrency <= 0 || cfg.RepairConcurrency > 4 {
		cfg.RepairConcurrency = 2
	}
	if cfg.RepairAttemptTimeout <= 0 {
		cfg.RepairAttemptTimeout = 20 * time.Second
	}
	return &Service{cfg: cfg}
}

func (s *Service) Start(ctx context.Context) error {
	if s == nil || s.cfg.LocalNodePrincipal.String() == "" || s.cfg.Data == nil || s.cfg.Policy == nil ||
		s.cfg.Discovery == nil || s.cfg.Trust == nil || s.cfg.Exchange == nil || s.cfg.Identity == nil || s.cfg.PrivateKey == nil {
		return ErrDependencies
	}
	go s.receive(ctx)
	go s.reconcileLoop(ctx)
	return nil
}

func (s *Service) receive(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case message := <-s.cfg.Exchange.ReplicaRequests():
			if err := s.handle(ctx, message); err != nil && s.cfg.RecordEvent != nil {
				s.cfg.RecordEvent("data", "replica_control_rejected", message.OperationID, safeReason(err), "", nil)
			}
		}
	}
}
