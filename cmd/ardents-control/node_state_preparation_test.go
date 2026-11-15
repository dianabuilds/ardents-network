package main

import (
	"errors"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/deployment"
)

func TestOfflineNodeStatePreparationPreservesInputBoundaries(t *testing.T) {
	invalid, binding := errors.New("invalid"), errors.New("binding")
	n := deployment.NodeNetworkInputs{Root: "/state", NetworkID: [32]byte{1}, Authorities: [][32]byte{{2}}, Threshold: 1, ProfileAuthority: [32]byte{3}, ClockObservationFile: "/clock"}
	paths := []string{"/participation", "/spend", "/hosting", "/certificate", "/key"}
	cfg, err := nodePreparationStateConfig(n, paths, invalid, binding)
	if err != nil || cfg.Root != n.Root || cfg.LocalRoleStateRoot != "" || cfg.PermitWork != nil {
		t.Fatalf("offline configuration: %v", err)
	}
	n.Authorities[0][0] = 9
	for _, pin := range cfg.Authorities {
		if pin[0] != 2 {
			t.Fatal("caller mutated copied trust pin")
		}
	}
	n.Authorities[0][0] = 2
	if _, err := nodePreparationStateConfig(n, []string{"/state/child"}, invalid, binding); !errors.Is(err, binding) {
		t.Fatalf("overlapping path: %v", err)
	}
	if _, err := nodePreparationStateConfig(n, []string{"relative"}, invalid, binding); !errors.Is(err, invalid) {
		t.Fatalf("relative path: %v", err)
	}
	n.Authorities = [][32]byte{{2}, {2}}
	if _, err := nodePreparationStateConfig(n, paths, invalid, binding); !errors.Is(err, binding) {
		t.Fatalf("duplicate pin: %v", err)
	}
}
