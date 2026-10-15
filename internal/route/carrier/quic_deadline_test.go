package carrier

import (
	"testing"
	"time"
)

func TestQUICDeadlinePreservesAbsoluteBoundAndLocalClock(t *testing.T) {
	if got := quicDeadline(time.Time{}); got != (time.Time{}) {
		t.Fatal("clearing deadline installed a physical timeout", got)
	}
	for _, offset := range []time.Duration{-time.Second, time.Second, time.Hour} {
		original := time.Now().Add(offset).UTC().Round(0)
		converted := quicDeadline(original)
		if !converted.Equal(original) || converted.UnixNano() != original.UnixNano() {
			t.Fatal("physical conversion changed original absolute deadline", original, converted)
		}
		if converted == converted.Round(0) {
			t.Fatal("physical deadline lacks a current local monotonic reading")
		}
	}
	local := time.Now().Add(time.Second)
	if converted := quicDeadline(local); !converted.Equal(local) {
		t.Fatal("physical conversion changed original local deadline", local, converted)
	}
}
