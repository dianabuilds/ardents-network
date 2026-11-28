package retiredservice

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/platform/systemd/observation"
)

// NodeDeclaration checks the retained declaration bound to a Node role.
func NodeDeclaration(service observation.Properties, declaration string, role string) bool {
	value := service["ExecStartEx"]
	var rows [][]json.RawMessage
	if value.Type != "a(sasasttttuii)" || observation.Decode(value.Data, &rows) != nil || len(rows) != 1 || len(rows[0]) != 10 {
		return false
	}
	var program string
	var args, flags []string
	if observation.Decode(rows[0][0], &program) != nil || observation.Decode(rows[0][1], &args) != nil || observation.Decode(rows[0][2], &flags) != nil {
		return false
	}
	if !strings.HasPrefix(program, "/usr/lib/ardents/") || filepath.Clean(program) != program || len(args) != 5 || args[0] != program || args[1] != "node" || args[2] != role || args[3] != "--retained-config" || args[4] != declaration {
		return false
	}
	return len(flags) == 1 && flags[0] == "no-env-expand"
}

// NodeStopped checks the fixed stopped unit and its retained writable roots.
func NodeStopped(unit, service observation.Properties, writableRoots []string, role string) bool {
	unitName := "ardents-" + role + ".service"
	for name, value := range map[string]string{"Id": unitName, "LoadState": "loaded", "FragmentPath": "/etc/systemd/system/" + unitName} {
		if !observation.Matches(unit, name, "s", value) {
			return false
		}
	}
	if !(observation.Matches(unit, "ActiveState", "s", "inactive") && observation.Matches(unit, "SubState", "s", "dead")) &&
		!(observation.Matches(unit, "ActiveState", "s", "failed") && observation.Matches(unit, "SubState", "s", "failed")) {
		return false
	}
	if !observation.Matches(unit, "DropInPaths", "as", []string{}) || !observation.Matches(unit, "TriggeredBy", "as", []string{}) {
		return false
	}
	for name, value := range map[string]string{"User": "ardents-node", "Group": "ardents-node", "KillMode": "control-group", "Restart": "no", "ProtectSystem": "strict"} {
		if !observation.Matches(service, name, "s", value) {
			return false
		}
	}
	for _, name := range []string{"MainPID", "ControlPID"} {
		if !observation.Matches(service, name, "u", uint32(0)) {
			return false
		}
	}
	for _, name := range []string{"NoNewPrivileges", "ProtectControlGroups"} {
		if !observation.Matches(service, name, "b", true) {
			return false
		}
	}
	for _, name := range []string{"SendSIGKILL", "DynamicUser", "Delegate"} {
		if !observation.Matches(service, name, "b", false) {
			return false
		}
	}
	if !observation.Matches(service, "ReadWritePaths", "as", writableRoots) {
		return false
	}
	if !observation.Matches(service, "ControlGroup", "s", "") && !observation.Matches(service, "ControlGroup", "s", "/system.slice/"+unitName) {
		return false
	}
	return true
}
