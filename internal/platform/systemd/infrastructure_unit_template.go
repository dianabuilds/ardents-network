package systemd

// PlatformUnitTemplate is the canonical platform-led infrastructure unit
// template shared by the admitted roles. The privileged pre-start check and the
// retained runtime command are substituted per role at rendering time; the
// signed release resource is this exact template byte-for-byte. Source release
// signing validates against it (the transitional SourceTemplate is retired).
const PlatformUnitTemplate = `[Unit]
Description=Ardents @ROLE@ runtime

[Service]
Type=exec
User=@USER@
Group=@USER@
SupplementaryGroups=
WorkingDirectory=/
ExecStartPre=+:@CHECK_PROGRAM@ platform check @MANIFEST@ @SHA256@
ExecStart=:@PROGRAM@ @RUNTIME@
Restart=no
KillMode=control-group
TimeoutStopSec=infinity
SendSIGKILL=no
SuccessExitStatus=130
UMask=0077
NoNewPrivileges=yes
CapabilityBoundingSet=
AmbientCapabilities=
ProtectSystem=strict
ProtectHome=yes
ReadWritePaths=@WRITABLE@
PrivateTmp=yes
ProtectControlGroups=yes
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectKernelLogs=yes
RestrictSUIDSGID=yes
LockPersonality=yes
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6
CPUAccounting=yes
MemoryAccounting=yes
TasksAccounting=yes
CPUQuota=100%
CPUQuotaPeriodSec=100ms
MemoryHigh=805306368
MemoryMax=1073741824
TasksMax=256
LimitNOFILE=1024
LimitCORE=0
Environment=GOMAXPROCS=1 GOMEMLIMIT=536870912
StandardInput=null
StandardOutput=journal
StandardError=journal
`
