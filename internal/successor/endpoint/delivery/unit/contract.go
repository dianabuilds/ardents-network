package unit

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
	"path"
	"reflect"
	"slices"
	"strings"
)

type managerValue = systemd.Value
type managerProperties = systemd.Properties

// Stopped configuration admission is not a running MainPID/InvocationID proof,
// a qualified worker or capability-profile repair. Both known manager profiles
// retain their current parent-lifetime refusal; no newer profile is admitted.
func VerifyFreshStopped(version string, unit, service managerProperties, request Configuration, generationDigest string) error {
	if err := VerifyConfiguration(version, unit, service, request, generationDigest); err != nil {
		return err
	}
	if !systemd.Matches(unit, "ActiveState", "s", "inactive") || !systemd.Matches(unit, "SubState", "s", "dead") ||
		!systemd.Matches(service, "MainPID", "u", uint32(0)) {
		return ErrBinding
	}
	return verifyStoppedEndpointExec(service, request.InstallationRoot, generationDigest)
}

// Configuration is shared by stopped and live observations, while each caller
// separately requires its exact process state. It grants no invocation proof.
func VerifyConfiguration(version string, unit, service managerProperties, request Configuration, generationDigest string) error {
	major, _, _ := strings.Cut(version, ".")
	if major != "249" && major != "255" || !canonicalDigest(generationDigest) {
		return ErrNativeUnavailable
	}
	for name, wanted := range map[string]string{"Id": "ardents-endpoint.service", "LoadState": "loaded", "FragmentPath": "/etc/systemd/system/ardents-endpoint.service"} {
		if !systemd.Matches(unit, name, "s", wanted) {
			return ErrBinding
		}
	}
	if !systemd.Matches(unit, "DropInPaths", "as", []string{}) ||
		!systemd.Contains(unit, "Requires", []string{"ardents-text-reader.socket", "ardents-text-publisher.socket"}) {
		return ErrBinding
	}
	for name, wanted := range map[string]string{"User": "ardents-endpoint", "Group": "ardents-endpoint", "Type": "exec", "WorkingDirectory": "/", "ProtectHome": "yes", "ProtectSystem": "strict", "KillMode": "control-group", "Restart": "no", "RootDirectory": "", "RootImage": ""} {
		if !systemd.Matches(service, name, "s", wanted) {
			return ErrBinding
		}
	}
	for name, wanted := range map[string]string{"ExitType": "main", "RestartMode": "normal"} {
		if _, present := service[name]; !present && major == "249" {
			continue
		}
		if !systemd.Matches(service, name, "s", wanted) {
			return ErrBinding
		}
	}
	paths := request.WritePaths
	if !systemd.Matches(service, "UMask", "u", uint32(0077)) ||
		!systemd.Matches(service, "SupplementaryGroups", "as", []string{}) || !systemd.Matches(service, "ReadWritePaths", "as", paths) {
		return ErrBinding
	}
	for _, name := range []string{"CapabilityBoundingSet", "AmbientCapabilities", "LimitCORE"} {
		if !systemd.Matches(service, name, "t", uint64(0)) {
			return ErrBinding
		}
	}
	if !systemd.Matches(service, "TimeoutStopUSec", "t", uint64(30_000_000)) {
		return ErrBinding
	}
	for _, name := range []string{"NoNewPrivileges", "PrivateTmp", "ProtectControlGroups", "ProtectKernelTunables", "ProtectKernelModules", "ProtectKernelLogs", "RestrictSUIDSGID", "LockPersonality", "MemoryAccounting", "CPUAccounting", "TasksAccounting"} {
		if !systemd.Matches(service, name, "b", true) {
			return ErrBinding
		}
	}
	if !systemd.Matches(service, "RemainAfterExit", "b", false) {
		return ErrBinding
	}
	for _, name := range []string{"ExecCondition", "ExecStartPre", "ExecStartPost", "ExecReload", "ExecStop", "ExecStopPost"} {
		if !systemd.Matches(service, name, "a(sasbttttuii)", []any{}) {
			return ErrBinding
		}
	}
	families, found := service["RestrictAddressFamilies"]
	var parts []json.RawMessage
	var allow bool
	var names []string
	if !found || families.Type != "(bas)" || systemd.Decode(families.Data, &parts) != nil || len(parts) != 2 ||
		systemd.Decode(parts[0], &allow) != nil || !allow || systemd.Decode(parts[1], &names) != nil {
		return ErrBinding
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"AF_INET", "AF_INET6", "AF_UNIX"}) {
		return ErrBinding
	}
	_, err := endpointExecObservation(service, request.InstallationRoot, generationDigest)
	return err
}

