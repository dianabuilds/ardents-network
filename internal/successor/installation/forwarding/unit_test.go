package forwarding

import (
	"strings"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
)

func TestForwardingUnitIndependentFixedPolicy(t *testing.T) {
	r, err := DecodeRequest([]byte(forwardingRequestVector))
	if err != nil {
		t.Fatal(err)
	}
	generation := strings.Repeat("a", 64)
	want := forwardingUnitVector(generation)
	unit, err := RenderUnit([]byte(Template), r, generation)
	if err != nil || string(unit) != want {
		t.Fatalf("fixed independent unit vector differs: %s: %v", unit, err)
	}
	for _, mutation := range []struct{ before, after string }{
		{"User=ardents-node", "User=ardents-source"}, {"Restart=no", "Restart=always"},
		{"MemoryMax=1073741824", "MemoryMax=2147483648"}, {"forwarding start-installed", "join start-installed"},
		{"ProtectSystem=strict", "ProtectSystem=no"}, {"KillMode=control-group", "KillMode=process"},
	} {
		template := strings.Replace(Template, mutation.before, mutation.after, 1)
		if _, err := RenderUnit([]byte(template), r, generation); err == nil {
			t.Fatal("changed fixed policy accepted", mutation)
		}
	}
	for _, template := range []string{infrastructure.SourceTemplate, Template + "\nExecStartPost=/bin/true\n"} {
		if _, err := RenderUnit([]byte(template), r, generation); err == nil {
			t.Fatal("foreign template accepted")
		}
	}
}

// Independent literal shared only by persisted-byte tests.
func forwardingUnitVector(generation string) string {
	return `[Unit]
Description=Ardents Forwarding Node
After=network-online.target
Wants=network-online.target
[Service]
Type=exec
User=ardents-node
Group=ardents-node
SupplementaryGroups=
ExecStart=:"/opt/ardents-forwarding/generations/` + generation + `/ardents-node-linux-amd64" forwarding start-installed "/opt/ardents-forwarding/binding.json"
WorkingDirectory=/
UMask=0077
Restart=no
RemainAfterExit=no
KillMode=control-group
TimeoutStopSec=30s
NoNewPrivileges=yes
CapabilityBoundingSet=
AmbientCapabilities=
PrivateTmp=yes
ProtectHome=yes
ProtectSystem=strict
ReadWritePaths="/var/lib/ardents-node-state" "/var/lib/ardents-node-participation" "/var/lib/ardents-node-spend" "/var/lib/ardents-node-hosting"
ProtectControlGroups=yes
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectKernelLogs=yes
RestrictSUIDSGID=yes
LockPersonality=yes
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6
MemoryAccounting=yes
CPUAccounting=yes
TasksAccounting=yes
CPUQuota=100%
CPUQuotaPeriodSec=100ms
MemoryHigh=805306368
MemoryMax=1073741824
TasksMax=256
LimitNOFILE=1024
LimitCORE=0
Environment=GOMAXPROCS=1 GOMEMLIMIT=536870912
StandardOutput=journal
StandardError=journal
`
}
