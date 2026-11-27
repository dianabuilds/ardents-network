package plan

import (
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/platform/processguard"
)

func TestDescriptorInvocationRequiresIndependentPurposeRoots(t *testing.T) {
	parent := t.TempDir()
	original := DescriptorPlan{
		nodePlanCore: nodePlanCore{
			Network:           &Network{Root: filepath.Join(parent, "network")},
			ProcessProfile:    processguard.DescriptorProfile,
			ParticipationRoot: filepath.Join(parent, "participation"),
			SpendRoot:         filepath.Join(parent, "spend"),
			HostingRoot:       filepath.Join(parent, "budget"),
			RetirementWaitMS:  10,
		},
		DescriptorRoot: filepath.Join(parent, "descriptor"),
	}
	if !original.ValidInvocation() {
		t.Fatal("independent declared roots refused")
	}
	for _, test := range []struct{ name, root string }{
		{"descriptor", original.DescriptorRoot},
		{"nested under descriptor", filepath.Join(original.DescriptorRoot, "history")},
		{"participation", original.ParticipationRoot},
		{"spend", original.SpendRoot},
		{"budget", original.HostingRoot},
		{"parent of all", parent},
		{"relative", "relative"},
		{"empty", ""},
		{"filesystem root", filepath.VolumeName(parent) + string(filepath.Separator)},
	} {
		t.Run(test.name, func(t *testing.T) {
			changed := original
			network := *original.Network
			network.Root = test.root
			changed.Network = &network
			if changed.ValidInvocation() {
				t.Fatalf("Descriptor invocation accepted aliased or invalid Network State root %q", test.root)
			}
		})
	}
	changed := original
	changed.SpendRoot = original.ParticipationRoot
	if changed.ValidInvocation() {
		t.Fatal("Descriptor invocation accepted two aliased role roots")
	}
}
