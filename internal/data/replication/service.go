package replication

import (
	"context"
	"crypto/ed25519"
	"time"

	"ardents/internal/data/transfer"
	"ardents/internal/diagnostics"
	"ardents/internal/discovery"
	identityapi "ardents/internal/identity/api"
)

type Config struct {
	LocalNodeID          string
	Data                 DataService
	Policy               PolicyService
	Discovery            *discovery.Service
	Trust                *discovery.TrustEvaluator
	Exchange             *transfer.PrivateExchange
	Diagnostics          *diagnostics.Recorder
	Identity             identityapi.Service
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
	if s == nil || s.cfg.LocalNodeID == "" || s.cfg.Data == nil || s.cfg.Policy == nil ||
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
			if err := s.handle(ctx, message); err != nil && s.cfg.Diagnostics != nil {
				s.cfg.Diagnostics.RecordEvent("data", "replica_control_rejected", message.OperationID, safeReason(err), "", nil)
			}
		}
	}
}
