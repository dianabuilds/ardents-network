package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/dianabuilds/ardents-network/internal/diagnostics/runtimeevent"
	"github.com/dianabuilds/ardents-network/internal/domain/admission/operator"
	endpointcommand "github.com/dianabuilds/ardents-network/internal/domain/endpoint/command"
	nodereceive "github.com/dianabuilds/ardents-network/internal/domain/node/receive"
	platformlaunch "github.com/dianabuilds/ardents-network/internal/platform/launch"
)

func run(ctx context.Context, arguments []string, out, diagnostic io.Writer) (code int) {
	if len(arguments) >= 2 {
		candidate := ""
		switch arguments[0] {
		case "network", "enrollment", "release", "publication", "execution", "admission", "endpoint":
			candidate = arguments[0] + "." + arguments[1]
		case "read", "publish", "link":
			candidate = "application." + arguments[0]
		case "stream":
			candidate = "application.stream." + arguments[1]
		}
		if observation, valid := runtimeevent.ClassifyCommand(candidate, "command", "completed"); valid {
			defer func() {
				_ = runtimeevent.WriteCommand(diagnostic, observation.Operation, "command", runtimeevent.CommandExit(code))
			}()
		}
	}
	if len(arguments) == 0 {
		fmt.Fprintln(diagnostic, participantUsage)
		return 2
	}
	command, args := arguments[0], arguments[1:]
	switch command {
	case "help":
		return runHelp(args, out, diagnostic)
	case "--help":
		if len(args) == 0 {
			return runHelp(nil, out, diagnostic)
		}
	case "publish", "link", "read":
		return runDocumentCommand(ctx, arguments, diagnostic)
	case "stream":
		return runStreamCommand(ctx, args, diagnostic)
	case "endpoint":
		return runEndpointCommand(ctx, args, out, diagnostic)
	case "platform":
		return platformlaunch.Run(ctx, args, out)
	case "infrastructure":
		return runInfrastructureCommand(ctx, args, out, diagnostic)
	case "source":
		return runSourceCommand(ctx, args, out, diagnostic)
	case "node":
		return runNode(ctx, args, out, diagnostic)
	case "publication":
		return runPublication(ctx, args, out)
	case "diagnostics":
		return runDiagnostics(ctx, args, os.Stdin, out, diagnostic)
	case "execution":
		return runExecution(ctx, args, out, diagnostic)
	case "release":
		return runRelease(ctx, args, out)
	case "enrollment":
		return runEnrollmentCommand(ctx, args, out, diagnostic)
	case "route":
		return nodereceive.RunRouteReceive(ctx, args, out, diagnostic)
	case "network":
		return runNetwork(ctx, args, out)
	case "node-identity":
		if len(args) > 0 && args[0] == "import" {
			return operator.RunIssuerProfile(ctx, "node-identity.import", args[1:], out, diagnostic)
		}
	case "issuance":
		return runIssuance(ctx, args, out, diagnostic)
	case "admission":
		return runAdmission(ctx, args, out, diagnostic)
	case "hosting":
		return runHosting(ctx, args, out, diagnostic)
	}
	fmt.Fprintln(diagnostic, participantUsage)
	return 2
}

func runEndpointCommand(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	if len(args) == 0 || args[0] != "withdraw" {
		return endpointcommand.Run(ctx, args, diagnostic)
	}
	if len(args) != 2 {
		fmt.Fprintln(diagnostic, "usage: ardents endpoint withdraw <administration-socket>")
		return 2
	}
	if err := withdrawEndpoint(ctx, args[1], out); err != nil {
		fmt.Fprintln(diagnostic, err)
		return 1
	}
	return 0
}
