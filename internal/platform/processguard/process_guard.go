package processguard

import (
	"context"
	"errors"
	"sync"
	"time"
)

const minimumCPUInterval = 100 * time.Millisecond

// ForwardingProfile names the Forwarding local development envelope, not a wire profile.
const ForwardingProfile = "ardents-forwarding-process-v1"

// IssuerProfile selects the independently qualified Issuer workload under the
// same mandatory native envelope. It grants no extra workers or protocol bytes.
const IssuerProfile = "ardents-issuer-process-v1"

// DescriptorProfile selects its distinct original Descriptor workload under
// the same mandatory native envelope, without additional protocol authority.
const DescriptorProfile = "ardents-descriptor-process-v1"

// IntroductionProfile selects the original Introduction workload under the
// same mandatory native envelope, without new protocol or Hosting allowance.
const IntroductionProfile = "ardents-introduction-process-v1"

// JoinProfile selects the original Rendezvous JOIN workload under the same
// mandatory native envelope. It grants no refill, renewal or Hosting allowance.
const JoinProfile = "ardents-join-process-v1"

// SourceProfile selects the original Direct-Origin Source distribution workload
// under the same mandatory native envelope. It grants no Node or Route duty.
const SourceProfile = "ardents-source-process-v1"

// Guard retains one original process/cgroup and its caller, native descriptors,
// first admission failure and completion. Copies cannot create another owner.
type Guard struct{ original *originalProcess }
type originalProcess struct {
	mu       sync.Mutex
	caller   context.Context
	native   *nativePin
	baseline Observation
	lastCPU  uint64
	failed   error
	closed   bool
	closeErr error
	started  time.Time
}

// Open checks the selected profile and establishes a genuine elapsed CPU baseline
// before returning admission. It never accepts a caller-supplied measurement.
func Open(ctx context.Context, profile string) (*Guard, error) {
	if ctx == nil || !knownProfile(profile) {
		return nil, errors.New("process protection profile unavailable")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	native, err := openNative()
	if err != nil {
		return nil, err
	}
	original := &originalProcess{caller: ctx, native: native, started: time.Now()}
	guard := &Guard{original: original}
	baseline, err := native.observe()
	if err == nil {
		err = validateObservation(baseline, time.Now())
	}
	if err != nil {
		original.failed = retainFailure(err, "startup", baseline, original)
		return nil, errors.Join(original.failed, guard.Close())
	}
	original.baseline = baseline

	timer := time.NewTimer(minimumCPUInterval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, errors.Join(ctx.Err(), guard.Close())
	case <-timer.C:
	}
	if _, err := guard.Check(); err != nil {
		return nil, errors.Join(err, guard.Close())
	}
	return guard, nil
}

func knownProfile(profile string) bool {
	switch profile {
	case ForwardingProfile, IssuerProfile, DescriptorProfile, IntroductionProfile, JoinProfile, SourceProfile:
		return true
	default:
		return false
	}
}

// Check reobserves actual original placement and fresh native readings. No
// cached success or notification is admission. Native I/O is serialized only
// within its own mechanism, never under the Node lifetime's state mutex.
func (guard *Guard) Check() (Observation, error) {
	if guard == nil || guard.original == nil {
		return Observation{}, errors.New("original process protection absent")
	}
	original := guard.original
	original.mu.Lock()
	defer original.mu.Unlock()
	if original.closed {
		if original.failed != nil {
			return Observation{}, original.failed
		}
		return Observation{}, errors.New("original process protection closed")
	}
	if original.failed != nil {
		return Observation{}, original.failed
	}
	if err := original.caller.Err(); err != nil {
		original.failed = err
		return Observation{}, err
	}
	observation, observationErr := original.observeAndCheck()
	callerErr := original.caller.Err()
	switch {
	case observationErr != nil:
		if callerErr != nil {
			observationErr = errors.Join(observationErr, callerErr)
		}
		original.failed = retainFailure(observationErr, "observation", observation, original)
	case callerErr != nil:
		original.failed = callerErr
	}
	return observation, original.failed
}

func (original *originalProcess) observeAndCheck() (Observation, error) {
	observation, err := original.native.observe()
	if err != nil {
		return observation, err
	}
	elapsed := observation.Completed.Sub(original.baseline.Completed)
	if elapsed < minimumCPUInterval ||
		observation.CPUUsec < original.baseline.CPUUsec ||
		observation.CPUUsec < original.lastCPU {
		return observation, &admissionFailure{reason: "cpu-interval"}
	}
	cpuUsed := observation.CPUUsec - original.baseline.CPUUsec
	observation.CPUWindowStarted = original.baseline.Completed
	observation.CPUFraction = float64(cpuUsed) / float64(elapsed/time.Microsecond)
	original.lastCPU = observation.CPUUsec
	return observation, validateObservation(observation, time.Now())
}

// Close releases only this mechanism's original pins once. The consumer must
// join every physical borrower before calling it; it grants no role completion.
func (guard *Guard) Close() error {
	if guard == nil || guard.original == nil {
		return nil
	}
	original := guard.original
	original.mu.Lock()
	defer original.mu.Unlock()
	if !original.closed {
		original.closed = true
		original.closeErr = original.native.close()
	}
	return original.closeErr
}
