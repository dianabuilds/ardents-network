//go:build ignore

package main

import (
	"context"
	"sync"
	"time"
)

type linuxSampler struct {
	cancel context.CancelFunc
	done   chan struct{}

	mu      sync.Mutex
	samples []linuxSample
	err     error
	stop    sync.Once
}

func newLinuxSampler(parent context.Context, interval time.Duration) *linuxSampler {
	ctx, cancel := context.WithCancel(parent)
	result := &linuxSampler{cancel: cancel, done: make(chan struct{})}
	go result.run(ctx, interval)
	return result
}

func (sampler *linuxSampler) run(ctx context.Context, interval time.Duration) {
	defer close(sampler.done)
	sampler.capture()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			sampler.capture()
		case <-ctx.Done():
			sampler.capture()
			return
		}
	}
}

func (sampler *linuxSampler) capture() {
	sample, err := sampleLinux()
	sampler.mu.Lock()
	defer sampler.mu.Unlock()
	if err != nil && sampler.err == nil {
		sampler.err = err
	}
	if sample != nil {
		sampler.samples = append(sampler.samples, *sample)
	}
}

func (sampler *linuxSampler) Stop() ([]linuxSample, error) {
	sampler.stop.Do(sampler.cancel)
	<-sampler.done
	sampler.mu.Lock()
	defer sampler.mu.Unlock()
	return append([]linuxSample(nil), sampler.samples...), sampler.err
}
