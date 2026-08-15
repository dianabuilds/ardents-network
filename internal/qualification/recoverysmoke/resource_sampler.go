package recoverysmoke

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/recovery"
)

type statsSampler struct {
	mu             sync.Mutex
	samples        []recovery.ResourceSample
	err            error
	cancel         context.CancelFunc
	done           chan struct{}
	ready          chan error
	readyOnce      sync.Once
	stopOnce       sync.Once
	stoppedSamples []recovery.ResourceSample
	stoppedErr     error
}

func (observer dockerObserver) startStats(ctx context.Context, identities map[string]string, clock time.Time) *statsSampler {
	sampleCtx, cancel := context.WithCancel(ctx)
	result := &statsSampler{cancel: cancel, done: make(chan struct{}), ready: make(chan error, 1)}
	go func() {
		defer close(result.done)
		samples, err := observer.streamResourceSamples(sampleCtx, identities, clock, func() {
			result.signalReady(nil)
		})
		if len(samples) == 0 {
			result.signalReady(errors.Join(errors.New("resource sampler ended before its first complete sample"), err))
		}
		result.mu.Lock()
		defer result.mu.Unlock()
		result.samples, result.err = samples, err
	}()
	return result
}

func (sampler *statsSampler) waitReady(ctx context.Context) error {
	select {
	case err := <-sampler.ready:
		return err
	case <-ctx.Done():
		return fmt.Errorf("wait for first complete host resource sample: %w", ctx.Err())
	}
}

func (sampler *statsSampler) signalReady(err error) {
	sampler.readyOnce.Do(func() { sampler.ready <- err })
}

func (sampler *statsSampler) stop() ([]recovery.ResourceSample, error) {
	sampler.stopOnce.Do(func() {
		sampler.cancel()
		<-sampler.done
		sampler.mu.Lock()
		defer sampler.mu.Unlock()
		for _, sample := range sampler.samples {
			if sample.ClientRSS > 0 && sample.PublisherRSS > 0 {
				sampler.stoppedSamples = append(sampler.stoppedSamples, sample)
			}
		}
		sampler.stoppedErr = sampler.err
	})
	return append([]recovery.ResourceSample(nil), sampler.stoppedSamples...), sampler.stoppedErr
}

func (sampler *statsSampler) stopAfter(originNanos int64) ([]recovery.ResourceSample, error) {
	samples, err := sampler.stop()
	if originNanos <= 0 {
		return nil, errors.Join(err, errors.New("resource sample origin is invalid"))
	}
	result := make([]recovery.ResourceSample, 0, len(samples))
	for _, sample := range samples {
		if sample.AtNanos <= originNanos {
			continue
		}
		sample.AtNanos -= originNanos
		result = append(result, sample)
	}
	return result, err
}

func (sampler *statsSampler) coverageStartedAfter(originNanos int64) (int64, error) {
	samples, err := sampler.stop()
	if originNanos <= 0 || len(samples) == 0 {
		return 0, errors.Join(err, errors.New("resource sampling coverage origin is invalid"))
	}
	return max(int64(1), samples[0].AtNanos-originNanos), err
}
