package processresource

import (
	"errors"
	"math"
	"testing"
)

func TestRequiredNativeCounterIsNotAnUnknownZero(t *testing.T) {
	for _, raw := range []string{"", "other 0\n", "high 0\nhigh 0\n", "high -1", "high 18446744073709551616", "high 0 extra"} {
		if _, err := nativeCounter(raw, "high"); err == nil {
			t.Fatalf("accepted unavailable/ambiguous counter %q", raw)
		}
	}
	if value, err := nativeCounter("high 0\n", "high"); err != nil || value != 0 {
		t.Fatal("known zero refused", err)
	}
}

func TestRequiredPressureRejectsMissingNonfiniteAndAmbiguous(t *testing.T) {
	for _, raw := range []string{"", "some avg10=NaN avg60=0 avg300=0 total=0", "some avg10=+Inf avg60=0 avg300=0 total=0", "some avg10=-1 avg60=0 avg300=0 total=0", "some avg10=101 avg60=0 avg300=0 total=0", "some avg10=0", "some avg10=0 avg60=0 avg300=0 total=0\nsome avg10=0 avg60=0 avg300=0 total=0"} {
		if _, err := nativePressure(raw, "some"); err == nil {
			t.Fatalf("accepted invalid pressure %q", raw)
		}
	}
	if value, err := nativePressure("some avg10=0.05 avg60=0 avg300=0 total=1", "some"); err != nil || value != 0.05 {
		t.Fatal("actual declared pressure refused", err)
	}
}

// TestNativeMemoryEventsSinglePassEquivalence proves the single-pass extraction
// (audit SD-6) returns exactly the values and parse refusals of the original five
// nativeCounter calls. The observe caller wraps the emergency-counter refusals and
// applies the overflow guard; that wrapping is reproduced here so the comparison
// covers the full prior behavior, including refusal text.
func TestNativeMemoryEventsSinglePassEquivalence(t *testing.T) {
	names := [5]string{"high", "max", "oom", "oom_kill", "oom_group_kill"}
	reference := func(raw string) (uint64, uint64, error) {
		var values [5]uint64
		for index, name := range names {
			value, err := nativeCounter(raw, name)
			if err != nil {
				if index == 0 {
					return 0, 0, err
				}
				return 0, 0, errors.Join(errors.New("native memory event invalid"), err)
			}
			values[index] = value
		}
		var emergency uint64
		for _, value := range values[1:] {
			if value > math.MaxUint64-emergency {
				return 0, 0, errors.New("native memory event invalid")
			}
			emergency += value
		}
		return values[0], emergency, nil
	}
	actual := func(raw string) (uint64, uint64, error) {
		events, err := nativeMemoryEvents(raw)
		if err != nil {
			return 0, 0, err
		}
		var emergency uint64
		for _, value := range [4]uint64{events.max, events.oom, events.oomKill, events.oomGroupKill} {
			if value > math.MaxUint64-emergency {
				return 0, 0, errors.New("native memory event invalid")
			}
			emergency += value
		}
		return events.high, emergency, nil
	}
	for _, raw := range []string{
		"high 1\nmax 2\noom 3\noom_kill 4\noom_group_kill 5\n",
		"high 0\nmax 0\noom 0\noom_kill 0\noom_group_kill 0\n",
		"max 2\noom 3\noom_kill 4\noom_group_kill 5\nhigh 1\n",
		"high 1\nmax 2\noom 3\noom_kill 4\noom_group_kill 5\nlow 99\n",
		"",
		"high 1\n",
		"high 1\nhigh 2\nmax 0\noom 0\noom_kill 0\noom_group_kill 0\n",
		"high 1\nmax 2\nmax 3\noom 0\noom_kill 0\noom_group_kill 0\n",
		"high 1 2\nmax 0\noom 0\noom_kill 0\noom_group_kill 0\n",
		"high abc\nmax 0\noom 0\noom_kill 0\noom_group_kill 0\n",
		"high 1\nmax 18446744073709551615\noom 1\noom_kill 0\noom_group_kill 0\n",
		"high 18446744073709551615\nmax 0\noom 0\noom_kill 0\noom_group_kill 0\n",
	} {
		rh, re, rerr := reference(raw)
		ah, ae, aerr := actual(raw)
		if (rerr == nil) != (aerr == nil) || rh != ah || re != ae {
			t.Fatalf("single-pass differs for %q: ref=(%d,%d,%v) actual=(%d,%d,%v)", raw, rh, re, rerr, ah, ae, aerr)
		}
		if rerr != nil && aerr != nil && rerr.Error() != aerr.Error() {
			t.Fatalf("refusal text differs for %q: ref=%v actual=%v", raw, rerr, aerr)
		}
	}
}
