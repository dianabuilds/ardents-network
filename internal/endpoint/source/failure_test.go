//go:build linux

package source

import (
	"errors"
	"testing"
)

func TestTextSourcePreparationFailureRetainsStageAndCause(t *testing.T) {
	cause := errors.New("opening issuance refused")
	failure := PreparationFailureAt("issuance", cause)
	if got := PreparationFailureStage(failure); got != "issuance" {
		t.Fatalf("source preparation stage = %q", got)
	}
	if !errors.Is(failure, cause) {
		t.Fatal("source preparation failure lost its cause")
	}
}

func TestTextPrefixPreparationFailureRetainsStageAndCause(t *testing.T) {
	cause := errors.New("source carrier refused")
	failure := PrefixFailureAt("opening", cause)
	if got := PrefixFailureStage(failure); got != "opening" {
		t.Fatalf("prefix preparation stage = %q", got)
	}
	if !errors.Is(failure, cause) {
		t.Fatal("prefix preparation failure lost its cause")
	}
}
