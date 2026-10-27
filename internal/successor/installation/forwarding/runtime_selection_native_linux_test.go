//go:build linux && network_signing_native

package forwarding

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// The private component controller supplies actual completed installation files,
// a separate mount namespace and the actual Node principal. This ordinary test
// executable cannot be an installed Forwarding invocation or provide Startup.
func TestForwardingRuntimeSelectionNativeIndependentFiles(t *testing.T) {
	path := os.Getenv("ARDENTS_FORWARDING_INITIAL_BINDING")
	mode := os.Getenv("ARDENTS_FORWARDING_RUNTIME_SELECTION_EXPECT")
	if !validPath(path) || os.Geteuid() < 100 || os.Geteuid() > 999 || os.Getegid() != os.Geteuid() || mode != "protected" && mode != "unprotected" {
		t.Fatal("invalid environment: actual private Node principal/mount case required")
	}
	s, err := readRuntimeSelection(t.Context(), path)
	if mode == "unprotected" {
		if s != nil || !errors.Is(err, ErrBinding) {
			if s != nil {
				_ = s.close()
			}
			t.Fatal("actual writable immutable mount admitted runtime custody", err)
		}
		return
	}
	if err != nil {
		t.Fatal("independent actual read-only selection", err)
	}
	defer s.close()
	if s.files.journal != nil || s.files.files[s.binding.Request.InstallationRoot+".forwarding-initial-journal"] != nil || s.files.dirs[s.binding.History] != nil {
		t.Fatal("non-root observation borrowed root journal/history custody")
	}
	if err := s.check(); err != nil {
		t.Fatal("actual protected selection changed", err)
	}
	if err := s.close(); err != nil {
		t.Fatal("original independent snapshot did not join", err)
	}
	if err := s.check(); !errors.Is(err, errCompletedClosed) {
		t.Fatal("closed runtime file observation revived", err)
	}
	// The fresh observer independently opens originals; no live fd crosses.
	s, err = readRuntimeSelection(t.Context(), path)
	if err != nil {
		t.Fatal("fresh independently acquired snapshot", err)
	}
	defer s.close()
	program := s.files.files[filepath.Join(s.files.sealed, programName)]
	if err := program.file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.check(); !errors.Is(err, os.ErrClosed) {
		t.Fatal("lost actual executable observation accepted", err)
	}
	if err := s.check(); !errors.Is(err, os.ErrClosed) {
		t.Fatal("failed original observation revived", err)
	}
	closed := s.close()
	if !errors.Is(closed, os.ErrClosed) || s.close() != closed {
		t.Fatal("original physical Close failure was erased", closed)
	}
}
