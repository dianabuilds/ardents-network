package deployment

import (
	"encoding/json"
	"path"
	"slices"

	"github.com/dianabuilds/ardents-network/internal/successor/systemdproperty"
)

// CheckInfrastructureService compares detached manager properties to the fixed
// selected infrastructure unit. The role owner supplies its own exact paths and
// program identity; this observation grants no process or startup authority.
// Section and property identify a specific mismatch when available.
func CheckInfrastructureService(unit, service systemdproperty.Properties, role, root, generation, programName string, writePaths []string) (section, property string, ok bool) {
	account := "ardents-node"
	switch role {
	case "source":
		account = "ardents-source"
	case "join", "descriptor", "introduction", "forwarding":
	default:
		return "", "", false
	}
	unitName := "ardents-" + role + ".service"

	for name, wanted := range map[string]string{"Id": unitName, "LoadState": "loaded", "FragmentPath": path.Join("/etc/systemd/system", unitName), "UnitFileState": "static"} {
		if !systemdproperty.Matches(unit, name, "s", wanted) {
			return "Unit", name, false
		}
	}
	for name, wanted := range map[string]struct {
		signature string
		value     any
	}{
		"DropInPaths": {"as", []string{}}, "Job": {"(uo)", []any{uint32(0), "/"}},
	} {
		if !systemdproperty.Matches(unit, name, wanted.signature, wanted.value) {
			return "Unit", name, false
		}
	}
	for name, wanted := range map[string]string{"User": account, "Group": account, "Type": "exec", "WorkingDirectory": "/", "ProtectHome": "yes", "ProtectSystem": "strict", "KillMode": "control-group", "Restart": "no", "RootDirectory": "", "RootImage": "", "ExitType": "main", "RestartMode": "normal", "StandardOutput": "journal", "StandardError": "journal"} {
		if !systemdproperty.Matches(service, name, "s", wanted) {
			return "Service", name, false
		}
	}
	for name, wanted := range map[string]uint64{"CapabilityBoundingSet": 0, "AmbientCapabilities": 0, "LimitCORE": 0, "LimitCORESoft": 0, "LimitNOFILE": 1024, "LimitNOFILESoft": 1024, "CPUQuotaPerSecUSec": 1_000_000, "CPUQuotaPeriodUSec": 100_000, "MemoryHigh": 805306368, "MemoryMax": 1073741824, "TasksMax": 256, "TimeoutStopUSec": 30_000_000} {
		if !systemdproperty.Matches(service, name, "t", wanted) {
			return "Service", name, false
		}
	}
	for _, name := range []string{"NoNewPrivileges", "PrivateTmp", "ProtectControlGroups", "ProtectKernelTunables", "ProtectKernelModules", "ProtectKernelLogs", "RestrictSUIDSGID", "LockPersonality", "MemoryAccounting", "CPUAccounting", "TasksAccounting"} {
		if !systemdproperty.Matches(service, name, "b", true) {
			return "Service", name, false
		}
	}
	if !systemdproperty.Matches(service, "UMask", "u", uint32(0077)) || !systemdproperty.Matches(service, "RemainAfterExit", "b", false) || !systemdproperty.Matches(service, "SupplementaryGroups", "as", []string{}) || !systemdproperty.Matches(service, "ReadWritePaths", "as", writePaths) || !systemdproperty.Matches(service, "Environment", "as", []string{"GOMAXPROCS=1", "GOMEMLIMIT=536870912"}) {
		return "", "", false
	}
	for _, name := range []string{"ExecCondition", "ExecStartPre", "ExecStartPost", "ExecReload", "ExecStop", "ExecStopPost"} {
		if !systemdproperty.Matches(service, name, "a(sasbttttuii)", []any{}) {
			return "", "", false
		}
	}
	families := service["RestrictAddressFamilies"]
	var parts []json.RawMessage
	var allow bool
	var names []string
	if families.Type != "(bas)" || systemdproperty.Decode(families.Data, &parts) != nil || len(parts) != 2 || systemdproperty.Decode(parts[0], &allow) != nil || !allow || systemdproperty.Decode(parts[1], &names) != nil {
		return "", "", false
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"AF_INET", "AF_INET6", "AF_UNIX"}) {
		return "", "", false
	}
	exec := service["ExecStartEx"]
	var entries [][]json.RawMessage
	if exec.Type != "a(sasasttttuii)" || systemdproperty.Decode(exec.Data, &entries) != nil || len(entries) != 1 || len(entries[0]) != 10 {
		return "", "", false
	}
	program := path.Join(root, "generations", generation, programName)
	for index, wanted := range map[int]any{0: program, 1: []string{program, role, "start-installed", path.Join(root, "binding.json")}, 2: []string{"no-env-expand"}} {
		p := systemdproperty.Properties{"value": {Type: "v", Data: entries[0][index]}}
		if !systemdproperty.Matches(p, "value", "v", wanted) {
			return "", "", false
		}
	}
	return "", "", true
}
