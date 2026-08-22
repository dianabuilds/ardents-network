//go:build windows

package updatetransaction

import (
	"errors"
	"testing"

	"golang.org/x/sys/windows"
)

func TestActivationBusyClassifiesOnlyWindowsContention(t *testing.T) {
	for _, err := range []error{
		windows.ERROR_SHARING_VIOLATION,
		windows.ERROR_LOCK_VIOLATION,
		errors.New("durability failure"),
		windows.ERROR_ACCESS_DENIED,
	} {
		want := errors.Is(err, windows.ERROR_SHARING_VIOLATION) || errors.Is(err, windows.ERROR_LOCK_VIOLATION)
		if got := isActivationBusy(err); got != want {
			t.Fatalf("isActivationBusy(%v) = %v, want %v", err, got, want)
		}
	}
}
