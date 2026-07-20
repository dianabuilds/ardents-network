package orchestration

import (
	"context"
	"fmt"
	"time"

	discoveryrecord "ardents/internal/discovery/record"
)

const startupRollbackTimeout = 5 * time.Second

type RuntimeCommand interface {
	StartLocked(context.Context) error
	StopLocked(context.Context) error
	FailLocked(code, domain, summary, detail, impact, recovery string)
}

type TransportStopper interface {
	State() string
	Stop(context.Context) error
}

type CommandService struct {
	runtime   RuntimeCommand
	transport TransportStopper
}

func NewCommandService(runtime RuntimeCommand, transport TransportStopper) *CommandService {
	return &CommandService{
		runtime:   runtime,
		transport: transport,
	}
}

func (s *CommandService) StartLocked(
	ctx context.Context,
	startBlobExchange func(context.Context) error,
) (context.Context, context.CancelFunc, error) {
	networkCtx, cancel := context.WithCancel(context.Background())
	stopStartupCancel := context.AfterFunc(ctx, cancel)
	if err := s.runtime.StartLocked(networkCtx); err != nil {
		stopStartupCancel()
		cancel()
		return nil, nil, err
	}
	if err := startBlobExchange(networkCtx); err != nil {
		stopStartupCancel()
		return nil, nil, s.rollbackDataPlaneStartup(cancel, err)
	}
	stopStartupCancel()
	if err := ctx.Err(); err != nil {
		return nil, nil, s.rollbackDataPlaneStartup(cancel, err)
	}
	return networkCtx, cancel, nil
}

func (s *CommandService) StopLocked(ctx context.Context, cancel context.CancelFunc) error {
	err := s.runtime.StopLocked(ctx)
	if cancel != nil {
		cancel()
	}
	return err
}

func (s *CommandService) StartDiscoveryRefreshLoop(
	ctx context.Context,
	configuredInterval time.Duration,
	refresh func(context.Context),
) {
	interval := DiscoveryRefreshInterval(configuredInterval)
	if interval <= 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				refresh(ctx)
			}
		}
	}()
}

func DiscoveryRefreshInterval(configuredInterval time.Duration) time.Duration {
	if configuredInterval > 0 {
		return configuredInterval
	}
	return discoveryrecord.LocalRecordTTL / 2
}

func (s *CommandService) rollbackDataPlaneStartup(cancel context.CancelFunc, startErr error) error {
	rollbackCtx, rollbackCancel := context.WithTimeout(context.Background(), startupRollbackTimeout)
	stopErr := s.runtime.StopLocked(rollbackCtx)
	if s.transport.State() != "stopped" {
		if transportStopErr := s.transport.Stop(rollbackCtx); transportStopErr != nil {
			if stopErr == nil {
				stopErr = transportStopErr
			} else {
				stopErr = fmt.Errorf("%v; transport stop: %w", stopErr, transportStopErr)
			}
		}
	}
	rollbackCancel()
	cancel()
	if stopErr != nil {
		s.runtime.FailLocked(
			"node.data_plane.rollback_failed",
			"data",
			"data-plane startup rollback failed",
			fmt.Sprintf("start error: %v; rollback error: %v", startErr, stopErr),
			"node runtime may still hold partial startup state",
			"operator",
		)
		return fmt.Errorf("start data-plane exchange: %w; rollback runtime: %v", startErr, stopErr)
	}
	return fmt.Errorf("start data-plane exchange: %w", startErr)
}
