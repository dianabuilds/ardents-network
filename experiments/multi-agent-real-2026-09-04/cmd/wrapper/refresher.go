//go:build ignore

package main

import (
	"errors"
	"os/exec"
)

type refreshRunner interface {
	run(arguments []string) ([]byte, int, error)
}

type dockerRunner struct{}

func (dockerRunner) run(arguments []string) ([]byte, int, error) {
	command := exec.Command("docker", arguments...)
	output, err := command.CombinedOutput()
	if err == nil {
		return output, 0, nil
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		return output, exitError.ExitCode(), err
	}
	return output, -1, err
}

func refreshArguments(manifest runManifest, persona personaConfig) []string {
	return []string{"exec", manifest.Container, "/workspace/artifacts/ardents-linux-amd64", "refresh-sources", "--once",
		"--state-root", persona.StateRoot, "--source-plan", persona.SourcePlan}
}
