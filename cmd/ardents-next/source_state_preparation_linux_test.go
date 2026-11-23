//go:build linux

package main

import (
	"bytes"
	"os"
	"testing"
)

func TestSourcePreparationRefusesOtherPrincipalBeforeInputEffects(t *testing.T) {
	if os.Geteuid() >= 100 && os.Geteuid() <= 999 {
		t.Fatal("ordinary test profile requires an unprivileged development user outside the fixed system-account range")
	}
	var out, diagnostic bytes.Buffer
	if code := run(t.Context(), []string{"source", "prepare-state", "/unopened-plan", "/unopened-bundle"}, &out, &diagnostic); code == 0 || out.Len() != 0 {
		t.Fatal("foreign principal reached preparation or receipt", code)
	}
}
