package main

import (
	"bytes"
	"debug/buildinfo"
	"debug/elf"
	"encoding/json"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/forwarding"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
)

type forwardingReleasePlan struct {
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

type forwardingReleasePurpose uint8

const (
	forwardingReleaseInitial forwardingReleasePurpose = iota
	forwardingReleaseForward
)

func decodeForwardingReleasePlan(raw []byte, now time.Time) (forwardingReleasePlan, error) {
	return decodeForwardingReleasePurpose(raw, now, forwardingReleaseInitial)
}

func decodeForwardingReleasePurpose(raw []byte, now time.Time, purpose forwardingReleasePurpose) (forwardingReleasePlan, error) {
	var p forwardingReleasePlan
	schema := "ardents-forwarding-initial-release-plan-v1"
	if purpose == forwardingReleaseForward {
		schema = "ardents-forwarding-forward-release-plan-v1"
	} else if purpose != forwardingReleaseInitial {
		return p, errSourceControlInput
	}
	if len(raw) == 0 || len(raw) > 1<<20 || sourceDeclarationObject(raw, &p, "schema", "network", "reference_time", "expires", "program", "descriptor", "template", "program_custom", "generation_custom") != nil || p.Schema != schema || p.Network == "" || p.Reference.IsZero() || !p.Expires.After(now) || !p.Expires.After(p.Reference) || p.Expires.Sub(p.Reference) > 7*24*time.Hour || p.Reference.Before(now.Add(-5*time.Minute)) || p.Reference.After(now.Add(time.Minute)) {
		return p, errSourceControlInput
	}
	for _, path := range []string{p.Program, p.Descriptor, p.Template} {
		if !sourceControlPath(path) {
			return forwardingReleasePlan{}, errSourceControlInput
		}
	}
	return p, nil
}
func validateForwardingReleaseResources(plan forwardingReleasePlan, program, descriptor, template []byte) error {
	d, err := infrastructure.ValidateForwardingResources(descriptor, program, template)
	initial := plan.Schema == "ardents-forwarding-initial-release-plan-v1"
	forward := plan.Schema == "ardents-forwarding-forward-release-plan-v1"
	if err != nil || !initial && !forward || initial && d.ReleaseVersion != 1 || forward && d.ReleaseVersion <= 1 || !bytes.Equal(template, []byte(forwarding.Template)) {
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
