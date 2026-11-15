package deployment

// These fixed systemd settings are identical for the selected infrastructure
// services. Each caller still supplies its own literal command and write roots.
const serviceBeforeWritePaths = `WorkingDirectory=/
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
`

const serviceAfterWritePaths = `ProtectControlGroups=yes
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
