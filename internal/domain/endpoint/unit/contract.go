package unit

import (
	"encoding/json"
	"errors"
	"path"
	"reflect"
	"slices"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/canonicaldigest"
	"github.com/dianabuilds/ardents-network/internal/platform/systemd/observation"
)

type managerValue = observation.Value
type managerProperties = observation.Properties

// VerifyPlatformConfiguration admits the platform-managed Endpoint unit whose sole
// ExecStartPre is the original privileged platform check (F-25 pre-start trust
// transfer). It grants no installation, start, replacement or recovery authority;
// the runtime remains the sole domain admission owner.
func VerifyPlatformConfiguration(version string, unit, service managerProperties, request Configuration, generationDigest, checkProgram, manifest, pin string) error {
	return verifyEndpointConfiguration(version, unit, service, request, generationDigest, platformCheckArgs(checkProgram, manifest, pin))
}

func verifyEndpointConfiguration(version string, unit, service managerProperties, request Configuration, generationDigest string, preExec []string) error {
	major, _, _ := strings.Cut(version, ".")
	if major != "249" && major != "255" || !canonicaldigest.Valid(generationDigest) {
		return ErrNativeUnavailable
	}
	for name, wanted := range map[string]string{"Id": "ardents-endpoint.service", "LoadState": "loaded", "FragmentPath": "/etc/systemd/system/ardents-endpoint.service"} {
		if !observation.Matches(unit, name, "s", wanted) {
			return ErrBinding
		}
	}
	if !observation.Matches(unit, "DropInPaths", "as", []string{}) ||
		!observation.Contains(unit, "Requires", []string{"ardents-text-reader.socket", "ardents-text-publisher.socket"}) {
		return ErrBinding
	}
	for name, wanted := range map[string]string{"User": "ardents-endpoint", "Group": "ardents-endpoint", "Type": "exec", "WorkingDirectory": "/", "ProtectHome": "yes", "ProtectSystem": "strict", "KillMode": "control-group", "Restart": "no", "RootDirectory": "", "RootImage": ""} {
		if !observation.Matches(service, name, "s", wanted) {
			return ErrBinding
		}
	}
	for name, wanted := range map[string]string{"ExitType": "main", "RestartMode": "normal"} {
		if _, present := service[name]; !present && major == "249" {
			continue
		}
		if !observation.Matches(service, name, "s", wanted) {
			return ErrBinding
		}
	}
	paths := request.WritePaths
	if !observation.Matches(service, "UMask", "u", uint32(0077)) ||
		!observation.Matches(service, "SupplementaryGroups", "as", []string{}) || !observation.Matches(service, "ReadWritePaths", "as", paths) {
		return ErrBinding
	}
	for _, name := range []string{"CapabilityBoundingSet", "AmbientCapabilities", "LimitCORE"} {
		if !observation.Matches(service, name, "t", uint64(0)) {
			return ErrBinding
		}
	}
	if !observation.Matches(service, "TimeoutStopUSec", "t", uint64(30_000_000)) {
		return ErrBinding
	}
	for _, name := range []string{"NoNewPrivileges", "PrivateTmp", "ProtectControlGroups", "ProtectKernelTunables", "ProtectKernelModules", "ProtectKernelLogs", "RestrictSUIDSGID", "LockPersonality", "MemoryAccounting", "CPUAccounting", "TasksAccounting"} {
		if !observation.Matches(service, name, "b", true) {
			return ErrBinding
		}
	}
	if !observation.Matches(service, "RemainAfterExit", "b", false) {
		return ErrBinding
	}
	for _, name := range []string{"ExecCondition", "ExecStartPost", "ExecReload", "ExecStop", "ExecStopPost"} {
		if !observation.Matches(service, name, "a(sasbttttuii)", []any{}) {
			return ErrBinding
		}
	}
	if !platformPreExecMatches(service, preExec) {
		return ErrBinding
	}
	families, found := service["RestrictAddressFamilies"]
	var parts []json.RawMessage
	var allow bool
	var names []string
	if !found || families.Type != "(bas)" || observation.Decode(families.Data, &parts) != nil || len(parts) != 2 ||
		observation.Decode(parts[0], &allow) != nil || !allow || observation.Decode(parts[1], &names) != nil {
		return ErrBinding
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"AF_INET", "AF_INET6", "AF_UNIX"}) {
		return ErrBinding
	}
	_, err := endpointExecObservation(service, request.InstallationRoot, generationDigest)
	return err
}

// platformPreExecMatches requires exactly one privileged ExecStartPre whose argv is
// the original platform check command. The "+" prefix renders the privileged flag;
// no environment expansion, drop-in, extra hook or unrelated helper is admitted.
func platformPreExecMatches(properties managerProperties, argv []string) bool {
	value, found := properties["ExecStartPreEx"]
	var entries [][]json.RawMessage
	if !found || value.Type != "a(sasasttttuii)" || observation.Decode(value.Data, &entries) != nil || len(entries) != 1 || len(entries[0]) != 10 {
		return false
	}
	for index, wanted := range []any{argv[0], argv, []string{"privileged", "no-env-expand"}} {
		encoded, err := json.Marshal(wanted)
		if err != nil {
			return false
		}
		var actual, expected any
		if observation.Decode(entries[0][index], &actual) != nil || observation.Decode(encoded, &expected) != nil || !reflect.DeepEqual(actual, expected) {
			return false
		}
	}
	return true
}

