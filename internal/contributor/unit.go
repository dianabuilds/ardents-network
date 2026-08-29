package contributor

const systemdUnit = `[Unit]
Description=Ardents Rendezvous Contributor (functional alpha)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
DynamicUser=yes
User=ardents-contributor
Group=ardents-contributor
StateDirectory=ardents-contributor
StateDirectoryMode=0700
ExecStartPre=+/bin/chown -R -- ardents-contributor:ardents-contributor /var/lib/private/ardents-contributor
ExecStart=/usr/lib/ardents-contributor/current/ardents-node node --config /var/lib/private/ardents-contributor/config/current/node.json
StandardOutput=null
StandardError=journal
Restart=on-failure
RestartSec=5s
RestartPreventExitStatus=2
TimeoutStopSec=10s
KillSignal=SIGTERM
FinalKillSignal=SIGKILL
Environment=GOMAXPROCS=1
Environment=GOMEMLIMIT=134217728
CPUQuota=100%
MemoryHigh=192M
MemoryMax=256M
TasksMax=64
LimitNOFILE=256
UMask=0077
CapabilityBoundingSet=
AmbientCapabilities=
NoNewPrivileges=yes
PrivateDevices=yes
PrivateTmp=yes
ProtectClock=yes
ProtectControlGroups=yes
ProtectHome=yes
ProtectHostname=yes
ProtectKernelLogs=yes
ProtectKernelModules=yes
ProtectKernelTunables=yes
ProtectProc=invisible
ProcSubset=pid
ProtectSystem=strict
RestrictAddressFamilies=AF_INET AF_INET6
RestrictNamespaces=yes
RestrictRealtime=yes
LockPersonality=yes
MemoryDenyWriteExecute=yes
SystemCallArchitectures=native
SystemCallFilter=@system-service

[Install]
WantedBy=multi-user.target
`
