package testkit

import (
	"testing"
	"time"
)

func WaitForCondition(t *testing.T, timeout time.Duration, description string, check func() (bool, string)) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	detail := "condition was not evaluated"
	for {
		matched, got := check()
		detail = got
		if matched {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for %s: %s", description, detail)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