func verifyStoppedEndpointExec(properties managerProperties, root, generationDigest string) error {
	parts, err := endpointExecObservation(properties, root, generationDigest)
	if err != nil {
		return err
	}
	// Fresh initial provisioning has never started this command. Retained process
	// timestamps or a nonzero PID are not evidence of a stopped fresh operation.
	for _, part := range parts[3:] {
		if !systemd.Matches(managerProperties{"value": {Type: "t", Data: part}}, "value", "t", uint64(0)) {
			return ErrBinding
		}
	}
	return nil
}

func endpointExecObservation(properties managerProperties, root, generationDigest string) ([]json.RawMessage, error) {
	value, found := properties["ExecStartEx"]
	var entries [][]json.RawMessage
	if !found || value.Type != "a(sasasttttuii)" || systemd.Decode(value.Data, &entries) != nil || len(entries) != 1 || len(entries[0]) != 10 {
		return nil, ErrBinding
	}
	program := path.Join(root, "generations", generationDigest, "ardents-linux-amd64")
	for index, wanted := range map[int]any{0: program, 1: []string{program, "endpoint", "start-installed", root}, 2: []string{"no-env-expand"}} {
		var actual, expected any
		encoded, err := json.Marshal(wanted)
		if err != nil || systemd.Decode(entries[0][index], &actual) != nil || systemd.Decode(encoded, &expected) != nil || !reflect.DeepEqual(actual, expected) {
			return nil, ErrBinding
		}
	}
	return entries[0], nil
}

func Invocation(unit, service managerProperties) (uint32, [16]byte, error) {
	var pid uint32
	var invocation [16]byte
	main := service["MainPID"]
	value := unit["InvocationID"]
	var parts []json.RawMessage
	if main.Type != "u" || systemd.Decode(main.Data, &pid) != nil || pid == 0 ||
		value.Type != "ay" || systemd.Decode(value.Data, &parts) != nil || len(parts) != len(invocation) {
		return 0, invocation, ErrBinding
	}
	for index, part := range parts {
		var number uint8
		if systemd.Decode(part, &number) != nil ||
			!systemd.Matches(managerProperties{"value": {Type: "y", Data: part}}, "value", "y", number) {
			return 0, [16]byte{}, ErrBinding
		}
		invocation[index] = number
	}
	if invocation == [16]byte{} {
		return 0, invocation, ErrBinding
	}
	return pid, invocation, nil
}

// This validates typed live observations against the original identity. Native
// process bytes, credentials and kernel scope still require separate checks.
func VerifyRunning(version string, unit, service managerProperties, request Configuration, generationDigest string, pid uint32, invocation [16]byte) error {
	if pid == 0 || invocation == [16]byte{} {
		return ErrBinding
	}
	if err := VerifyConfiguration(version, unit, service, request, generationDigest); err != nil {
		return err
	}
	if !systemd.Matches(unit, "ActiveState", "s", "active") || !systemd.Matches(unit, "SubState", "s", "running") ||
		!systemd.Matches(unit, "InvocationID", "ay", invocation) || !systemd.Matches(service, "MainPID", "u", pid) ||
		!systemd.Matches(service, "ControlGroup", "s", "/system.slice/ardents-endpoint.service") {
		return ErrBinding
	}
	parts, err := endpointExecObservation(service, request.InstallationRoot, generationDigest)
	if err != nil {
		return err
	}
	// Both start clocks identify an actually started execution. Stop clocks,
	// exit result and status remain zero for this exact running invocation.
	for _, index := range []int{3, 4} {
		var number uint64
		if systemd.Decode(parts[index], &number) != nil || number == 0 {
			return ErrBinding
		}
	}
	for _, index := range []int{5, 6, 8, 9} {
		if !systemd.Matches(managerProperties{"value": {Type: "t", Data: parts[index]}}, "value", "t", uint64(0)) {
			return ErrBinding
		}
	}
	if !systemd.Matches(managerProperties{"value": {Type: "u", Data: parts[7]}}, "value", "u", pid) {
		return ErrBinding
	}
	return nil
}

