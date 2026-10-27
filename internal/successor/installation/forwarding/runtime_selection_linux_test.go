package forwarding

import (
	"errors"
	"os"
	"testing"
)

func TestRuntimeSelectionCannotBorrowRootObservation(t *testing.T) {
	if os.Geteuid() != 0 {
		// A malformed path remains a pre-acquisition refusal for any principal.
		if s, err := readRuntimeSelection(t.Context(), "relative"); s != nil || !errors.Is(err, ErrInput) {
			t.Fatal("malformed non-root selection accepted", err)
		}
		return
	}
	if s, err := readRuntimeSelection(t.Context(), "/var/lib/ardents-node-installation/binding.json"); s != nil || !errors.Is(err, ErrInput) {
		t.Fatal("root observation substituted for independent non-root custody", err)
	}
}

func TestRuntimeMountObservationRefusesForeignFilesystem(t *testing.T) {
	f, err := os.Open("/proc")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	d := &inputDirectory{file: f}
	for _, readOnly := range []bool{false, true} {
		if err := runtimeDirectoryMount(d, readOnly); !errors.Is(err, ErrBinding) {
			t.Fatal("foreign filesystem supplied native immutable/writable custody", err)
		}
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := runtimeDirectoryMount(d, true); !errors.Is(err, ErrBinding) {
		t.Fatal("lost original mount descriptor supplied protection", err)
	}
}
