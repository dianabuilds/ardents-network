package issuer

import (
	"errors"
	"fmt"
	"net"
	"testing"
)

func TestReleaseErrorsRetainFirstUnexpectedFailure(t *testing.T) {
	first := errors.New("first cleanup failure")
	second := errors.New("second cleanup failure")
	var releases releaseErrors
	releases.record(errors.Join(net.ErrClosed, first))
	releases.record(second)
	if err := releases.result(); !errors.Is(err, first) || errors.Is(err, second) || errors.Is(err, net.ErrClosed) {
		t.Fatalf("cleanup result = %v, want only first unexpected failure", err)
	}
}

func TestReleaseErrorsIgnoreWrappedClosed(t *testing.T) {
	var releases releaseErrors
	releases.record(fmt.Errorf("released: %w", net.ErrClosed))
	if err := releases.result(); err != nil {
		t.Fatalf("cleanup result = %v, want nil", err)
	}
}
