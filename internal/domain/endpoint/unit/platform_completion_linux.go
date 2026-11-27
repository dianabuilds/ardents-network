package unit

import "github.com/dianabuilds/ardents-network/internal/platform/systemd/observation"

// VerifyPlatformCompleted requires a retained successful execution of the
// exact loaded platform configuration. An unloaded or never-started unit
// has no exit evidence. Physical scope and socket observations are separate.
func VerifyPlatformCompleted(version string, unit, service managerProperties, request Configuration, generationDigest, checkProgram, manifest, pin string) error {
	if err := VerifyPlatformConfiguration(version, unit, service, request, generationDigest, checkProgram, manifest, pin); err != nil {
		return err
	}
	if err := VerifyNoJob(unit); err != nil {
		return err
	}
	if !observation.Matches(unit, "ActiveState", "s", "inactive") || !observation.Matches(unit, "SubState", "s", "dead") ||
		!observation.Matches(service, "MainPID", "u", uint32(0)) || !observation.Matches(service, "Result", "s", "success") {
		return ErrBinding
	}
	parts, err := endpointExecObservation(service, request.InstallationRoot, generationDigest)
	if err != nil {
		return err
	}
	var clocks [4]uint64
	for index := range clocks {
		if observation.Decode(parts[index+3], &clocks[index]) != nil || clocks[index] == 0 {
			return ErrBinding
		}
	}
	var pid uint32
	if clocks[2] < clocks[0] || clocks[3] < clocks[1] ||
		observation.Decode(parts[7], &pid) != nil || pid == 0 ||
		!observation.Matches(managerProperties{"code": {Type: "i", Data: parts[8]}}, "code", "i", int32(1)) ||
		!observation.Matches(managerProperties{"status": {Type: "i", Data: parts[9]}}, "status", "i", int32(0)) {
		return ErrBinding
	}
	return nil
}
