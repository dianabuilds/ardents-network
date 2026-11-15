package descriptor

import (
	"encoding/json"
	posixpath "path"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/successor/deployment"
	releaseartifact "github.com/dianabuilds/ardents-network/internal/successor/release/infrastructure"
)

const programName = releaseartifact.ProgramName

func initialSHA(value string) bool {
	return len(value) == 64 && strings.Trim(value, "0123456789abcdef") == ""
}

// Rendering is closed Descriptor policy: no caller-selected argv or unit settings.
// Detached bytes grant no loaded manager, native installation or Start right.
func renderUnit(template []byte, r Request, generation string) ([]byte, error) {
	raw, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	if _, err := DecodeRequest(append(raw, '\n')); err != nil {
		return nil, err
	}
	if string(template) != deployment.DescriptorTemplate || !initialSHA(generation) {
		return nil, ErrBinding
	}
	quote := func(path string) string { return "\"" + path + "\"" }
	return []byte(strings.NewReplacer(
		"@ARDENTS_DESCRIPTOR_PROGRAM@", quote(posixpath.Join(r.InstallationRoot, "generations", generation, programName)),
		"@ARDENTS_DESCRIPTOR_BINDING@", quote(posixpath.Join(r.InstallationRoot, "binding.json")),
		"@ARDENTS_DESCRIPTOR_WRITE_PATHS@", quote(r.StateRoot)+" "+quote(r.ParticipationRoot)+" "+quote(r.SpendRoot)+" "+quote(r.HostingRoot)+" "+quote(r.DescriptorRoot),
	).Replace(deployment.DescriptorTemplate)), nil
}
