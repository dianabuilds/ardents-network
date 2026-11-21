package process

import "testing"

// The endpoint placement is the exact unified cgroup2 service path. These cases
// prove the observation tolerates Ubuntu 22.04's redundant v1 name=systemd
// tracking line while still refusing any cgroup2 placement tamper, a missing
// unified line, or an ambiguous repeated unified line.
const endpointPlacement = "0::/system.slice/ardents-endpoint.service\n"

func TestCgroupPlacementMatchesPureUnified(t *testing.T) {
	if !cgroupPlacementMatches([]byte(endpointPlacement), endpointPlacement) {
		t.Fatal("pure unified cgroup2 placement must match")
	}
}

func TestCgroupPlacementMatchesHybridNameSystemd(t *testing.T) {
	hybrid := "1:name=systemd:/system.slice/ardents-endpoint.service\n" + endpointPlacement
	if !cgroupPlacementMatches([]byte(hybrid), endpointPlacement) {
		t.Fatal("Ubuntu 22.04 hybrid name=systemd line must be tolerated")
	}
	reversed := endpointPlacement + "1:name=systemd:/system.slice/ardents-endpoint.service\n"
	if !cgroupPlacementMatches([]byte(reversed), endpointPlacement) {
		t.Fatal("hybrid line order must not matter")
	}
	arbitraryV1 := "1:name=systemd:/some/other/tracking/path\n" + endpointPlacement
	if !cgroupPlacementMatches([]byte(arbitraryV1), endpointPlacement) {
		t.Fatal("the controller-less v1 tracking line grants no confinement and must not affect the cgroup2 decision")
	}
}

func TestCgroupPlacementRefusesTamperedUnifiedPath(t *testing.T) {
	for _, cgroup := range []string{
		"0::/system.slice/evil.service\n",
		"1:name=systemd:/system.slice/ardents-endpoint.service\n0::/system.slice/evil.service\n",
		"0::/system.slice/ardents-endpoint.service/extra\n",
		"0::/\n",
	} {
		if cgroupPlacementMatches([]byte(cgroup), endpointPlacement) {
			t.Fatalf("tampered cgroup2 placement must refuse: %q", cgroup)
		}
	}
}

func TestCgroupPlacementRefusesMissingOrRepeatedUnifiedLine(t *testing.T) {
	for _, cgroup := range []string{
		"",
		"1:name=systemd:/system.slice/ardents-endpoint.service\n",
		"0::/system.slice/ardents-endpoint.service\n0::/system.slice/ardents-endpoint.service\n",
		"0::/system.slice/ardents-endpoint.service\n0::/system.slice/evil.service\n",
	} {
		if cgroupPlacementMatches([]byte(cgroup), endpointPlacement) {
			t.Fatalf("missing or repeated unified line must refuse: %q", cgroup)
		}
	}
}
