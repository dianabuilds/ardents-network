package waku

import (
	"ardents/internal/network"
	"fmt"
	"os"
	"strings"
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
	defaultStoreMaxMessages        = 100000
	defaultStoreMaxAgeSeconds      = 7 * 24 * 60 * 60
	defaultStoreMaxBytes           = int64(2 << 30)
	providerFailureThreshold       = 3
	providerBanDuration            = 30 * time.Second
)

type providerPenalty struct {
	failures    int
	bannedUntil time.Time
}

func normalizeLimits(in network.Limits) network.Limits {
	defaults := network.Limits{
		MaxMessageBytes: defaultMaxMessageBytes, MaxPeerConnections: defaultMaxPeerConnections,
		MaxConnectionsPerIP: defaultMaxConnectionsPerIP, MaxConcurrentOperations: defaultMaxConcurrentOperations,
		OperationRate: defaultOperationRate, OperationBurst: defaultOperationBurst,
		MaxFilterSubscribers: defaultMaxFilterSubscribers, MaxStoreResults: defaultMaxStoreResults,
		StoreMaxMessages: defaultStoreMaxMessages, StoreMaxAgeSeconds: defaultStoreMaxAgeSeconds,
		StoreMaxBytes: defaultStoreMaxBytes,
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
	if in.StoreMaxMessages > 0 {
		defaults.StoreMaxMessages = in.StoreMaxMessages
	}
	if in.StoreMaxAgeSeconds > 0 {
		defaults.StoreMaxAgeSeconds = in.StoreMaxAgeSeconds
	}
	if in.StoreMaxBytes > 0 {
		defaults.StoreMaxBytes = in.StoreMaxBytes
	}
	return defaults
}

func validateLimits(in network.Limits) error {
	if in.MaxMessageBytes < 0 || in.MaxPeerConnections < 0 || in.MaxConnectionsPerIP < 0 ||
		in.MaxConcurrentOperations < 0 || in.OperationRate < 0 || in.OperationBurst < 0 ||
		in.MaxFilterSubscribers < 0 || in.MaxStoreResults < 0 ||
		in.StoreMaxMessages < 0 || in.StoreMaxAgeSeconds < 0 || in.StoreMaxBytes < 0 {
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
		return fmt.Errorf("filter or Store limits are invalid")
	}
	if limits.StoreMaxMessages < 1 || limits.StoreMaxAgeSeconds < 1 {
		return fmt.Errorf("persistent Store retention limits must be finite and positive")
	}
	if limits.StoreMaxBytes < 4<<20 {
		return fmt.Errorf("persistent Store byte limit must be at least 4194304")
	}
	if limits.StoreMaxMessages > 10_000_000 || limits.StoreMaxAgeSeconds > 10*365*24*60*60 {
		return fmt.Errorf("persistent Store retention limits exceed supported bounds")
	}
	if limits.StoreMaxBytes > 1<<40 {
		return fmt.Errorf("persistent Store byte limit exceeds supported bounds")
	}
	return nil
}

func (s *Service) initializeLimits() {
	s.cfg.Limits = normalizeLimits(s.cfg.Limits)
	s.operationSlots = make(chan struct{}, s.cfg.Limits.MaxConcurrentOperations)
	s.operationRate = rate.NewLimiter(rate.Limit(s.cfg.Limits.OperationRate), s.cfg.Limits.OperationBurst)
	s.providerPenalties = make(map[string]providerPenalty)
}

func (s *Service) acquireNetworkOperation(payloadBytes int, provider string) (func(error), error) {
	now := timeNowUTC()
	provider = strings.TrimSpace(provider)
	s.mu.Lock()
	if payloadBytes > s.cfg.Limits.MaxMessageBytes {
		s.abuse.OversizedMessages++
		s.mu.Unlock()
		return nil, fmt.Errorf("network message exceeds %d byte limit", s.cfg.Limits.MaxMessageBytes)
	}
	if penalty := s.providerPenalties[provider]; provider != "" && penalty.bannedUntil.After(now) {
		s.mu.Unlock()
		return nil, fmt.Errorf("network provider is temporarily banned until %s", penalty.bannedUntil.UTC().Format(time.RFC3339))
	}
	if s.operationRate != nil && !s.operationRate.Allow() {
		s.abuse.RateLimitedOperations++
		s.mu.Unlock()
		return nil, fmt.Errorf("network operation rate limit exceeded")
	}
	select {
	case s.operationSlots <- struct{}{}:
		s.mu.Unlock()
		return func(err error) { s.finishNetworkOperation(provider, err) }, nil
	default:
		s.abuse.BackpressuredOperations++
		s.mu.Unlock()
		return nil, fmt.Errorf("network operation concurrency limit exceeded")
	}
}

func (s *Service) finishNetworkOperation(provider string, operationErr error) {
	<-s.operationSlots
	if provider == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if operationErr == nil {
		delete(s.providerPenalties, provider)
		return
	}
	penalty := s.providerPenalties[provider]
	penalty.failures++
	if penalty.failures >= providerFailureThreshold {
		penalty.failures = 0
		penalty.bannedUntil = timeNowUTC().Add(providerBanDuration)
	}
	s.providerPenalties[provider] = penalty
}

func (s *Service) AbuseSnapshot() network.AbuseSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := timeNowUTC()
	snapshot := s.abuse
	snapshot.State = "ready"
	snapshot.Limits = s.cfg.Limits
	for provider, penalty := range s.providerPenalties {
		if !penalty.bannedUntil.After(now) {
			if penalty.failures == 0 {
				delete(s.providerPenalties, provider)
			}
			continue
		}
		snapshot.BannedProviders++
	}
	if snapshot.BannedProviders > 0 {
		snapshot.State = "degraded"
		snapshot.Reason = "one or more network providers are temporarily banned after repeated failures"
	}
	s.populateStorePressureLocked(&snapshot)
	return snapshot
}

