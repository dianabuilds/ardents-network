//go:build linux

package main

// Shared enrolled protected-Endpoint resource fixtures for the real-host
// platform-led native qualifications (F-25 pre-start check and full launch).
// Independently spelled fixed contract bytes, validated by the production
// PrepareEndpointTemplate / fixed-resource owners, never produced from a
// renderer's property map. Worker confinement is the runtime's separate
// observation; these socket bytes match the fixed activation contract.

const endpointPlatformNativeTemplate = `[Unit]
Description=Ardents protected text Endpoint
Requires=ardents-text-reader.socket ardents-text-publisher.socket
After=network-online.target ardents-text-reader.socket ardents-text-publisher.socket
Wants=network-online.target

[Service]
Type=exec
User=ardents-endpoint
Group=ardents-endpoint
SupplementaryGroups=
ExecStart=:@ARDENTS_ENDPOINT_PROGRAM@ endpoint start-installed @ARDENTS_INSTALLATION_ROOT@
WorkingDirectory=/
UMask=0077
NoNewPrivileges=yes
CapabilityBoundingSet=
AmbientCapabilities=
PrivateTmp=yes
ProtectHome=yes
ProtectSystem=strict
ReadWritePaths=@ARDENTS_WRITE_PATHS@
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
RemainAfterExit=no
KillMode=control-group
TimeoutStopSec=30s
StandardOutput=journal
StandardError=journal
`

const endpointPlatformNativeReaderSocket = `[Unit]
Description=Ardents protected text Reader activation
PartOf=ardents-endpoint.service

[Socket]
ListenStream=/run/ardents-text/reader.sock
Accept=yes
SocketUser=ardents-endpoint
SocketGroup=ardents-endpoint
SocketMode=0600
DirectoryMode=0711
MaxConnections=64
RemoveOnStop=yes

[Install]
WantedBy=sockets.target
`

const endpointPlatformNativePublisherSocket = `[Unit]
Description=Ardents protected text Publisher activation
PartOf=ardents-endpoint.service

[Socket]
ListenStream=/run/ardents-text/publisher.sock
Accept=yes
SocketUser=ardents-endpoint
SocketGroup=ardents-endpoint
SocketMode=0600
DirectoryMode=0711
MaxConnections=64
RemoveOnStop=yes

[Install]
WantedBy=sockets.target
`

const endpointPlatformNativeReaderWorker = `[Unit]
Description=Ardents fixed text reader worker

BindsTo=ardents-endpoint.service
After=ardents-endpoint.service
CollectMode=inactive-or-failed

[Service]
Type=exec
Slice=system.slice
ExecStart=/ardents-text worker-reader
RootDirectory=/usr/lib/ardents/text-worker-root
WorkingDirectory=/
Environment=GOMAXPROCS=2 GOMEMLIMIT=96MiB
DynamicUser=yes
User=ardtxt-r-%i
SupplementaryGroups=
CapabilityBoundingSet=
AmbientCapabilities=
NoNewPrivileges=yes
PrivateNetwork=yes
PrivateIPC=yes
PrivateDevices=yes
PrivateTmp=yes
ProtectSystem=strict
ProtectHome=yes
ProtectControlGroups=yes
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectKernelLogs=yes
RestrictNamespaces=yes
RestrictSUIDSGID=yes
LockPersonality=yes
RestrictAddressFamilies=AF_UNIX
SystemCallArchitectures=native
MemoryDenyWriteExecute=yes
SystemCallFilter=~io_uring_setup io_uring_enter io_uring_register bpf ptrace process_vm_readv process_vm_writev perf_event_open keyctl add_key request_key userfaultfd @mount @module @reboot @swap @raw-io
MemoryMax=128M
TasksMax=32
LimitCORE=0
Restart=no
KillMode=control-group
TimeoutStopSec=2s
StandardInput=socket
StandardOutput=socket
StandardError=null
`

const endpointPlatformNativePublisherWorker = `[Unit]
Description=Ardents fixed text publisher worker

BindsTo=ardents-endpoint.service
After=ardents-endpoint.service
CollectMode=inactive-or-failed

[Service]
Type=exec
Slice=system.slice
ExecStart=/ardents-text worker-publisher
RootDirectory=/usr/lib/ardents/text-worker-root
WorkingDirectory=/
Environment=GOMAXPROCS=2 GOMEMLIMIT=96MiB
DynamicUser=yes
User=ardtxt-p-%i
SupplementaryGroups=
CapabilityBoundingSet=
AmbientCapabilities=
NoNewPrivileges=yes
PrivateNetwork=yes
PrivateIPC=yes
PrivateDevices=yes
PrivateTmp=yes
ProtectSystem=strict
ProtectHome=yes
ProtectControlGroups=yes
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectKernelLogs=yes
RestrictNamespaces=yes
RestrictSUIDSGID=yes
LockPersonality=yes
RestrictAddressFamilies=AF_UNIX
SystemCallArchitectures=native
MemoryDenyWriteExecute=yes
SystemCallFilter=~io_uring_setup io_uring_enter io_uring_register bpf ptrace process_vm_readv process_vm_writev perf_event_open keyctl add_key request_key userfaultfd @mount @module @reboot @swap @raw-io
MemoryMax=128M
TasksMax=32
LimitCORE=0
Restart=no
KillMode=control-group
TimeoutStopSec=2s
StandardInput=socket
StandardOutput=socket
StandardError=null
`

const endpointPlatformNativePolkit = `polkit.addRule(function(action, subject) {
    if (action.id == "org.freedesktop.systemd1.manage-units" &&
        subject.user == "ardents-endpoint") {
        return polkit.Result.NO;
    }
});
`

const endpointPlatformNativeTmpfiles = `d /run/ardents-text 0711 ardents-endpoint ardents-endpoint -
`

func endpointPlatformNativeResources() map[string][]byte {
	return map[string][]byte{
		"ardents-endpoint.service":        []byte(endpointPlatformNativeTemplate),
		"ardents-text-reader.socket":      []byte(endpointPlatformNativeReaderSocket),
		"ardents-text-publisher.socket":   []byte(endpointPlatformNativePublisherSocket),
		"ardents-text-reader@.service":    []byte(endpointPlatformNativeReaderWorker),
		"ardents-text-publisher@.service": []byte(endpointPlatformNativePublisherWorker),
		"50-ardents-text.rules":           []byte(endpointPlatformNativePolkit),
		"ardents-text.conf":               []byte(endpointPlatformNativeTmpfiles),
	}
}
