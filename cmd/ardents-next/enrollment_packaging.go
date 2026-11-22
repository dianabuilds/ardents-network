package main

import (
	"context"
	"fmt"
	"io"

	"github.com/dianabuilds/ardents-network/internal/enrollment"
)

func runEnrollmentCommand(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	if len(args) == 0 {
		return runEnrollment(ctx, args, out)
	}
	var err error
	switch args[0] {
	case "artifact-name":
		if len(args) != 3 {
			fmt.Fprintln(diagnostic, "usage: ardents enrollment artifact-name <command> <platform>")
			return 2
		}
		_, err = fmt.Fprintln(out, enrollment.ExecutableArtifactName(args[1], args[2]))
	case "check-protected-generation":
		err = checkProtectedEnrollmentGeneration(args[1:])
	default:
		return runEnrollment(ctx, args, out)
	}
	if err != nil {
		fmt.Fprintln(diagnostic, err)
		return 2
	}
	return 0
}

func checkProtectedEnrollmentGeneration(args []string) error {
	if len(args) != 4 {
		return fmt.Errorf("usage: ardents enrollment check-protected-generation <static-root> <endpoint> <document-worker> <release-identity>")
	}
	return enrollment.CheckProtectedGenerationFiles(args[0], args[1], args[2], args[3])
}
