package processresource

import "testing"

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
