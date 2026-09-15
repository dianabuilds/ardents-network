package resource

import "testing"

func TestCgroupV2ProcessPathSelectsExactUnifiedHierarchy(t *testing.T) {
	for _, test := range []struct {
		name, raw, want string
		valid           bool
	}{
		{name: "host systemd unit", raw: "0::/system.slice/ardents-rendezvous-contributor.service\n", want: "/system.slice/ardents-rendezvous-contributor.service", valid: true},
		{name: "private cgroup namespace", raw: "0::/\n", want: "/", valid: true},
		{name: "legacy hierarchy", raw: "2:cpu:/service\n", valid: false},
		{name: "relative", raw: "0::service\n", valid: false},
		{name: "traversal", raw: "0::/system.slice/../user.slice\n", valid: false},
		{name: "duplicate", raw: "0::/first\n0::/second\n", valid: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := cgroupV2ProcessPath(test.raw)
			if (err == nil) != test.valid || got != test.want {
				t.Fatalf("path = %q, %v; want %q, valid=%t", got, err, test.want, test.valid)
			}
		})
	}
}
