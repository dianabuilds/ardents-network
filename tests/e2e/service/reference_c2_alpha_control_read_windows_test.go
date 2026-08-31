//go:build referencec2

package service_test

import (
	"errors"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestReferenceC2PublicationReadRetriesOnlyBoundedSharingViolations(t *testing.T) {
	const path = "publication.json"
	sharingViolation := &os.PathError{Op: "open", Path: path, Err: syscall.Errno(32)}

	t.Run("eventual success", func(t *testing.T) {
		calls, waits := 0, 0
		want := []byte(`{"alpha_corpus":"ready"}`)
		got, err := readReferenceC2PublicationWith(path, func(string) ([]byte, error) {
			calls++
			if calls < 3 {
				return nil, sharingViolation
			}
			return want, nil
		}, func(delay time.Duration) {
			if delay != referenceC2PublicationReadDelay {
				t.Fatalf("retry delay = %v", delay)
			}
			waits++
		})
		if err != nil || string(got) != string(want) || calls != 3 || waits != 2 {
			t.Fatalf("publication read = %q, %v; calls=%d waits=%d", got, err, calls, waits)
		}
	})

	t.Run("persistent sharing violation is bounded", func(t *testing.T) {
		calls, waits := 0, 0
		_, err := readReferenceC2PublicationWith(path, func(string) ([]byte, error) {
			calls++
			return nil, sharingViolation
		}, func(time.Duration) { waits++ })
		if !errors.Is(err, syscall.Errno(32)) || calls != 50 || waits != 49 {
			t.Fatalf("persistent read = %v; calls=%d waits=%d", err, calls, waits)
		}
	})

	t.Run("other read errors fail immediately", func(t *testing.T) {
		denied := errors.New("access denied")
		calls, waits := 0, 0
		_, err := readReferenceC2PublicationWith(path, func(string) ([]byte, error) {
			calls++
			return nil, denied
		}, func(time.Duration) { waits++ })
		if !errors.Is(err, denied) || calls != 1 || waits != 0 {
			t.Fatalf("non-sharing read = %v; calls=%d waits=%d", err, calls, waits)
		}
	})
}
