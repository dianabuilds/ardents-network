package systemd

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"sort"
)

var errInfrastructurePrestart = errors.New("platform launch refused")

// ObservePrestart verifies the loaded unit and the manager's exact current
// ControlPID against this invocation. It performs read-only manager I/O.
func (p InfrastructureUnit) ObservePrestart(ctx context.Context) error {
	if err := InfrastructureSiblingsAbsent(ctx, p.Role); err != nil {
		return err
	}
	unit, service, err := Infrastructure(ctx, p.Role)
	if err != nil {
		return err
	}
	return p.verifyPrestart(unit, service, uint32(os.Getpid()))
}

// Compare loaded properties, not just the on-disk unit. In particular the
// mandatory precheck cannot be ignored, replaced or run as an unrelated helper.
// Only the actual ControlPID of this start-pre invocation may complete it.
func (p InfrastructureUnit) verifyPrestart(unit, service Properties, pid uint32) error {
	if pid == 0 {
		return errInfrastructurePrestart
	}
	for name, wanted := range map[string]string{"Id": "ardents-" + p.Role + ".service", "LoadState": "loaded", "FragmentPath": "/etc/systemd/system/ardents-" + p.Role + ".service", "UnitFileState": "static", "ActiveState": "activating", "SubState": "start-pre"} {
		if !Matches(unit, name, "s", wanted) {
			return errInfrastructurePrestart
		}
	}
	if !Matches(unit, "DropInPaths", "as", []string{}) || !Matches(unit, "TriggeredBy", "as", []string{}) {
		return errInfrastructurePrestart
	}
	var invocation []uint8
	value := unit["InvocationID"]
	if value.Type != "ay" || Decode(value.Data, &invocation) != nil || len(invocation) != 16 {
		return errInfrastructurePrestart
	}
	var nonzero byte
	for _, v := range invocation {
		nonzero |= v
	}
	if nonzero == 0 {
		return errInfrastructurePrestart
	}
	for name, wanted := range map[string]string{"User": InfrastructureAccount(p.Role), "Group": InfrastructureAccount(p.Role), "Type": "exec", "WorkingDirectory": "/", "ProtectHome": "yes", "ProtectSystem": "strict", "KillMode": "control-group", "Restart": "no", "RootDirectory": "", "RootImage": "", "ExitType": "main", "StandardInput": "null", "StandardOutput": "journal", "StandardError": "journal", "ControlGroup": "/system.slice/ardents-" + p.Role + ".service"} {
		if !Matches(service, name, "s", wanted) {
			return errInfrastructurePrestart
		}
	}
	for name, wanted := range map[string]uint64{"CapabilityBoundingSet": 0, "AmbientCapabilities": 0, "LimitCORE": 0, "LimitCORESoft": 0, "LimitNOFILE": 1024, "LimitNOFILESoft": 1024, "CPUQuotaPerSecUSec": 1_000_000, "CPUQuotaPeriodUSec": 100_000, "MemoryHigh": 805306368, "MemoryMax": 1073741824, "TasksMax": 256, "TimeoutStopUSec": math.MaxUint64} {
		if !Matches(service, name, "t", wanted) {
			return errInfrastructurePrestart
		}
	}
	for _, name := range []string{"NoNewPrivileges", "PrivateTmp", "ProtectControlGroups", "ProtectKernelTunables", "ProtectKernelModules", "ProtectKernelLogs", "RestrictSUIDSGID", "LockPersonality", "MemoryAccounting", "CPUAccounting", "TasksAccounting"} {
		if !Matches(service, name, "b", true) {
			return errInfrastructurePrestart
		}
	}
	for _, name := range []string{"SendSIGKILL", "RemainAfterExit", "DynamicUser"} {
		if !Matches(service, name, "b", false) {
			return errInfrastructurePrestart
		}
	}
	if !Matches(service, "UMask", "u", uint32(0077)) || !Matches(service, "MainPID", "u", uint32(0)) || !Matches(service, "ControlPID", "u", pid) || !Matches(service, "NRestarts", "u", uint32(0)) || !Matches(service, "SupplementaryGroups", "as", []string{}) || !Matches(service, "ReadWritePaths", "as", p.WritableRoots) || !Matches(service, "Environment", "as", []string{"GOMAXPROCS=1", "GOMEMLIMIT=536870912"}) {
		return errInfrastructurePrestart
	}
	// These paths could replace or hide the verified immutable inputs or open
	// additional write access. No environment file may alter runtime placement.
	for name, signature := range map[string]string{"EnvironmentFiles": "a(sb)", "BindPaths": "a(ssbt)", "BindReadOnlyPaths": "a(ssbt)", "TemporaryFileSystem": "a(ss)", "ReadOnlyPaths": "as", "InaccessiblePaths": "as", "ExecSearchPath": "as"} {
		if !Matches(service, name, signature, []any{}) {
			return errInfrastructurePrestart
		}
	}
	for _, name := range []string{"ExecCondition", "ExecStartPost", "ExecReload", "ExecStop", "ExecStopPost"} {
		if !Matches(service, name, "a(sasbttttuii)", []any{}) {
			return errInfrastructurePrestart
		}
	}
	if !infrastructureAddressFamilies(service) || !Matches(service, "SuccessExitStatus", "(aiai)", []any{[]int32{130}, []int32{}}) {
		return errInfrastructurePrestart
	}
	if !infrastructureExec(service, "ExecStartEx", p.runtimeArgs(), []string{"no-env-expand"}) || !infrastructureExec(service, "ExecStartPreEx", p.prestartArgs(), []string{"privileged", "no-env-expand"}) {
		return errInfrastructurePrestart
	}
	return nil
}

// The manager reports the address-family whitelist as a sorted set, not in the
// template's spelling order. The protection is the exact admitted family set in
// whitelist mode; an order-sensitive byte comparison would refuse the genuine
// loaded policy on the real manager.
func infrastructureAddressFamilies(service Properties) bool {
	value, found := service["RestrictAddressFamilies"]
	if !found || value.Type != "(bas)" {
		return false
	}
	var pair []json.RawMessage
	if Decode(value.Data, &pair) != nil || len(pair) != 2 {
		return false
	}
	var whitelist bool
	var families []string
	if json.Unmarshal(pair[0], &whitelist) != nil || json.Unmarshal(pair[1], &families) != nil {
		return false
	}
	sort.Strings(families)
	return whitelist && len(families) == 3 && families[0] == "AF_INET" && families[1] == "AF_INET6" && families[2] == "AF_UNIX"
}

func infrastructureExec(service Properties, name string, args, flags []string) bool {
	value := service[name]
	var rows [][]json.RawMessage
	if value.Type != "a(sasasttttuii)" || Decode(value.Data, &rows) != nil || len(rows) != 1 || len(rows[0]) != 10 {
		return false
	}
	for i, wanted := range []any{args[0], args, flags} {
		if !Matches(Properties{"v": {Type: "v", Data: rows[0][i]}}, "v", "v", wanted) {
			return false
		}
	}
	return true
}
