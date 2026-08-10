package tooling

import (
	"reflect"
	"testing"
)

func TestNativeCaptureFilterUsesResolvedAddresses(t *testing.T) {
	got := nativeCaptureFilter([]string{"172.20.0.2", "fd00::2"})
	want := []string{"host", "172.20.0.2", "or", "host", "fd00::2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("capture filter = %v, want %v", got, want)
	}
}
