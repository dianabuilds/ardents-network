package introduction

import (
	"encoding/json"
	posixpath "path"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
)

const programName = infrastructure.ProgramName

func initialSHA(value string) bool {
	return len(value) == 64 && strings.Trim(value, "0123456789abcdef") == ""
}

// Template is the distinct fixed Introduction resource under the common Node envelope.
// It admits no configurable command, role switch, boot or automatic restart.
const Template = `[Unit]
Description=Ardents Introduction Node
After=network-online.target
Wants=network-online.target
[Service]
Type=exec
User=ardents-node
Group=ardents-node
SupplementaryGroups=
ExecStart=:@ARDENTS_INTRODUCTION_PROGRAM@ introduction start-installed @ARDENTS_INTRODUCTION_BINDING@
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
ReadWritePaths=@ARDENTS_INTRODUCTION_WRITE_PATHS@
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

// Rendering is closed Introduction policy: no caller-selected argv or unit settings.
// Detached bytes grant no loaded manager, native installation or Start right.
func renderUnit(template []byte, r Request, generation string) ([]byte, error) {
	raw, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	if _, err := DecodeRequest(append(raw, '\n')); err != nil {
		return nil, err
	}
	if string(template) != Template || !initialSHA(generation) {
		return nil, ErrBinding
	}
	quote := func(path string) string { return "\"" + path + "\"" }
	return []byte(strings.NewReplacer(
		"@ARDENTS_INTRODUCTION_PROGRAM@", quote(posixpath.Join(r.InstallationRoot, "generations", generation, programName)),
		"@ARDENTS_INTRODUCTION_BINDING@", quote(posixpath.Join(r.InstallationRoot, "binding.json")),
		"@ARDENTS_INTRODUCTION_WRITE_PATHS@", quote(r.StateRoot)+" "+quote(r.ParticipationRoot)+" "+quote(r.SpendRoot)+" "+quote(r.HostingRoot)+" "+quote(r.IntroductionRoot),
	).Replace(Template)), nil
}
