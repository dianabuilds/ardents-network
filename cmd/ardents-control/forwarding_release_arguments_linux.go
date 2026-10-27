package main

import (
	"encoding/json"
	"flag"
	"io"
	"time"
)

// Public declarations and existing artifact bytes only. No key, signature,
// initial pin or qualification fact is manufactured by this preparer.
func forwardingReleasePlanArguments(args []string, inputs *sourceControlInputs) ([]byte, string, error) {
	if inputs == nil {
		return nil, "", errSourceControlInput
	}
	set := flag.NewFlagSet("forwarding prepare-release-plan", flag.ContinueOnError)
	set.SetOutput(io.Discard)
	p := forwardingReleasePlan{Schema: "ardents-forwarding-initial-release-plan-v1"}
	var reference, expires, programCustom, generationCustom, destination string
	set.StringVar(&p.Network, "network", "", "approved software network declaration")
	set.StringVar(&reference, "reference", "", "current UTC RFC3339 time")
	set.StringVar(&expires, "expires", "", "UTC RFC3339 expiry")
	set.StringVar(&p.Program, "program", "", "exact canonical Node program")
	set.StringVar(&p.Descriptor, "descriptor", "", "exact Forwarding descriptor")
	set.StringVar(&p.Template, "template", "", "fixed Forwarding template")
	set.StringVar(&programCustom, "program-custom", "", "reviewed public program claims")
	set.StringVar(&generationCustom, "generation-custom", "", "reviewed public generation claims")
	set.StringVar(&destination, "output", "", "fresh output directory")
	if set.Parse(args) != nil || set.NArg() != 0 || set.NFlag() != 9 || !forwardingUniqueOptions(args, "") || !sourceControlPath(destination) {
		return nil, "", errSourceControlInput
	}
	var err error
	p.Reference, err = time.Parse(time.RFC3339, reference)
	if err != nil {
		return nil, "", err
	}
	p.Expires, err = time.Parse(time.RFC3339, expires)
	if err != nil {
		return nil, "", err
	}
	p.ProgramCustom, err = inputs.read(programCustom, 1<<20)
	if err != nil {
		return nil, "", err
	}
	p.GenerationCustom, err = inputs.read(generationCustom, 1<<20)
	if err != nil {
		return nil, "", err
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return nil, "", err
	}
	if _, err = decodeForwardingReleasePlan(raw, time.Now().UTC()); err != nil {
		return nil, "", err
	}
	var files [3][]byte
	for i, path := range []string{p.Program, p.Descriptor, p.Template} {
		maximum := int64(64 << 20)
		if i == 1 {
			maximum = 16 << 10
		}
		files[i], err = inputs.read(path, maximum)
		if err != nil {
			return nil, "", err
		}
	}
	if err = validateForwardingReleaseResources(p, files[0], files[1], files[2]); err != nil {
		return nil, "", err
	}
	return append(raw, '\n'), destination, nil
}
