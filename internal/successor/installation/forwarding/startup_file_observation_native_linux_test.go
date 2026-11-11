//go:build linux && network_signing_native

package forwarding

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// This selected component uses the genuine protected non-root file fixture.
// Its callbacks supply no manager/process, Startup or domain authority.
func TestForwardingStartupFilesNativeOriginalObservation(t *testing.T) {
	if os.Getenv("ARDENTS_FORWARDING_RUNTIME_SELECTION_EXPECT") != "protected" {
		t.Fatal("invalid environment: protected non-root file fixture required")
	}
	for _, fault := range []string{"none", "foreign-refusal", "closed-file", "caller"} {
		t.Run(fault, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			s, err := readRuntimeSelection(ctx, os.Getenv("ARDENTS_FORWARDING_INITIAL_BINDING"))
			if err != nil {
				t.Fatal("independent original file observer", err)
			}
			defer s.close()
			foreign := errors.New("original foreign refusal")
			err = s.observeStartupFiles(func() error {
				for range 3 {
					if err := s.observeStartupPaths([]string{filepath.Join(s.files.sealed, "unexpected.service")}); err != nil {
						return err
					}
				}
				switch fault {
				case "foreign-refusal":
					return foreign
				case "closed-file":
					if err := s.files.files[filepath.Join(s.files.sealed, programName)].file.Close(); err != nil {
						t.Fatal(err)
					}
					return foreign
				case "caller":
					cancel()
					return ctx.Err()
				}
				return nil
			})
			switch fault {
			case "none":
				if err != nil || s.close() != nil {
					t.Fatal("complete independent physical observation refused", err)
				}
				return
			case "foreign-refusal":
				if err != foreign {
					t.Fatal("unchanged foreign error identity was replaced", err)
				}
			case "closed-file":
				if !errors.Is(err, foreign) || !errors.Is(err, os.ErrClosed) || !errors.Is(err, ErrBinding) {
					t.Fatal("post-observation physical failure or original refusal lost", err)
				}
			case "caller":
				if err != context.Canceled {
					t.Fatal("bare original caller cancellation was wrapped", err)
				}
			}
			first := err
			if next := s.observeStartupFiles(func() error {
				t.Error("denied original file observer ran late native work")
				return nil
			}); next != first {
				t.Fatal("later observation revived or replaced original refusal", next)
			}
			if !errors.Is(s.close(), first) {
				t.Fatal("original joined close erased retained refusal")
			}
		})
	}
}

func BenchmarkForwardingStartupFilesNative(b *testing.B) {
	if os.Getenv("ARDENTS_FORWARDING_RUNTIME_SELECTION_EXPECT") != "protected" {
		b.Fatal("invalid environment: protected non-root file fixture required")
	}
	s, err := readRuntimeSelection(b.Context(), os.Getenv("ARDENTS_FORWARDING_INITIAL_BINDING"))
	if err != nil {
		b.Fatal(err)
	}
	defer func() {
		if err := s.close(); err != nil {
			b.Fatal(err)
		}
	}()
	absent := []string{filepath.Join(s.files.sealed, "unexpected.service")}
	for _, name := range []string{"separate", "compound"} {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				observe := func() error {
					for range 3 {
						if err := s.observeStartupPaths(absent); err != nil {
							return err
						}
					}
					return nil
				}
				if name == "separate" {
					// Original three separately bracketed file observations;
					// this benchmark supplies no manager or Startup admission.
					for range 3 {
						if err = checkStartupNamespace(s, absent); err != nil {
							break
						}
					}
				} else {
					err = s.observeStartupFiles(observe)
				}
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// The root fixture actor opens this exact original writable descriptor before
// protecting the mount, then passes it only to this final negative control.
// No installed Node is running; this stand is never used after the mutation.
func TestForwardingStartupFilesNativeRestoredBytesRemainDenied(t *testing.T) {
	if os.Getenv("ARDENTS_FORWARDING_RUNTIME_SELECTION_EXPECT") != "protected" {
		t.Fatal("invalid environment: protected non-root file fixture required")
	}
	number, err := strconv.Atoi(os.Getenv("ARDENTS_FORWARDING_TEST_ORIGINAL_PROGRAM_FD"))
	if err != nil || number < 3 || number > 256 {
		t.Fatal("invalid environment: original root actor descriptor required")
	}
	writer := os.NewFile(uintptr(number), "original test actor")
	if writer == nil {
		t.Fatal("original root actor descriptor absent")
	}
	defer writer.Close()
	s, err := readRuntimeSelection(t.Context(), os.Getenv("ARDENTS_FORWARDING_INITIAL_BINDING"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.close()
	original := s.files.files[filepath.Join(s.files.sealed, programName)]
	actual, err := writer.Stat()
	if err != nil || !os.SameFile(original.info, actual) {
		t.Fatal("foreign writer descriptor", err)
	}
	first := original.body[0]
	err = s.observeStartupFiles(func() error {
		for _, value := range []byte{first ^ 1, first} {
			if _, err := writer.WriteAt([]byte{value}, 0); err != nil {
				return err
			}
			if err := writer.Sync(); err != nil {
				return err
			}
		}
		return nil
	})
	if !errors.Is(err, ErrBinding) {
		t.Fatal("restored bytes revived original complete observation", err)
	}
	var byteNow [1]byte
	if n, readErr := writer.ReadAt(byteNow[:], 0); readErr != nil || n != 1 || byteNow[0] != first {
		t.Fatal("actual original byte restoration not established", readErr)
	}
	actual, statErr := writer.Stat()
	if statErr != nil || sameInputFile(original.info, actual) {
		t.Fatal("actual original metadata mutation not established", statErr)
	}
	retained := err
	if next := s.observeStartupFiles(func() error { t.Error("denied original admitted late work"); return nil }); next != retained {
		t.Fatal("restored original lost permanent denial", next)
	}
	if !errors.Is(s.close(), retained) {
		t.Fatal("joined close lost original refusal")
	}
}
