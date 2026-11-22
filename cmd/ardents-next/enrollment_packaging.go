package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

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
		return fmt.Errorf("usage: ardents enrollment check-protected-generation <static-root> <endpoint> <text-worker> <release-identity>")
	}
	root := args[0]
	paths := map[string]string{"ardents-linux-amd64": args[1], "ardents-text-linux-amd64": args[2]}
	for _, name := range []string{"ardents-text-reader@.service", "ardents-text-publisher@.service", "ardents-text-reader.socket", "ardents-text-publisher.socket", "50-ardents-text.rules", "ardents-text.conf", "ardents-endpoint.service"} {
		paths[name] = filepath.Join(root, name)
	}
	files := make(map[string][]byte, len(paths))
	for name, path := range paths {
		contents, err := readProtectedEnrollmentFile(path, 64<<20)
		if err != nil {
			return err
		}
		files[name] = contents
	}
	raw, err := readProtectedEnrollmentFile(filepath.Join(root, "protected-endpoint.json"), 16<<10)
	if err != nil {
		return err
	}
	return enrollment.ValidateProtectedGeneration(raw, files, args[3])
}

func readProtectedEnrollmentFile(path string, maximum int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > maximum {
		return nil, fmt.Errorf("protected producer input is not a bounded direct regular file: %s", path)
	}
	return os.ReadFile(path)
}
