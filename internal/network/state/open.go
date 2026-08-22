package state

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/resource"
)

// Open recovers one state root and verifies any current generation before use.
func Open(input Config) (*networkState, error) {
	resolved, err := validateConfig(input)
	if err != nil {
		return nil, err
	}
	guard, err := openResourceGuard(resolved.profile)
	if err != nil {
		return nil, err
	}
	storage, err := openDurableRoot(resolved.root)
	if err != nil {
		return nil, err
	}
	opened := false
	defer func() {
		if !opened {
			_ = storage.close()
		}
	}()
	workContext, workCancel := context.WithCancel(context.Background())
	defer func() {
		if !opened {
			workCancel()
		}
	}()
	runtime := &networkState{config: resolved, storage: storage, workContext: workContext,
		workCancel: workCancel, resourceGuard: guard}
	if err := runtime.recover(workContext); err != nil {
		return nil, err
	}
	opened = true
	return runtime, nil
}

func openResourceGuard(profile string) (*resource.Guard, error) {
	if profile == "" {
		return nil, nil
	}
	guard, err := resource.New(resource.Config{Profile: profile, Interval: time.Second})
	if err != nil {
		return nil, err
	}
	if err := guard.Check(); err != nil {
		return nil, err
	}
	return guard, nil
}

func (s *networkState) recover(workContext context.Context) error {
	current, decision, err := loadCurrent(s.config, s.storage)
	if err != nil {
		return err
	}
	s.current, s.currentDecision = current, decision
	if err := s.loadDistributionState(); err != nil {
		return err
	}
	if err := s.startSource(workContext); err != nil {
		return err
	}
	if s.config.automatic > 0 {
		s.work.Add(1)
		go s.runAutomaticRefresh(workContext)
	}
	if s.config.profile == "h3-s-v1" {
		s.work.Add(1)
		go s.runResourceGovernor(workContext)
	}
	return nil
}

func (s *networkState) startSource(workContext context.Context) error {
	if !s.config.sourceInfo.Serving {
		return nil
	}
	if s.current == nil {
		return errors.New("source mode requires a current generation")
	}
	if err := s.retainSourceServer(); err != nil {
		return err
	}
	s.serverDone = make(chan struct{})
	ready := make(chan error, 1)
	go func() {
		err := s.serveSource(workContext, ready)
		s.mu.Lock()
		s.serverErr = err
		close(s.serverDone)
		s.mu.Unlock()
	}()
	if err := <-ready; err != nil {
		s.workCancel()
		<-s.serverDone
		return errors.Join(err, s.releaseSourceServer())
	}
	return nil
}
