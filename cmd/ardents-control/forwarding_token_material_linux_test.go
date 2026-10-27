package main

import (
	"strings"
	"testing"
)

func TestForwardingTokenMaterialFlagsRejectAmbiguousBindingBeforeEffects(t *testing.T) {
	args := []string{"--network-id", strings.Repeat("01", 32), "--issuer-node-id", strings.Repeat("02", 32), "--issuer-node-key", strings.Repeat("03", 32), "--start", "2026-10-27T12:00:00Z", "--end", "2026-10-27T13:00:00Z", "--output", "/protected/offline-material"}
	if p, binding, output, err := forwardingTokenMaterialArguments(args); err != nil || output != args[11] || binding.Network[0] != 1 || binding.Issuer[0] != 2 || binding.Signer[0] != 3 || !binding.Start.Equal(p.Start) || !binding.End.Equal(p.End) {
		t.Fatal("complete finite public binding refused", err)
	}
	for _, tc := range []struct {
		name  string
		index int
		value string
	}{
		{"zero-network", 1, strings.Repeat("00", 32)}, {"upper-key", 5, strings.Repeat("AB", 32)},
		{"unaligned-start", 7, "2026-10-27T12:00:01Z"}, {"fraction", 7, "2026-10-27T12:00:00.1Z"},
		{"offset", 7, "2026-10-27T15:00:00+03:00"}, {"unaligned-end", 9, "2026-10-27T13:00:01Z"},
		{"long", 9, "2026-10-27T19:00:00Z"}, {"empty-interval", 9, "2026-10-27T12:00:00Z"},
		{"relative", 11, "relative"}, {"noncanonical-path", 11, "/protected/../material"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := append([]string(nil), args...)
			changed[tc.index] = tc.value
			if _, _, _, err := forwardingTokenMaterialArguments(changed); err == nil {
				t.Fatal("invalid public binding accepted")
			}
		})
	}
	for _, extra := range [][]string{{"--network-id", strings.Repeat("04", 32)}, {"--end", "2026-10-27T14:00:00Z"}, {"--unknown", "true"}, {"extra"}} {
		if _, _, _, err := forwardingTokenMaterialArguments(append(append([]string(nil), args...), extra...)); err == nil {
			t.Fatal("duplicate or unknown argument accepted")
		}
	}
}
