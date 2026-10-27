package main

import (
	"encoding/json"
	"flag"
	"io"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/forwarding"
)

// These are unopened path declarations. Original input/native admission belongs
// to Installation, not this finite canonical request byte producer.
func forwardingRequestArguments(args []string) ([]byte, string, error) {
	r := forwarding.Request{Schema: "ardents-forwarding-installation-v1", Invocation: "forwarding", Profile: "ardents-forwarding-process-v1"}
	flags := flag.NewFlagSet("forwarding prepare-request", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var destination string
	for _, option := range []struct {
		name string
		path *string
	}{
		{"installation-root", &r.InstallationRoot}, {"state-root", &r.StateRoot}, {"participation-root", &r.ParticipationRoot},
		{"spend-root", &r.SpendRoot}, {"hosting-root", &r.HostingRoot}, {"credential-directory", &r.CredentialDirectory},
		{"plan", &r.Plan}, {"certificate", &r.Certificate}, {"private-key", &r.PrivateKey}, {"clock", &r.Clock}, {"output", &destination},
	} {
		flags.StringVar(option.path, option.name, "", "exact absolute path")
	}
	if flags.Parse(args) != nil || flags.NArg() != 0 || flags.NFlag() != 11 || !forwardingUniqueOptions(args, "") || !sourceControlPath(destination) {
		return nil, "", errSourceControlInput
	}
	for _, path := range []string{r.InstallationRoot, r.StateRoot, r.ParticipationRoot, r.SpendRoot, r.HostingRoot, r.CredentialDirectory, r.Plan, r.Certificate, r.PrivateKey, r.Clock} {
		if destination == path || strings.HasPrefix(destination, path+"/") || strings.HasPrefix(path, destination+"/") {
			return nil, "", errSourceControlInput
		}
	}
	raw, err := json.Marshal(r)
	if err != nil {
		return nil, "", err
	}
	raw = append(raw, '\n')
	if _, err := forwarding.DecodeRequest(raw); err != nil {
		return nil, "", err
	}
	return raw, destination, nil
}
