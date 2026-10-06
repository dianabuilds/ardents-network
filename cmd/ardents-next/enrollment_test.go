package main

import (
	"bytes"
	"context"
	"testing"
)

func TestEnrollmentCommandRefusesBeforeBundleEffects(t *testing.T) {
	for _, args := range [][]string{
		{"enrollment"}, {"enrollment", "verify", "missing", "not-a-pin"}, {"enrollment", "install", "missing", "not-a-pin"},
	} {
		var out, diagnostic bytes.Buffer
		if code := run(context.Background(), args, &out, &diagnostic); code != 2 || out.String() != "{\"outcome\":\"invalid-input\"}\n" {
			t.Fatalf("args %v: code %d out %q", args, code, out.String())
		}
	}
}
