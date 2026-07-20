package transport

import (
	"fmt"
	"time"

	"golang.org/x/time/rate"
)

const (
	defaultMaxMessageBytes         = 140 * 1024
	defaultMaxPeerConnections      = 64
	defaultMaxConnectionsPerIP     = 4
	defaultMaxConcurrentOperations = 16
	defaultOperationRate           = 20
	defaultOperationBurst          = 40
	defaultMaxFilterSubscribers    = 32
	defaultMaxStoreResults         = 128
	providerFailureThreshold       = 3
	providerBanDuration            = 30 * time.Second
)

type Limits struct {
	MaxMessageBytes         int
	MaxPeerConnections      int
	MaxConnectionsPerIP     int
	MaxConcurrentOperations int
	OperationRate           int
	OperationBurst          int
	MaxFilterSubscribers    int
	MaxStoreResults         int
}

type AbuseSnapshot struct {
	State                   string
	Reason                  string
	RateLimitedOperations   uint64
	BackpressuredOperations uint64
	OversizedMessages       uint64
	BannedProviders         int
	Limits                  Limits
}

type providerPenalty struct {
	failures    int
	bannedUntil time.Time
}

func normalizeLimits(in Limits) Limits {
	defaults := Limits{
		MaxMessageBytes: defaultMaxMessageBytes, MaxPeerConnections: defaultMaxPeerConnections,
		MaxConnectionsPerIP: defaultMaxConnectionsPerIP, MaxConcurrentOperations: defaultMaxConcurrentOperations,
		OperationRate: defaultOperationRate, OperationBurst: defaultOperationBurst,
		MaxFilterSubscribers: defaultMaxFilterSubscribers, MaxStoreResults: defaultMaxStoreResults,
	}
	if in.MaxMessageBytes > 0 {
		defaults.MaxMessageBytes = in.MaxMessageBytes
	}
	if in.MaxPeerConnections > 0 {
		defaults.MaxPeerConnections = in.MaxPeerConnections
	}
	if in.MaxConnectionsPerIP > 0 {
		defaults.MaxConnectionsPerIP = in.MaxConnectionsPerIP
	}
	if in.MaxConcurrentOperations > 0 {
		defaults.MaxConcurrentOperations = in.MaxConcurrentOperations
	}
	if in.OperationRate > 0 {
		defaults.OperationRate = in.OperationRate
	}
	if in.OperationBurst > 0 {
		defaults.OperationBurst = in.OperationBurst
	}
	if in.MaxFilterSubscribers > 0 {
		defaults.MaxFilterSubscribers = in.MaxFilterSubscribers
	}
	if in.MaxStoreResults > 0 {
		defaults.MaxStoreResults = in.MaxStoreResults
	}
	return defaults
}

func validateLimits(in Limits) error {
	if in.MaxMessageBytes < 0 || in.MaxPeerConnections < 0 || in.MaxConnectionsPerIP < 0 ||
		in.MaxConcurrentOperations < 0 || in.OperationRate < 0 || in.OperationBurst < 0 ||
		in.MaxFilterSubscribers < 0 || in.MaxStoreResults < 0 {
		return fmt.Errorf("network limits cannot be negative")
	}
	limits := normalizeLimits(in)
	if limits.MaxMessageBytes > 150*1024 || limits.MaxMessageBytes < 1024 {
		return fmt.Errorf("maximum message bytes must be between 1024 and 153600")
	}
	if limits.MaxPeerConnections > 256 || limits.MaxConnectionsPerIP > limits.MaxPeerConnections {
		return fmt.Errorf("peer connection limits are invalid")
	}
	if limits.MaxConcurrentOperations > 128 || limits.OperationRate > 1000 || limits.OperationBurst > 2000 {
		return fmt.Errorf("network operation limits are invalid")
	}
	if limits.MaxFilterSubscribers > 256 || limits.MaxStoreResults > 1024 {
		return fmt.Errorf("Filter or Store limits are invalid")
	}
	return nil
}

func (s *Service) initializeLimits() {
	s.cfg.Limits = normalizeLimits(s.cfg.Limits)
	s.operationSlots = make(chan struct{}, s.cfg.Limits.MaxConcurrentOperations)
	s.operationRate = rate.NewLimiter(rate.Limit(s.cfg.Limits.OperationRate), s.cfg.Limits.OperationBurst)
	s.providerPenalties = make(map[string]providerPenalty)
}
