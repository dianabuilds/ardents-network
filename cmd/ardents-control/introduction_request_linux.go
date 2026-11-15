package main

import (
	"encoding/json"
	"flag"
	"io"

	introductioninstallation "github.com/dianabuilds/ardents-network/internal/successor/installation/introduction"
)

// This finite producer opens no declared credentials, clock or mutable root.
// Installation independently admits original native resources before effects.
func introductionRequestArguments(args []string) ([]byte, string, error) {
	r := introductioninstallation.Request{Schema: "ardents-introduction-installation-v1", Invocation: "introduction", Profile: "ardents-introduction-process-v1"}
	flags := flag.NewFlagSet("introduction prepare-request", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var output string
	for _, option := range []struct {
		name string
		path *string
	}{
		{"installation-root", &r.InstallationRoot}, {"state-root", &r.StateRoot}, {"participation-root", &r.ParticipationRoot},
		{"introduction-root", &r.IntroductionRoot}, {"spend-root", &r.SpendRoot}, {"hosting-root", &r.HostingRoot}, {"credential-directory", &r.CredentialDirectory},
		{"plan", &r.Plan}, {"certificate", &r.Certificate}, {"private-key", &r.PrivateKey}, {"clock", &r.Clock}, {"output", &output},
	} {
		flags.StringVar(option.path, option.name, "", "exact absolute path")
	}
	if flags.Parse(args) != nil || flags.NArg() != 0 || flags.NFlag() != 12 || !forwardingUniqueOptions(args, "") {
		return nil, "", errSourceControlInput
	}
	if err := r.CheckPreparationOutput(output); err != nil {
		return nil, "", err
	}
	raw, err := json.Marshal(r)
	if err != nil {
		return nil, "", err
	}
	return append(raw, '\n'), output, nil
}