// A retained stopped predecessor may have historical execution clocks/PID and
// exit status. They are typed observations, not a current process or successful
// execution proof. Fresh initial provisioning keeps its never-started rule.
func VerifyQuiescent(version string, unit, service managerProperties, request Configuration, generationDigest string) error {
	if err := VerifyConfiguration(version, unit, service, request, generationDigest); err != nil {
		return err
	}
	if !systemd.Matches(unit, "ActiveState", "s", "inactive") || !systemd.Matches(unit, "SubState", "s", "dead") ||
		!systemd.Matches(service, "MainPID", "u", uint32(0)) {
		return ErrBinding
	}
	parts, err := endpointExecObservation(service, request.InstallationRoot, generationDigest)
	if err != nil {
		return err
	}
	for index := 3; index < len(parts); index++ {
		value := managerValue{Data: parts[index]}
		var expected any
		switch {
		case index < 7:
			var number uint64
			if systemd.Decode(value.Data, &number) != nil {
				return ErrBinding
			}
			value.Type, expected = "t", number
		case index == 7:
			var number uint32
			if systemd.Decode(value.Data, &number) != nil {
				return ErrBinding
			}
			value.Type, expected = "u", number
		default:
			var number int32
			if systemd.Decode(value.Data, &number) != nil {
				return ErrBinding
			}
			value.Type, expected = "i", number
		}
		if !systemd.Matches(managerProperties{"value": value}, "value", value.Type, expected) {
			return ErrBinding
		}
	}
	return nil
}

// VerifyStoppedAttempt observes physical termination of an attempted start.
// A successful Stop may leave a failed service in failed/failed. This separate
// check retains that failure; it grants no fresh provisioning, recovery or
// running admission. The caller still joins original scopes and activation.
func VerifyStoppedAttempt(version string, unit, service managerProperties, request Configuration, generationDigest string) error {
	if err := VerifyNoJob(unit); err != nil {
		return err
	}
	if systemd.Matches(unit, "ActiveState", "s", "inactive") {
		return VerifyQuiescent(version, unit, service, request, generationDigest)
	}
	if err := VerifyConfiguration(version, unit, service, request, generationDigest); err != nil {
		return err
	}
	if !systemd.Matches(unit, "ActiveState", "s", "failed") || !systemd.Matches(unit, "SubState", "s", "failed") ||
		!systemd.Matches(service, "MainPID", "u", uint32(0)) {
		return ErrBinding
	}
	parts, err := endpointExecObservation(service, request.InstallationRoot, generationDigest)
	if err != nil {
		return err
	}
	var clocks [4]uint64
	for index := range clocks {
		if systemd.Decode(parts[index+3], &clocks[index]) != nil || clocks[index] == 0 ||
			!systemd.Matches(managerProperties{"clock": {Type: "t", Data: parts[index+3]}}, "clock", "t", clocks[index]) {
			return ErrBinding
		}
	}
	if clocks[2] < clocks[0] || clocks[3] < clocks[1] {
		return ErrBinding
	}
	var pid uint32
	var code, status int32
	if systemd.Decode(parts[7], &pid) != nil || pid == 0 ||
		!systemd.Matches(managerProperties{"pid": {Type: "u", Data: parts[7]}}, "pid", "u", pid) ||
		systemd.Decode(parts[8], &code) != nil || systemd.Decode(parts[9], &status) != nil || status <= 0 ||
		!systemd.Matches(managerProperties{"code": {Type: "i", Data: parts[8]}}, "code", "i", code) ||
		!systemd.Matches(managerProperties{"status": {Type: "i", Data: parts[9]}}, "status", "i", status) {
		return ErrBinding
	}
	result := ""
	switch code {
	case 1:
		result = "exit-code"
	case 2:
		result = "signal"
	case 3:
		result = "core-dump"
	default:
		return ErrBinding
	}
	if !systemd.Matches(service, "Result", "s", result) {
		return ErrBinding
	}
	return nil
}

