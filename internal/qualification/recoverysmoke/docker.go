package recoverysmoke

import (
	"context"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func runDocker(ctx context.Context, input config, fixture prepared) (result Result) {
	commit := input.SourceCommit
	observer := dockerObserver{input: input, sourceCommit: commit,
		project: "ardents-recovery-" + commit[:8] + "-" + hex.EncodeToString(fixture.manifest[:4]),
		image:   "ardents-recovery-smoke:" + commit[:12], runtimeUser: runtimeUser()}
	defer func() {
		if result.DockerCleanup {
			return
		}
		observer.generation = filepath.Join(input.FixtureRoot, "generations", "1")
		observer.evidenceFile = filepath.Join(input.EvidenceRoot, "empty.json")
		cleanupErr := observer.resetRecoveryTopology(context.Background(), 2*time.Minute)
		if cleanupErr != nil {
			result.Verdict, result.Reason = "invalid", cleanupFailureReason(result.Reason, cleanupErr)
			return
		}
		result.DockerCleanup = true
	}()
	observer.generation = filepath.Join(input.FixtureRoot, "generations", "1")
	observer.evidenceFile = filepath.Join(input.EvidenceRoot, "empty.json")
	topology, err := observer.compose(ctx, time.Minute, "--profile", "*", "config")
	if err != nil {
		return observer.invalid(err)
	}
	if err := os.WriteFile(filepath.Join(input.EvidenceRoot, "topology.yaml"), topology, 0o600); err != nil {
		return observer.invalid(err)
	}
	if _, err := observer.compose(ctx, 10*time.Minute, "build", "publisher-endpoint"); err != nil {
		return observer.invalid(err)
	}
	rawImage, err := observer.docker(ctx, time.Minute, "image", "inspect", "--format", "{{.Id}}", observer.image)
	if err != nil || !strings.HasPrefix(strings.TrimSpace(string(rawImage)), "sha256:") {
		return observer.invalid(errors.New("built recovery image identity is invalid"))
	}
	observer.imageID = strings.TrimSpace(string(rawImage))
	rawRevision, err := observer.docker(ctx, time.Minute, "image", "inspect", "--format",
		"{{index .Config.Labels \"org.opencontainers.image.revision\"}}", observer.imageID)
	if err != nil || strings.TrimSpace(string(rawRevision)) != commit {
		return observer.invalid(errors.New("recovery image revision does not match source commit"))
	}
	return observer.runRecoveryCell(ctx, fixture, observer.imageID, topology)
}

func cleanupFailureReason(reason string, cleanupErr error) string {
	if reason == "" {
		return cleanupErr.Error()
	}
	return errors.Join(errors.New(reason), cleanupErr).Error()
}

func (observer dockerObserver) invalid(err error) Result {
	return Result{Verdict: "invalid", Reason: err.Error(), EvidenceRoot: observer.input.EvidenceRoot,
		SourceCommit: observer.sourceCommit, DockerCleanup: false}
}
