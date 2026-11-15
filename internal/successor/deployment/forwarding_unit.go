package deployment

// ForwardingTemplate is the fixed forwarding service release resource.
const ForwardingTemplate = `[Unit]
Description=Ardents Forwarding Node
After=network-online.target
Wants=network-online.target
[Service]
Type=exec
User=ardents-node
Group=ardents-node
SupplementaryGroups=
ExecStart=:@ARDENTS_FORWARDING_PROGRAM@ forwarding start-installed @ARDENTS_FORWARDING_BINDING@
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
ReadWritePaths=@ARDENTS_FORWARDING_WRITE_PATHS@
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