// MainPID zero or inactive alone cannot complete a queued manager operation.
// The typed fixed-unit Job property must describe no queued job as well.
func VerifyNoJob(unit managerProperties) error {
	if !systemd.Matches(unit, "Job", "(uo)", []any{uint32(0), "/"}) {
		return ErrBinding
	}
	return nil
}

func VerifyListeningSocket(unit, socket managerProperties, role string) error {
	return verifyActivationSocketState(unit, socket, role, "active", "listening")
}

func VerifyStoppedSocket(unit, socket managerProperties, role string) error {
	return verifyActivationSocketState(unit, socket, role, "inactive", "dead")
}

func verifyActivationSocketState(unit, socket managerProperties, role, active, substate string) error {
	if role != "reader" && role != "publisher" {
		return ErrInput
	}
	name := "ardents-text-" + role + ".socket"
	for key, wanted := range map[string]string{"Id": name, "LoadState": "loaded", "FragmentPath": "/etc/systemd/system/" + name, "ActiveState": active, "SubState": substate} {
		if !systemd.Matches(unit, key, "s", wanted) {
			return ErrBinding
		}
	}
	if !systemd.Matches(unit, "DropInPaths", "as", []string{}) ||
		!systemd.Matches(unit, "PartOf", "as", []string{"ardents-endpoint.service"}) ||
		!systemd.Matches(socket, "Accept", "b", true) ||
		!systemd.Matches(socket, "SocketUser", "s", "ardents-endpoint") ||
		!systemd.Matches(socket, "SocketGroup", "s", "ardents-endpoint") ||
		!systemd.Matches(socket, "SocketMode", "u", uint32(0600)) ||
		!systemd.Matches(socket, "DirectoryMode", "u", uint32(0711)) ||
		!systemd.Matches(socket, "MaxConnections", "u", uint32(64)) ||
		!systemd.Matches(socket, "RemoveOnStop", "b", true) ||
		!systemd.Matches(socket, "Listen", "a(ss)", [][2]string{{"Stream", "/run/ardents-text/" + role + ".sock"}}) {
		return ErrBinding
	}
	for _, name := range []string{"ExecStartPre", "ExecStartPost", "ExecStopPre", "ExecStopPost"} {
		if !systemd.Matches(socket, name, "a(sasbttttuii)", []any{}) {
			return ErrBinding
		}
	}
	return nil
}

func VerifyStoppedUnit(body, unit string) error {
	expected := map[string]string{"LoadState": "loaded", "ActiveState": "inactive", "SubState": "dead", "FragmentPath": "/etc/systemd/system/" + unit, "DropInPaths": ""}
	if unit == "ardents-endpoint.service" {
		expected["MainPID"] = "0"
	} else if unit != "ardents-text-reader.socket" && unit != "ardents-text-publisher.socket" {
		return ErrBinding
	}
	seen := make(map[string]bool)
	for _, line := range strings.Split(strings.TrimSuffix(body, "\n"), "\n") {
		key, value, found := strings.Cut(line, "=")
		wanted, known := expected[key]
		if !found || !known || seen[key] || value != wanted {
			return ErrBinding
		}
		seen[key] = true
	}
	if len(seen) != len(expected) {
		return ErrBinding
	}
	return nil
}

// Configuration is a detached projection of an admitted Installation request.
// It grants no process, Release, mutation or manager authority.
type Configuration struct {
	InstallationRoot string
	WritePaths       []string
}

var (
	ErrInput             = errors.New("installation unit: invalid fixed role")
	ErrBinding           = errors.New("installation unit: observed contract differs")
	ErrNativeUnavailable = errors.New("installation unit: manager profile unavailable")
)

func canonicalDigest(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32 && hex.EncodeToString(decoded) == value
}
