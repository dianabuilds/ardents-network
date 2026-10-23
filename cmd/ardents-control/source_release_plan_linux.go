package main

import (
	"bytes"
	"debug/buildinfo"
	"debug/elf"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
)

// A distinct closed initial Source purpose. Endpoint metadata plans/receipts
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

func decodeSourceReleasePurpose(raw []byte, now time.Time, forward bool) (sourceReleasePlan, error) {
	var plan sourceReleasePlan
	schema := "ardents-source-initial-release-plan-v1"
	if forward {
		schema = "ardents-source-forward-release-plan-v1"
	}
	if len(raw) == 0 || len(raw) > 1<<20 || sourceDeclarationObject(raw, &plan, "schema", "network", "reference_time", "expires", "program", "descriptor", "template", "program_custom", "generation_custom") != nil ||
		plan.Schema != schema || plan.Network == "" || plan.Reference.IsZero() || !plan.Expires.After(now) || !plan.Expires.After(plan.Reference) || plan.Expires.Sub(plan.Reference) > 7*24*time.Hour ||
		plan.Reference.Before(now.Add(-5*time.Minute)) || plan.Reference.After(now.Add(time.Minute)) {
		return sourceReleasePlan{}, errSourceControlInput
	}
	for _, path := range []string{plan.Program, plan.Descriptor, plan.Template} {
		if !sourceControlPath(path) {
			return sourceReleasePlan{}, errSourceControlInput
		}
	}
	return plan, nil
}

func sourceDeclarationObject(raw []byte, value any, fields ...string) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	first, err := d.Token()
	if err != nil || first != json.Delim('{') {
		return errSourceControlInput
	}
	wanted, seen := map[string]bool{}, map[string]bool{}
	for _, field := range fields {
		wanted[field] = true
	}
	for d.More() {
		token, err := d.Token()
		field, ok := token.(string)
		if err != nil || !ok || !wanted[field] || seen[field] {
			return errSourceControlInput
		}
		seen[field] = true
		var body json.RawMessage
		if d.Decode(&body) != nil || bytes.Equal(bytes.TrimSpace(body), []byte("null")) {
			return errSourceControlInput
		}
	}
	last, err := d.Token()
	if err != nil || last != json.Delim('}') || len(seen) != len(wanted) {
		return errSourceControlInput
	}
	if _, err := d.Token(); err != io.EOF {
		return errSourceControlInput
	}
	if json.Unmarshal(raw, value) != nil {
		return errSourceControlInput
	}
	return nil
}

func validateSourceReleaseResources(plan sourceReleasePlan, program, descriptor, template []byte) error {
	d, err := infrastructure.ValidateResources(descriptor, program, template)
	if err != nil || (plan.Schema == "ardents-source-initial-release-plan-v1" && d.ReleaseVersion != 1) || !bytes.Equal(template, []byte(infrastructure.SourceTemplate)) {
		return errors.Join(errSourceControlInput, err)
	}
	info, err := buildinfo.Read(bytes.NewReader(program))
	if err != nil || info.Path != "github.com/dianabuilds/ardents-network/cmd/ardents-node" {
		return errors.Join(errSourceControlInput, err)
	}
	settings := map[string]string{}
	for _, setting := range info.Settings {
		settings[setting.Key] = setting.Value
	}
	if settings["GOOS"] != "linux" || settings["GOARCH"] != "amd64" || settings["CGO_ENABLED"] != "0" {
		return errSourceControlInput
	}
	image, err := elf.NewFile(bytes.NewReader(program))
	if err != nil {
		return err
	}
	if image.Class != elf.ELFCLASS64 || image.Machine != elf.EM_X86_64 {
		return errors.Join(errSourceControlInput, image.Close())
	}
	for _, header := range image.Progs {
		if header.Type == elf.PT_INTERP {
			return errors.Join(errSourceControlInput, image.Close())
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
			return errSourceControlInput
		}
		var fields map[string]json.RawMessage
		if json.Unmarshal(raw, &fields) != nil || fields == nil {
			return errSourceControlInput
		}
		delete(fields, "builder_attestations")
		var err error
		common[i], err = json.Marshal(fields)
		if err != nil {
			return err
		}
	}
	if !bytes.Equal(common[0], common[1]) {
		return errSourceControlInput
	}
	return nil
}