const storePressureDegradedRatio = 0.90

func (s *Service) populateStorePressureLocked(snapshot *network.AbuseSnapshot) {
	provider := s.messageProvider
	if provider == nil {
		return
	}
	snapshot.StoreEnabled = true
	snapshot.StoreCapacityMessages = s.cfg.Limits.StoreMaxMessages
	snapshot.StoreCapacityBytes = s.cfg.Limits.StoreMaxBytes
	count, err := provider.Count()
	if err != nil {
		snapshot.State = "failed"
		snapshot.Reason = "Waku Store pressure cannot be measured"
		return
	}
	snapshot.StoreMessages = count
	if snapshot.StoreCapacityMessages > 0 {
		snapshot.StoreUsageRatio = float64(count) / float64(snapshot.StoreCapacityMessages)
	}
	if strings.TrimSpace(s.cfg.StorePath) == "" {
		if snapshot.StoreUsageRatio >= storePressureDegradedRatio {
			snapshot.State = "degraded"
			snapshot.Reason = "Waku Store retention capacity is under pressure"
		}
		return
	}
	disk, err := inspectStoreDisk(s.cfg.StorePath)
	if err != nil {
		snapshot.State = "failed"
		snapshot.Reason = "Waku Store file pressure cannot be measured"
		return
	}
	snapshot.StoreFileBytes = disk.total
	mainRatio := float64(disk.main) / float64(snapshot.StoreCapacityBytes*3/4)
	walRatio := float64(disk.wal) / float64(snapshot.StoreCapacityBytes/8)
	for _, ratio := range []float64{mainRatio, walRatio} {
		if ratio > snapshot.StoreUsageRatio {
			snapshot.StoreUsageRatio = ratio
		}
	}
	if snapshot.StoreUsageRatio >= storePressureDegradedRatio && snapshot.State != "failed" {
		snapshot.State = "degraded"
		snapshot.Reason = "Waku Store retention capacity is under pressure"
	}
}

func storeDiskBytes(path string) (int64, error) {
	usage, err := inspectStoreDisk(path)
	return usage.total, err
}

type storeDiskUsage struct {
	main  int64
	wal   int64
	shm   int64
	total int64
}

func inspectStoreDisk(path string) (storeDiskUsage, error) {
	info, err := os.Stat(path)
	if err != nil {
		return storeDiskUsage{}, err
	}
	usage := storeDiskUsage{main: info.Size(), total: info.Size()}
	for _, item := range []struct {
		suffix string
		target *int64
	}{{"-wal", &usage.wal}, {"-shm", &usage.shm}} {
		suffix := item.suffix
		info, err = os.Stat(path + suffix)
		if err == nil {
			*item.target = info.Size()
			usage.total += info.Size()
			continue
		}
		if !os.IsNotExist(err) {
			return storeDiskUsage{}, err
		}
	}
	return usage, nil
}
