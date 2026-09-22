package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/contributor"
)

var errOldContributorStartRetired = errors.New("old Contributor start is retired")

type contributorRequest struct {
	action       contributor.Action
	confirmation string
	retiredStart bool
}

type contributorHostEnvironment struct {
	root       string
	supervisor contributor.Supervisor
}

// loadContributorHostEnvironment is the command's host-system boundary. The
// retirement oracle replaces it to prove retired starts never cross that
// boundary; maintained callers use the platform implementation below.
var loadContributorHostEnvironment = func() (contributorHostEnvironment, error) {
	supervisor, err := newSystemdSupervisor()
	return contributorHostEnvironment{root: contributorHostRoot(), supervisor: supervisor}, err
}

func runContributor(ctx context.Context, arguments []string, output io.Writer) error {
	request, err := parseContributorRequest(arguments)
	if err != nil {
		return err
	}
	if request.retiredStart {
		return errOldContributorStartRetired
	}
	host, err := loadContributorHostEnvironment()
	if err != nil {
		return err
	}
	profile, err := contributor.Open(contributor.Config{Root: host.root, Supervisor: host.supervisor})
	if err != nil {
		return err
	}
	report, err := profile.Control(ctx, request.action, request.confirmation)
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
		return contributorRequest{retiredStart: true}, nil
	case len(arguments) == 1 && arguments[0] == "diagnose":
		return contributorRequest{action: contributor.Diagnose}, nil
	case len(arguments) == 1 && arguments[0] == "restart":
		return contributorRequest{retiredStart: true}, nil
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
