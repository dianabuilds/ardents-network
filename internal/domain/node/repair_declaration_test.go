package node

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/processresource"
)

func TestRepairDeclarationBindsRoleAndOrderedRoots(t *testing.T) {
	base := t.TempDir()
	plan := testCoreMap()
	network := testNodeNetwork()
	network.Root = filepath.Join(base, "state")
	plan["network"] = network
	roots := []string{network.Root}
	for _, name := range []string{"participation_root", "spend_root", "hosting_root", "descriptor_root"} {
		path := filepath.Join(base, name)
		roots = append(roots, path)
		plan[name] = path
	}
	plan["process_profile"] = processresource.DescriptorProfile
	original := marshalTestPlan(t, plan)
	pins, got, err := RepairDeclaration("descriptor", original)
	if err != nil || pins.NetworkID != network.NetworkID || !slices.Equal(roots, got) {
		t.Fatalf("original repaired scope = %v, %v", got, err)
	}
	for _, role := range []string{"forwarding", "issuer", "source"} {
		if _, _, err := RepairDeclaration(role, original); err == nil {
			t.Fatalf("descriptor declaration supplied repair for %s", role)
		}
	}
	plan["descriptor_root"] = filepath.Join(roots[1], "nested")
	if _, _, err := RepairDeclaration("descriptor", marshalTestPlan(t, plan)); err == nil {
		t.Fatal("overlapping root supplied a repair declaration")
	}
	plan["descriptor_root"] = roots[4]
	network.Root = filepath.VolumeName(base) + string(filepath.Separator)
	plan["network"] = network
	if _, _, err := RepairDeclaration("descriptor", marshalTestPlan(t, plan)); err == nil {
		t.Fatal("filesystem root supplied a repair declaration with nested roots")
	}
}
