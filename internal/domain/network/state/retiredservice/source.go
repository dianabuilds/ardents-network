package retiredservice

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/platform/systemd/observation"
)

// SourceDeclaration checks the loaded Source declaration.
func SourceDeclaration(service observation.Properties, declaration string) bool {
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
	if !strings.HasPrefix(program, "/usr/lib/ardents/") || filepath.Clean(program) != program || len(args) != 4 || args[0] != program || args[1] != "source" || args[2] != "serve-retained" || args[3] != declaration {
		return false
	}
	return len(flags) == 1 && flags[0] == "no-env-expand"
}

// SourceStopped checks the fixed stopped Source and its retained roots.
func SourceStopped(unit, service observation.Properties, stateRoot, participation string) bool {
	for name, value := range map[string]string{"Id": "ardents-source.service", "LoadState": "loaded", "FragmentPath": "/etc/systemd/system/ardents-source.service"} {
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
	for name, value := range map[string]string{"User": "ardents-source", "Group": "ardents-source", "KillMode": "control-group", "Restart": "no", "ProtectSystem": "strict"} {
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
	if !observation.Matches(service, "ReadWritePaths", "as", []string{stateRoot, participation}) {
		return false
	}
	if !observation.Matches(service, "ControlGroup", "s", "") && !observation.Matches(service, "ControlGroup", "s", "/system.slice/ardents-source.service") {
		return false
	}
	return true
}