func endpointExecObservation(properties managerProperties, root, generationDigest string) ([]json.RawMessage, error) {
	value, found := properties["ExecStartEx"]
	var entries [][]json.RawMessage
	if !found || value.Type != "a(sasasttttuii)" || observation.Decode(value.Data, &entries) != nil || len(entries) != 1 || len(entries[0]) != 10 {
		return nil, ErrBinding
	}
	program := path.Join(root, "generations", generationDigest, "ardents-linux-amd64")
	for index, wanted := range map[int]any{0: program, 1: []string{program, "endpoint", "start-installed", root}, 2: []string{"no-env-expand"}} {
		var actual, expected any
		encoded, err := json.Marshal(wanted)
		if err != nil || observation.Decode(entries[0][index], &actual) != nil || observation.Decode(encoded, &expected) != nil || !reflect.DeepEqual(actual, expected) {
			return nil, ErrBinding
		}
	}
	return entries[0], nil
}

// VerifyPlatformRunning admits the running platform-managed Endpoint invocation
// whose sole ExecStartPre is the original privileged F-25 platform check. It
// verifies the complete confinement and exact running process. It grants no
// installation, start, replacement or recovery authority.
func VerifyPlatformRunning(version string, unit, service managerProperties, request Configuration, generationDigest string, pid uint32, invocation [16]byte, checkProgram, manifest, pin string) error {
	if err := VerifyPlatformConfiguration(version, unit, service, request, generationDigest, checkProgram, manifest, pin); err != nil {
		return err
	}
	return verifyRunningInvocation(unit, service, request, generationDigest, pid, invocation)
}

// verifyRunningInvocation binds the exact started execution: active/running
// state, this InvocationID/MainPID/control group, and nonzero start clocks with
// zero stop clocks, exit result and status for this running invocation.
func verifyRunningInvocation(unit, service managerProperties, request Configuration, generationDigest string, pid uint32, invocation [16]byte) error {
	if pid == 0 || invocation == [16]byte{} {
		return ErrBinding
	}
	if !observation.Matches(unit, "ActiveState", "s", "active") || !observation.Matches(unit, "SubState", "s", "running") ||
		!observation.Matches(unit, "InvocationID", "ay", invocation) || !observation.Matches(service, "MainPID", "u", pid) ||
		!observation.Matches(service, "ControlGroup", "s", "/system.slice/ardents-endpoint.service") {
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
		if observation.Decode(parts[index], &number) != nil || number == 0 {
			return ErrBinding
		}
	}
	for _, index := range []int{5, 6, 8, 9} {
		if !observation.Matches(managerProperties{"value": {Type: "t", Data: parts[index]}}, "value", "t", uint64(0)) {
			return ErrBinding
		}
	}
	if !observation.Matches(managerProperties{"value": {Type: "u", Data: parts[7]}}, "value", "u", pid) {
		return ErrBinding
	}
	return nil
}

// VerifyNoJob checks the typed fixed-unit Job property for a queued operation.
// MainPID zero or inactive alone cannot establish that no job remains queued.
func VerifyNoJob(unit managerProperties) error {
	if !observation.Matches(unit, "Job", "(uo)", []any{uint32(0), "/"}) {
		return ErrBinding
	}
	return nil
}

func VerifyListeningSocket(unit, socket managerProperties, role string) error {
	return VerifyListeningSocketAt(unit, socket, role, "ardents-text-"+role+".socket", "/run/ardents-text/"+role+".sock")
}

// VerifyListeningSocketAt verifies an activation socket for an explicit worker
// inventory (socket unit name and listen path). The fixed Endpoint wrappers preserve
// the installed Application inventory; the qualification inventory supplies its own
// name/path. All other socket hardening is shared across inventories.
func VerifyListeningSocketAt(unit, socket managerProperties, role, socketName, listenPath string) error {
	return verifyActivationSocketState(unit, socket, role, "active", "listening", socketName, listenPath)
}

func VerifyStoppedSocket(unit, socket managerProperties, role string) error {
	return VerifyStoppedSocketAt(unit, socket, role, "ardents-text-"+role+".socket", "/run/ardents-text/"+role+".sock")
}

func VerifyStoppedSocketAt(unit, socket managerProperties, role, socketName, listenPath string) error {
	return verifyActivationSocketState(unit, socket, role, "inactive", "dead", socketName, listenPath)
}

func verifyActivationSocketState(unit, socket managerProperties, role, active, substate, socketName, listenPath string) error {
	if role != "reader" && role != "publisher" {
		return ErrInput
	}
	name := socketName
	for key, wanted := range map[string]string{"Id": name, "LoadState": "loaded", "FragmentPath": "/etc/systemd/system/" + name, "ActiveState": active, "SubState": substate} {
		if !observation.Matches(unit, key, "s", wanted) {
			return ErrBinding
		}
	}
	if !observation.Matches(unit, "DropInPaths", "as", []string{}) ||
		!observation.Matches(unit, "PartOf", "as", []string{"ardents-endpoint.service"}) ||
		!observation.Matches(socket, "Accept", "b", true) ||
		!observation.Matches(socket, "SocketUser", "s", "ardents-endpoint") ||
		!observation.Matches(socket, "SocketGroup", "s", "ardents-endpoint") ||
		!observation.Matches(socket, "SocketMode", "u", uint32(0600)) ||
		!observation.Matches(socket, "DirectoryMode", "u", uint32(0711)) ||
		!observation.Matches(socket, "MaxConnections", "u", uint32(64)) ||
		!observation.Matches(socket, "RemoveOnStop", "b", true) ||
		!observation.Matches(socket, "Listen", "a(ss)", [][2]string{{"Stream", listenPath}}) {
		return ErrBinding
	}
	for _, name := range []string{"ExecStartPre", "ExecStartPost", "ExecStopPre", "ExecStopPost"} {
		if !observation.Matches(socket, name, "a(sasbttttuii)", []any{}) {
			return ErrBinding
		}
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
