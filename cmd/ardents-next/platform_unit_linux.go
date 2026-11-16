//go:build linux

package main

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
)

// The signed template runs the existing runtime directly. systemd owns process
// start/stop; no Ardents supervisor, startup ACK or recovery journal participates.
// An unjoined runtime is never forcibly declared complete by a stop timeout.
const platformUnitTemplate = `[Unit]
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

func platformUser(p platformLaunch) string {
	if p.Role == "source" {
		return "ardents-source"
	}
	return "ardents-node"
}

func platformRuntimeArgs(p platformLaunch) []string {
	if p.Role == "source" {
		return []string{p.Program, "source", "serve-retained", p.Configuration}
	}
	return []string{p.Program, "node", p.Role, "--retained-config", p.Configuration}
}

func renderPlatformUnit(p platformLaunch, manifest, pin string) []byte {
	runtime := platformRuntimeArgs(p)
	r := strings.NewReplacer("@ROLE@", p.Role, "@USER@", platformUser(p), "@CHECK_PROGRAM@", filepath.Join(p.Distribution, "ardents-node-linux-amd64"), "@MANIFEST@", manifest, "@SHA256@", pin, "@PROGRAM@", p.Program, "@RUNTIME@", strings.Join(runtime[1:], " "), "@WRITABLE@", strings.Join(p.WritableRoots, " "))
	return []byte(r.Replace(platformUnitTemplate))
}

func observePlatformPrestart(ctx context.Context, p platformLaunch, manifest, pin string) error {
	if err := systemd.InfrastructureSiblingsAbsent(ctx, p.Role); err != nil {
		return err
	}
	unit, service, err := systemd.Infrastructure(ctx, p.Role)
	if err != nil {
		return err
	}
	return verifyPlatformPrestart(unit, service, p, manifest, pin, uint32(os.Getpid()))
}

// Compare loaded properties, not just the on-disk unit. In particular the
// mandatory precheck cannot be ignored, replaced or run as an unrelated helper.
// Only the actual ControlPID of this start-pre invocation may complete it.
func verifyPlatformPrestart(unit, service systemd.Properties, p platformLaunch, manifest, pin string, pid uint32) error {
	if pid == 0 {
		return errPlatformInput
	}
	for name, wanted := range map[string]string{"Id": "ardents-" + p.Role + ".service", "LoadState": "loaded", "FragmentPath": "/etc/systemd/system/ardents-" + p.Role + ".service", "UnitFileState": "static", "ActiveState": "activating", "SubState": "start-pre"} {
		if !systemd.Matches(unit, name, "s", wanted) {
			return errPlatformInput
		}
	}
	if !systemd.Matches(unit, "DropInPaths", "as", []string{}) || !systemd.Matches(unit, "TriggeredBy", "as", []string{}) {
		return errPlatformInput
	}
	var invocation []uint8
	value := unit["InvocationID"]
	if value.Type != "ay" || systemd.Decode(value.Data, &invocation) != nil || len(invocation) != 16 {
		return errPlatformInput
	}
	var nonzero byte
	for _, v := range invocation {
		nonzero |= v
	}
	if nonzero == 0 {
		return errPlatformInput
	}
	for name, wanted := range map[string]string{"User": platformUser(p), "Group": platformUser(p), "Type": "exec", "WorkingDirectory": "/", "ProtectHome": "yes", "ProtectSystem": "strict", "KillMode": "control-group", "Restart": "no", "RootDirectory": "", "RootImage": "", "ExitType": "main", "StandardInput": "null", "StandardOutput": "journal", "StandardError": "journal", "ControlGroup": "/system.slice/ardents-" + p.Role + ".service"} {
		if !systemd.Matches(service, name, "s", wanted) {
			return errPlatformInput
		}
	}
	for name, wanted := range map[string]uint64{"CapabilityBoundingSet": 0, "AmbientCapabilities": 0, "LimitCORE": 0, "LimitCORESoft": 0, "LimitNOFILE": 1024, "LimitNOFILESoft": 1024, "CPUQuotaPerSecUSec": 1_000_000, "CPUQuotaPeriodUSec": 100_000, "MemoryHigh": 805306368, "MemoryMax": 1073741824, "TasksMax": 256, "TimeoutStopUSec": math.MaxUint64} {
		if !systemd.Matches(service, name, "t", wanted) {
			return errPlatformInput
		}
	}
	for _, name := range []string{"NoNewPrivileges", "PrivateTmp", "ProtectControlGroups", "ProtectKernelTunables", "ProtectKernelModules", "ProtectKernelLogs", "RestrictSUIDSGID", "LockPersonality", "MemoryAccounting", "CPUAccounting", "TasksAccounting"} {
		if !systemd.Matches(service, name, "b", true) {
			return errPlatformInput
		}
	}
	for _, name := range []string{"SendSIGKILL", "RemainAfterExit", "DynamicUser"} {
		if !systemd.Matches(service, name, "b", false) {
			return errPlatformInput
		}
	}
	if !systemd.Matches(service, "UMask", "u", uint32(0077)) || !systemd.Matches(service, "MainPID", "u", uint32(0)) || !systemd.Matches(service, "ControlPID", "u", pid) || !systemd.Matches(service, "NRestarts", "u", uint32(0)) || !systemd.Matches(service, "SupplementaryGroups", "as", []string{}) || !systemd.Matches(service, "ReadWritePaths", "as", p.WritableRoots) || !systemd.Matches(service, "Environment", "as", []string{"GOMAXPROCS=1", "GOMEMLIMIT=536870912"}) {
		return errPlatformInput
	}
	// These paths could replace or hide the verified immutable inputs or open
	// additional write access. No environment file may alter runtime placement.
	for name, signature := range map[string]string{"EnvironmentFiles": "a(sb)", "BindPaths": "a(ssbt)", "BindReadOnlyPaths": "a(ssbt)", "TemporaryFileSystem": "a(ss)", "ReadOnlyPaths": "as", "InaccessiblePaths": "as", "ExecSearchPath": "as"} {
		if !systemd.Matches(service, name, signature, []any{}) {
			return errPlatformInput
		}
	}
	for _, name := range []string{"ExecCondition", "ExecStartPost", "ExecReload", "ExecStop", "ExecStopPost"} {
		if !systemd.Matches(service, name, "a(sasbttttuii)", []any{}) {
			return errPlatformInput
		}
	}
	if !systemd.Matches(service, "RestrictAddressFamilies", "(bas)", []any{true, []string{"AF_UNIX", "AF_INET", "AF_INET6"}}) || !systemd.Matches(service, "SuccessExitStatus", "(aiai)", []any{[]int32{130}, []int32{}}) {
		return errPlatformInput
	}
	check := filepath.Join(p.Distribution, "ardents-node-linux-amd64")
	if !platformExec(service, "ExecStartEx", platformRuntimeArgs(p), []string{"no-env-expand"}) || !platformExec(service, "ExecStartPreEx", []string{check, "platform", "check", manifest, pin}, []string{"privileged", "no-env-expand"}) {
		return errPlatformInput
	}
	return nil
}

func platformExec(service systemd.Properties, name string, args, flags []string) bool {
	value := service[name]
	var rows [][]json.RawMessage
	if value.Type != "a(sasasttttuii)" || systemd.Decode(value.Data, &rows) != nil || len(rows) != 1 || len(rows[0]) != 10 {
		return false
	}
	for i, wanted := range []any{args[0], args, flags} {
		if !systemd.Matches(systemd.Properties{"v": {Type: "v", Data: rows[0][i]}}, "v", "v", wanted) {
			return false
		}
	}
	return true
}
