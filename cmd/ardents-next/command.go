package main

import (
	"context"
	"fmt"
	"io"
	"os"
)

func run(ctx context.Context, arguments []string, out, diagnostic io.Writer) int {
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
		return runPlatform(ctx, args, out)
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
		return runExecution(ctx, args, out)
	case "release":
		return runRelease(ctx, args, out)
	case "enrollment":
		return runEnrollmentCommand(ctx, args, out, diagnostic)
	case "route":
		return runRoute(ctx, args, out, diagnostic)
	case "network":
		return runNetwork(ctx, args, out)
	case "node-identity":
		if len(args) > 0 && args[0] == "import" {
			return runIssuerProfile(ctx, "node-identity.import", args[1:], out, diagnostic)
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
		return runInstalledEndpoint(ctx, args, diagnostic)
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

func runSourceCommand(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "inspect-latest":
			return runSourceInspection(ctx, args[1:], out, diagnostic)
		case "prepare-trust":
			return runSourcePreparation(ctx, args, out)
		case "prepare-state":
			return runSourceStatePreparation(ctx, args[1:], out, diagnostic)
		case "repair-retained":
			return runRepair(ctx, "source", args[1:], out, diagnostic)
		}
	}
	return runSource(ctx, args, out, diagnostic)
}
