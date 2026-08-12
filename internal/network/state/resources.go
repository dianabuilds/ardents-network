package state

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/resource"
)

func (s *networkState) runResourceGovernor(ctx context.Context) {
	defer s.work.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mu.RLock()
			timers := uint64(s.activeSource) + 2
			s.mu.RUnlock()
			observation, err := s.resourceGuard.Observe(timers, 0, 0)
			if err == nil && s.config.resources != nil {
				raw, marshalErr := json.Marshal(struct {
					Schema   string          `json:"schema"`
					Kind     string          `json:"kind"`
					At       time.Time       `json:"at"`
					Resource resource.Sample `json:"resource"`
				}{"ardents-h3-resource-sample-v1", "resource-sample", time.Now().UTC(), observation.Sample})
				if marshalErr != nil {
					err = marshalErr
				} else {
					err = s.config.resources(append(raw, '\n'))
				}
			}
			if err != nil {
				s.failResourceGovernor(errors.New("H3-S resource accounting is unavailable"))
				return
			}
			s.mu.Lock()
			s.resourceProtect = observation.Protect
			s.mu.Unlock()
			if observation.Drain {
				s.failResourceGovernor(errors.New("H3-S resource emergency requires drain and exit"))
				return
			}
		}
	}
}

func (s *networkState) failResourceGovernor(err error) {
	s.mu.Lock()
	s.resourceErr, s.resourceProtect = err, true
	cancel := s.workCancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}
