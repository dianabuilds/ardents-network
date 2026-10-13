package stock

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
)

// Concurrent Source operations must not turn callback completion order into
// a false clock rollback. Real issuance prepares the original permission;
// the scheduler changes only when two fresh observations return.
func TestHolderConcurrentObservationsPreserveTimeOrder(t *testing.T) {
	t.Run("concurrent", func(t *testing.T) {
		o, host, _ := issuedStockFixture(t)
		entered, release := make(chan struct{}), make(chan struct{})
		var observations atomic.Int32
		o.observe = func() (admission.AuthorityFacts, time.Time, error) {
			n := observations.Add(1)
			at := host.now.Add(time.Duration(n) * time.Millisecond)
			if n == 1 {
				close(entered)
				<-release
			}
			return host.profile, at, nil
		}
		results := make(chan error, 2)
		request := func() {
			raw, _, err := o.Request([3]uint32{0, 4, 0})
			clear(raw)
			results <- err
		}
		go request()
		<-entered
		go request()
		remaining := 2
		select {
		case err := <-results:
			// Old implementation lets this observation overtake the held one.
			remaining--
			if err != nil {
				t.Error("newer fresh observation refused", err)
			}
		case <-time.After(50 * time.Millisecond):
			// Serialized observation waits without retaining the state mutex.
		}
		close(release)
		for range remaining {
			if err := <-results; err != nil {
				t.Error("concurrent fresh observation refused", err)
			}
		}
		// Actual backwards time remains a refusal after both users finish.
		o.observe = func() (admission.AuthorityFacts, time.Time, error) {
			return host.profile, host.now, nil
		}
		if _, _, err := o.Request([3]uint32{0, 4, 0}); err == nil {
			t.Error("real clock rollback accepted")
		}
	})
}
