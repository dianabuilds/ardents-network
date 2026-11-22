//go:build !linux

package main

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestSourceUnsupportedPlatformRefusesBeforePlanEffects(t *testing.T) {
	for _, operation := range []string{"serve", "prepare-state"} {
		t.Run(operation, func(t *testing.T) {
			plan := filepath.Join(t.TempDir(), "absent-plan")
			args := []string{"source", operation, plan}
			refusal := "source"
			if operation == "prepare-state" {
				args = append(args, filepath.Join(filepath.Dir(plan), "absent-bundle"))
				refusal = "source.prepare-state"
			}
			var out, diagnostic bytes.Buffer
			if code := run(t.Context(), args, &out, &diagnostic); code != 1 {
				t.Fatal("unsupported Source performed command effects", code)
			}
			assertUnsupportedPlatformRefusal(t, &diagnostic, &out, refusal)
		})
	}
}
