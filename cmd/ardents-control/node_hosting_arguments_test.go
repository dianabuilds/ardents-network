package main

import (
	"errors"
	"testing"
)

func TestNodeHostingArgumentsPreserveRoleErrors(t *testing.T) {
	args := []string{"--binding", "/opt/node/binding.json", "--provider", "provider", "--start", "2026-10-27T00:00:00Z", "--end", "2026-11-27T00:00:00Z", "--unit", "GiB", "--quantity", "100", "--directions", "tx+rx", "--interfaces", "eth0,lo", "--initial-used-bytes", "1000", "--low-watermark-bytes", "1048576"}
	for _, role := range []string{"join", "descriptor", "forwarding", "introduction"} {
		t.Run(role, func(t *testing.T) {
			invalid := errors.New(role + " invalid input")
			path, policy, err := nodeHostingArguments(args, role, invalid)
			if err != nil || path != args[1] || policy.InitialUsedBytes != 1000 {
				t.Fatalf("valid declaration refused: %v", err)
			}
			for _, extra := range [][]string{{"--quantity", "1"}, {"--role", "other"}, {"trailing"}} {
				candidate := append(append([]string(nil), args...), extra...)
				if _, _, err := nodeHostingArguments(candidate, role, invalid); !errors.Is(err, invalid) {
					t.Fatalf("invalid declaration: got %v, want role error", err)
				}
			}
		})
	}
}
