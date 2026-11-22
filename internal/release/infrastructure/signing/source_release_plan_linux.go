package signing

import (
	"bytes"
	"debug/buildinfo"
	"debug/elf"
	"encoding/json"
	"errors"
	artifactio "github.com/dianabuilds/ardents-network/internal/platform/artifacts"
	"time"

	"github.com/dianabuilds/ardents-network/internal/platform/systemd"
	"github.com/dianabuilds/ardents-network/internal/release/infrastructure"
)

type sourceReleasePurpose uint8

const (
	sourceReleaseInitial sourceReleasePurpose = iota
	sourceReleaseForward
	sourceReleaseTimestamp
)

// Distinct closed Source purposes. Endpoint metadata plans/receipts
// are not translated, and no caller chooses a target, metadata role or message.
type sourceReleasePlan struct {
	Schema           string          `json:"schema"`
	Network          string          `json:"network"`
	Reference        time.Time       `json:"reference_time"`
	Expires          time.Time       `json:"expires"`
	Program          string          `json:"program"`
	Descriptor       string          `json:"descriptor"`
	Template         string          `json:"template"`
	ProgramCustom    json.RawMessage `json:"program_custom"`
	GenerationCustom json.RawMessage `json:"generation_custom"`
}

func decodeSourceReleasePurpose(raw []byte, now time.Time, purpose sourceReleasePurpose) (sourceReleasePlan, error) {
	var plan sourceReleasePlan
	schema := "ardents-source-initial-release-plan-v1"
	switch purpose {
	case sourceReleaseInitial:
	case sourceReleaseForward:
		schema = "ardents-source-forward-release-plan-v1"
	case sourceReleaseTimestamp:
		schema = "ardents-source-timestamp-release-plan-v1"
	default:
		return sourceReleasePlan{}, artifactio.ErrInput
	}
	if len(raw) == 0 || len(raw) > 1<<20 || artifactio.DecodeDeclaration(raw, &plan, "schema", "network", "reference_time", "expires", "program", "descriptor", "template", "program_custom", "generation_custom") != nil ||
		plan.Schema != schema || plan.Network == "" || plan.Reference.IsZero() || !plan.Expires.After(now) || !plan.Expires.After(plan.Reference) || plan.Expires.Sub(plan.Reference) > 7*24*time.Hour ||
		plan.Reference.Before(now.Add(-5*time.Minute)) || plan.Reference.After(now.Add(time.Minute)) {
		return sourceReleasePlan{}, artifactio.ErrInput
	}
	for _, path := range []string{plan.Program, plan.Descriptor, plan.Template} {
		if !artifactio.AbsolutePath(path) {
			return sourceReleasePlan{}, artifactio.ErrInput
		}
	}
	return plan, nil
}

func validateSourceReleaseResources(plan sourceReleasePlan, program, descriptor, template []byte) error {
	d, err := infrastructure.ValidateResources(descriptor, program, template)
	if err != nil || (plan.Schema == "ardents-source-initial-release-plan-v1" && d.ReleaseVersion != 1) || !bytes.Equal(template, []byte(systemd.PlatformUnitTemplate)) {
		return errors.Join(artifactio.ErrInput, err)
	}
	info, err := buildinfo.Read(bytes.NewReader(program))
	if err != nil || info.Path != "github.com/dianabuilds/ardents-network/cmd/ardents-next" {
		return errors.Join(artifactio.ErrInput, err)
	}
	settings := map[string]string{}
	for _, setting := range info.Settings {
		settings[setting.Key] = setting.Value
	}
	if settings["GOOS"] != "linux" || settings["GOARCH"] != "amd64" || settings["CGO_ENABLED"] != "0" {
		return artifactio.ErrInput
	}
	image, err := elf.NewFile(bytes.NewReader(program))
	if err != nil {
		return err
	}
	if image.Class != elf.ELFCLASS64 || image.Machine != elf.EM_X86_64 {
		return errors.Join(artifactio.ErrInput, image.Close())
	}
	for _, header := range image.Progs {
		if header.Type == elf.PT_INTERP {
			return errors.Join(artifactio.ErrInput, image.Close())
		}
	}
	if err := image.Close(); err != nil {
		return err
	}
	var common [2][]byte
	for i, raw := range []json.RawMessage{plan.ProgramCustom, plan.GenerationCustom} {
		var identity struct {
			Release string `json:"release_identity"`
			Version int64  `json:"release_version"`
		}
		if json.Unmarshal(raw, &identity) != nil || identity.Release != d.ReleaseIdentity || identity.Version != d.ReleaseVersion {
			return artifactio.ErrInput
		}
		var fields map[string]json.RawMessage
		if json.Unmarshal(raw, &fields) != nil || fields == nil {
			return artifactio.ErrInput
		}
		delete(fields, "builder_attestations")
		var err error
		common[i], err = json.Marshal(fields)
		if err != nil {
			return err
		}
	}
	if !bytes.Equal(common[0], common[1]) {
		return artifactio.ErrInput
	}
	return nil
}
