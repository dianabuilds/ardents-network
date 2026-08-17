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
	arguments = append(arguments[:len(arguments)-1], "--detach", "--network", "container:"+unit.endpoint,
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
	default:
		t.Fatalf("unknown capacity sidecar %q", kind)
	}
	if output, err := dockerOutput(ctx, arguments...); err != nil {
		t.Fatalf("start capacity %s %d: %v\n%s", kind, unit.index, err, output)
	}
}
