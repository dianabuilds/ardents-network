//go:build linux

package main

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/introduction"
)

// Fixed canonical consumers retain independent preflight and initial-install
// admission. Initial stopped completion grants no runtime duty.
func runIntroductionInstallation(parent context.Context, args []string, out io.Writer) error {
	if parent != nil && out != nil && len(args) == 2 && args[0] == "start-installed" {
		return runInstalledIntroduction(parent, args[1], out)
	}
	if parent != nil && out != nil && len(args) == 6 && (args[0] == "start-initial" || args[0] == "restart") {
		return startIntroductionGeneration(parent, args[1:], out, args[0] == "restart")
	}

	if parent != nil && out != nil && len(args) == 7 && args[0] == "install-initial-stopped" {
		return installInitialIntroduction(parent, args[1:], out)
	}
	if parent != nil && out != nil && len(args) == 6 && args[0] == "inspect-initial-history" {
		return inspectInitialIntroduction(parent, args[1:], out)
	}
	if parent != nil && out != nil && len(args) == 3 && args[0] == "stop" {
		return stopIntroduction(parent, args[1:], out)
	}
	if parent != nil && out != nil && len(args) == 3 && args[0] == "inspect-retirement" {
		return inspectIntroductionRetirement(parent, args[1:], out)
	}
	if parent != nil && out != nil && len(args) == 3 && args[0] == "inspect-start-history" {
		return inspectStartedIntroduction(parent, args[1:], out)
	}

	if parent == nil || out == nil || len(args) != 2 || args[0] != "inspect-inputs" {
		return errors.New("usage: ardents-node introduction inspect-inputs REQUEST | install-initial-stopped DISTRIBUTION PIN RELEASE-HISTORY ENVIRONMENT NETWORK REQUEST | start-installed BINDING | restart DISTRIBUTION RELEASE-HISTORY ENVIRONMENT NETWORK BINDING | start-initial DISTRIBUTION RELEASE-HISTORY ENVIRONMENT NETWORK BINDING | inspect-initial-history DISTRIBUTION RELEASE-HISTORY ENVIRONMENT NETWORK BINDING | inspect-start-history RELEASE-HISTORY BINDING | stop RELEASE-HISTORY BINDING | inspect-retirement RELEASE-HISTORY BINDING")
	}
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	input, err := introduction.ReadInputs(ctx, args[1])
	if err == nil {
		err = introduction.InspectInitialInputs(input)
	}
	if err = errors.Join(err, input.Close(), ctx.Err()); err != nil {
		return err
	}
	_, err = io.WriteString(out, "{\"outcome\":\"introduction-inputs-preflight-completed\",\"installed\":false}\n")
	return errors.Join(err, ctx.Err())
}
