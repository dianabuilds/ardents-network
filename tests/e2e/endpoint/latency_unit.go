package endpoint

import "fmt"

// RenderLatencyUnit renders the fixed qualification unit for the participant.
func RenderLatencyUnit(program, root, control, base string, cold, warm int, seed string) string {
	return fmt.Sprintf(`[Unit]
Description=Ardents cold/warm latency qualification Endpoint
Requires=ardents-text-reader.socket ardents-text-publisher.socket
After=network-online.target ardents-text-reader.socket ardents-text-publisher.socket
Wants=network-online.target

[Service]
Type=exec
User=ardents-endpoint
Group=ardents-endpoint
SupplementaryGroups=
ExecStart=%s -test.run=^TestNativeEndpointColdWarmLatencyParticipant$ -test.v -test.timeout=28m
Environment=ARDENTS_LATENCY_ROOT=%s
Environment=ARDENTS_LATENCY_CONTROL=%s
Environment=ARDENTS_LATENCY_COLD=%d
Environment=ARDENTS_LATENCY_WARM=%d
Environment=ARDENTS_LATENCY_SEED=%s
WorkingDirectory=/
UMask=0077
NoNewPrivileges=yes
CapabilityBoundingSet=
AmbientCapabilities=
PrivateTmp=yes
ProtectHome=yes
ProtectSystem=strict
ReadWritePaths=%s /etc/ardents
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
LimitCORE=0
Restart=no
RestartMode=normal
RemainAfterExit=no
ExitType=main
KillMode=control-group
TimeoutStopSec=30s
RuntimeMaxSec=29m
StandardOutput=journal
StandardError=journal
`, program, root, control, cold, warm, seed, base)
}
