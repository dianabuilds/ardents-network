package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/contributor"
)

type contributorRequest struct {
	action       contributor.Action
	bundle, pin  string
	confirmation string
	apply        bool
}

func runContributor(ctx context.Context, arguments []string, output io.Writer) error {
	request, err := parseContributorRequest(arguments)
	if err != nil {
		return err
	}
	supervisor, err := newSystemdSupervisor()
	if err != nil {
		return err
	}
	profile, err := contributor.Open(contributor.Config{Root: contributorHostRoot(), Supervisor: supervisor})
	if err != nil {
		return err
	}
	var report contributor.Report
	if request.apply {
		report, err = profile.Apply(ctx, request.bundle, request.pin)
	} else {
		report, err = profile.Control(ctx, request.action, request.confirmation)
	}
	if err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(struct {
		Schema string `json:"schema"`
		contributor.Report
	}{Schema: "ardents-contributor-report-v1", Report: report})
}

func parseContributorRequest(arguments []string) (contributorRequest, error) {
	usage := errors.New("usage: ardents-node contributor (apply --bundle PATH --manifest-pin SHA256|diagnose|restart|drain|withdraw|remove --confirm DEPLOYMENT_ID)")
	switch {
	case len(arguments) == 5 && arguments[0] == "apply" && arguments[1] == "--bundle" && arguments[2] != "" && arguments[3] == "--manifest-pin" && arguments[4] != "":
		return contributorRequest{apply: true, bundle: arguments[2], pin: arguments[4]}, nil
	case len(arguments) == 1 && arguments[0] == "diagnose":
		return contributorRequest{action: contributor.Diagnose}, nil
	case len(arguments) == 1 && arguments[0] == "restart":
		return contributorRequest{action: contributor.Restart}, nil
	case len(arguments) == 1 && arguments[0] == "drain":
		return contributorRequest{action: contributor.Drain}, nil
	case len(arguments) == 1 && arguments[0] == "withdraw":
		return contributorRequest{action: contributor.Withdraw}, nil
	case len(arguments) == 3 && arguments[0] == "remove" && arguments[1] == "--confirm" && arguments[2] != "":
		return contributorRequest{action: contributor.Remove, confirmation: arguments[2]}, nil
	default:
		return contributorRequest{}, usage
	}
}
