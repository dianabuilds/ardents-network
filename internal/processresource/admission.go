package processresource

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Profile names the Forwarding local development envelope, not a wire profile.
const Profile = "ardents-forwarding-process-v1"

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

// Observation projects actual native readings only. Route timer/queue counts
// are not supplied by this owner and cannot be inferred from these fields.
// CPUFraction is measured over the original declared elapsed interval.
type Observation struct {
	Started, Completed, CPUWindowStarted                 time.Time
	CPUUsec, MemoryBytes, RSSBytes, GoMemoryBytes        uint64
	FDs, Threads                                         uint64
	CPUFraction, CPUPressure, MemoryPressure, IOPressure float64
	HighEvents, EmergencyEvents, PIDEvents               uint64
}

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
	if ctx == nil || (profile != Profile && profile != IssuerProfile && profile != DescriptorProfile && profile != IntroductionProfile && profile != JoinProfile && profile != SourceProfile) {
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
	original.baseline, err = native.observe()
	if err == nil {
		err = pressure(original.baseline)
	}
	if err == nil {
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case <-ctx.Done():
			err = ctx.Err()
		case <-timer.C:
		}
		timer.Stop()
	}
	if err == nil {
		_, err = guard.Check()
	}
	if err != nil {
		if original.failed == nil && err != ctx.Err() {
			original.failed = retainFailure(err, "startup", original.baseline, original)
			err = original.failed
		}
		return nil, errors.Join(err, guard.Close())
	}
	return guard, nil
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
	observation, err := original.native.observe()
	if err == nil {
		elapsed := observation.Completed.Sub(original.baseline.Completed)
		if elapsed < 100*time.Millisecond || observation.CPUUsec < original.baseline.CPUUsec || observation.CPUUsec < original.lastCPU {
			err = &admissionFailure{reason: "cpu-interval"}
		} else {
			observation.CPUWindowStarted = original.baseline.Completed
			observation.CPUFraction = float64(observation.CPUUsec-original.baseline.CPUUsec) / float64(elapsed/time.Microsecond)
			original.lastCPU = observation.CPUUsec
			err = pressure(observation)
		}
	}
	if callerErr := original.caller.Err(); callerErr != nil {
		if err == nil {
			err = callerErr
		} else {
			err = errors.Join(err, callerErr)
		}
	}
	if err != nil {
		original.failed = err
		if err != original.caller.Err() {
			original.failed = retainFailure(err, "observation", observation, original)
		}
		err = original.failed
	}
	return observation, err
}

func pressure(observation Observation) error {
	return pressureAt(observation, time.Now())
}

func pressureAt(observation Observation, now time.Time) error {
	if observation.Completed.IsZero() || observation.Started.IsZero() || observation.Completed.Before(observation.Started) || observation.Completed.Sub(observation.Started) > time.Second || now.Before(observation.Completed) || now.Sub(observation.Completed) > time.Second {
		return &admissionFailure{reason: "observation-stale"}
	}
	// The owning development contract admits brief measured CPU/I/O stalls;
	// established trend pressure and every hard/event boundary still retire.
	if observation.HighEvents != 0 || observation.EmergencyEvents != 0 || observation.PIDEvents != 0 {
		return &admissionFailure{reason: "kernel-limit-event"}
	}
	if observation.MemoryBytes >= 768<<20 || observation.RSSBytes >= 768<<20 || observation.GoMemoryBytes >= 512<<20 || observation.FDs >= 1024 || observation.Threads >= 256 {
		return &admissionFailure{reason: "resource-bound"}
	}
	if observation.MemoryPressure > 0 {
		return &admissionFailure{reason: "memory-pressure"}
	}
	if observation.CPUPressure >= 1 {
		return &admissionFailure{reason: "cpu-pressure"}
	}
	if observation.IOPressure >= 1 {
		return &admissionFailure{reason: "io-pressure"}
	}
	return nil
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
