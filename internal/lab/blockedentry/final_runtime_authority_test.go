package blockedentry

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFinalCampaignRejectsAmbientDockerAndHomeAuthority(t *testing.T) {
	t.Setenv("PATH", "ambient-path")
	t.Setenv("DOCKER_HOST", "tcp://ambient")
	t.Setenv("DOCKER_CONTEXT", "ambient-context")
	t.Setenv("DOCKER_CONFIG", "ambient-config")
	t.Setenv("HOME", "ambient-home")
	t.Setenv("USERPROFILE", "ambient-profile")
	config := Config{RunnerPath: "runner", CampaignSpecPath: "spec", EvidenceRoot: filepath.Join("root", "run")}
	command := campaignCommand(context.Background(), config)
	environment := strings.Join(command.Env, "\n")
	for _, forbidden := range []string{"ambient-path", "tcp://ambient", "ambient-context",
		"ambient-config", "ambient-home", "ambient-profile"} {
		if strings.Contains(environment, forbidden) {
			t.Fatalf("final campaign retained ambient authority %q", forbidden)
		}
	}
	if !strings.Contains(environment, "PATH=/usr/bin:/bin") ||
		!strings.Contains(environment, "DOCKER_HOST=unix:///var/run/docker.sock") ||
		!strings.Contains(environment, "DOCKER_CONFIG="+config.EvidenceRoot+".partial/secret/runtime/docker-config") {
		t.Fatalf("final campaign authority is not frozen: %s", environment)
	}
}

func TestFinalDockerConfigIsEmptyAndOwnerOnly(t *testing.T) {
	root := t.TempDir()
	if err := freezeFinalRuntimeAuthority(root); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, filepath.FromSlash(finalDockerConfigPath))
	raw, err := os.ReadFile(path)
	if err != nil || string(raw) != "{}\n" {
		t.Fatalf("docker config=%q err=%v", raw, err)
	}
}
