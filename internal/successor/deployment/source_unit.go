package deployment

// SourceTemplate is the exact release resource. Future rendering is restricted
// to one executable/binding and the two independently admitted writable roots.
// It deliberately has no Endpoint policy switch or arbitrary service selector.
const SourceTemplate = `[Unit]
Description=Ardents Direct-Origin Source
After=network-online.target
Wants=network-online.target
[Service]
Type=exec
User=ardents-source
Group=ardents-source
SupplementaryGroups=
ExecStart=:@ARDENTS_SOURCE_PROGRAM@ source start-installed @ARDENTS_SOURCE_BINDING@
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
ReadWritePaths=@ARDENTS_SOURCE_WRITE_PATHS@
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
