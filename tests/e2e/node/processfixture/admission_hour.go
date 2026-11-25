package processfixture

import (
	"testing"
	"time"
)

// WaitAdmissionHour avoids beginning a token fixture during its last minute.
func WaitAdmissionHour(t testing.TB) {
	t.Helper()
	if remaining := time.Until(time.Now().UTC().Truncate(time.Hour).Add(time.Hour)); remaining < time.Minute {
		timer := time.NewTimer(remaining)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-t.Context().Done():
			t.Fatal(t.Context().Err())
		}
	}
}
