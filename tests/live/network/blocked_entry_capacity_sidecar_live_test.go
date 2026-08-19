//go:build live

package network_test

import (
	"context"
	"testing"
)

func startCapacitySidecar(t *testing.T, ctx context.Context, project, image string,
	unit blockedCapacityUnit, kind string,
) {
	t.Helper()
	name := unit.endpoint + "-" + kind
	arguments := capacityContainerBase("run", name, project, image)
	network := "container:" + unit.endpoint
	if kind == "resource-collector" {
		network = "none"
	}
	arguments = append(arguments[:len(arguments)-1], "--detach", "--network", network,
		"--user", "0:0", "--mount", bindMount(unit.sync, "/run/evidence", false))
	switch kind {
	case "observer":
		arguments = append(arguments, "--cap-add", "NET_RAW", "--env", "ARDENTS_DNS_OBSERVER=1",
			"--env", "ARDENTS_DNS_SYNC=/run/evidence", image,
			"/usr/local/bin/camouflage.test", "-test.run", "^$")
	case "policy":
		arguments = append(arguments, "--cap-add", "NET_ADMIN", "--env", "ARDENTS_BLOCKED_ROLE=policy",
			"--env", "ARDENTS_BLOCKED_POLICY_TARGETS=172.31.20.11", "--env", "ARDENTS_DNS_SYNC=/run/evidence",
			image, "/usr/local/bin/network-live.test", "-test.count=1", "-test.v", "-test.run", "^TestBlockedEntryRole$")
	case "resource-collector":
		arguments = append(arguments, "--pid", "container:"+unit.endpoint,
			"--env", "ARDENTS_BLOCKED_ROLE=resource-collector", "--env", "ARDENTS_DNS_SYNC=/run/evidence",
			"--env", "ARDENTS_BLOCKED_TIMELINE_MONOTONIC_ANCHOR_MILLIS="+blockedMonotonicAnchorMillis(),
			image, "/usr/local/bin/network-live.test", "-test.count=1", "-test.v", "-test.run", "^TestBlockedEntryRole$")
	case "carrier-collector":
		arguments = append(arguments, "--env", "ARDENTS_BLOCKED_ROLE=carrier-collector",
			"--env", "ARDENTS_DNS_SYNC=/run/evidence",
			"--env", "ARDENTS_BLOCKED_TIMELINE_MONOTONIC_ANCHOR_MILLIS="+blockedMonotonicAnchorMillis(),
			image, "/usr/local/bin/network-live.test", "-test.count=1", "-test.v", "-test.run", "^TestBlockedEntryRole$")
	default:
		t.Fatalf("unknown capacity sidecar %q", kind)
	}
	if output, err := dockerOutput(ctx, arguments...); err != nil {
		t.Fatalf("start capacity %s %d: %v\n%s", kind, unit.index, err, output)
	}
}
