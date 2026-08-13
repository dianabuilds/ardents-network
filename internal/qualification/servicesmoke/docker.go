package servicesmoke

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
)

func runDocker(ctx context.Context, input Config, fixture prepared) (result Result) {
	commit, err := cleanCommit(ctx, input.SourceRoot)
	if err != nil {
		return Result{Verdict: "invalid", Reason: err.Error(), EvidenceRoot: input.EvidenceRoot, DockerCleanup: true}
	}
	observer := dockerObserver{input: input, sourceCommit: commit,
		project: "ardents-service-" + commit[:8] + "-" + hex.EncodeToString(fixture.manifest[:4]),
		image:   "ardents-service-smoke:" + commit[:12], runtimeUser: runtimeUser()}
	defer func() {
		observer.generation = filepath.Join(input.FixtureRoot, "generations", "1")
		observer.evidenceFile = filepath.Join(input.EvidenceRoot, "empty.json")
		_, cleanupErr := observer.compose(context.Background(), 2*time.Minute, "down", "-v", "--remove-orphans")
		if cleanupErr != nil {
			result.Verdict, result.Reason = "invalid", cleanupErr.Error()
			return
		}
		result.DockerCleanup = true
	}()
	observer.generation = filepath.Join(input.FixtureRoot, "generations", "1")
	observer.evidenceFile = filepath.Join(input.EvidenceRoot, "empty.json")
	topology, err := observer.compose(ctx, time.Minute, "config")
	if err != nil {
		return observer.invalid(err)
	}
	shortcuts, err := topologyReceipt(topology)
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
		return observer.invalid(errors.New("built image identity is invalid"))
	}
	imageID := strings.TrimSpace(string(rawImage))
	if err := byteio.WriteJSON(filepath.Join(input.EvidenceRoot, "preflight.json"), map[string]any{
		"schema": "ardents-h3-service-preflight-v1", "source_commit": commit, "image_id": imageID,
		"manifest_digest": hex32(fixture.manifest), "claim": "local development evidence only"}, 64<<10); err != nil {
		return observer.invalid(err)
	}
	started, attempts := time.Now(), 0
	attemptFiles := make([]string, 0, 4)
	for {
		attempts++
		attemptRoot := filepath.Join(input.EvidenceRoot, fmt.Sprintf("attempt-%06d", attempts))
		if err := os.Mkdir(attemptRoot, 0o700); err != nil {
			return observer.invalid(err)
		}
		generations := make([]generationEvidence, 0, 2)
		ungranted := true
		for generation := 1; generation <= 2; generation++ {
			observation, hostileRejected, runErr := observer.runGeneration(ctx, fixture, generation)
			if runErr != nil {
				return Result{Verdict: "fail", Reason: runErr.Error(), EvidenceRoot: input.EvidenceRoot,
					Attempts: attempts, SourceCommit: commit, ImageID: imageID}
			}
			generations = append(generations, observation)
			ungranted = ungranted && hostileRejected
		}
		negative, negativeErr := observer.negativeReceipt(ctx)
		if negativeErr != nil {
			return Result{Verdict: "fail", Reason: negativeErr.Error(), EvidenceRoot: input.EvidenceRoot,
				Attempts: attempts, SourceCommit: commit, ImageID: imageID}
		}
		negative.Negatives["ungranted-sibling"] = ungranted
		negative.Mechanisms["ungranted-sibling"] = "hostile-sibling-volume-boundary"
		evidence := newAttemptEvidence(fixture, commit, imageID, string(topology), generations,
			negative, shortcuts)
		observer.evidenceFile, err = writeAttempt(attemptRoot, evidence)
		if err != nil {
			return observer.invalid(err)
		}
		attemptFiles = append(attemptFiles, observer.evidenceFile)
		if time.Since(started) >= input.Duration {
			break
		}
	}
	return Result{Verdict: "pass", Reason: fmt.Sprintf("local Docker H3 Stage 3 smoke passed %d migration attempts", attempts),
		EvidenceRoot: input.EvidenceRoot, Attempts: attempts, SourceCommit: commit, ImageID: imageID,
		attemptFiles: attemptFiles, dockerProject: observer.project, imageTag: observer.image}
}

func verifyRetained(ctx context.Context, input Config, result *Result) error {
	if !result.DockerCleanup || !result.FixtureCleanup {
		return errors.New("independent verification requires completed Docker and private-fixture cleanup")
	}
	if _, err := os.Lstat(input.FixtureRoot); !errors.Is(err, os.ErrNotExist) {
		return errors.New("private fixture remains when independent verification begins")
	}
	observer := dockerObserver{input: input, sourceCommit: result.SourceCommit, project: result.dockerProject,
		image: result.imageTag, runtimeUser: runtimeUser(), generation: filepath.Join(input.FixtureRoot, "generations", "1")}
	defer func() {
		observer.evidenceFile = filepath.Join(input.EvidenceRoot, "empty.json")
		_, _ = observer.compose(context.Background(), 2*time.Minute, "down", "-v", "--remove-orphans")
	}()
	observed := cleanupObservation{Observed: true, Project: result.dockerProject, FixtureAbsent: true,
		Containers: []string{}, Networks: []string{}, Volumes: []string{}}
	for _, kind := range []string{"container", "network", "volume"} {
		raw, err := observer.docker(ctx, time.Minute, kind, "ls", "-q", "--filter",
			"label=com.docker.compose.project="+result.dockerProject)
		if err != nil || strings.TrimSpace(string(raw)) != "" {
			return errors.New("docker project resources remain before independent verification")
		}
	}
	for _, evidenceFile := range result.attemptFiles {
		raw, err := byteio.ReadFile(evidenceFile, 4<<20)
		if err != nil {
			return err
		}
		var evidence attemptEvidence
		if err := json.Unmarshal(raw, &evidence); err != nil {
			return err
		}
		for name := range evidence.Cleanup {
			evidence.Cleanup[name] = true
		}
		evidence.CleanupObservation = observed
		evidence.PrivateMaterialAbsent = true
		if _, err := writeAttempt(filepath.Dir(evidenceFile), evidence); err != nil {
			return err
		}
		observer.evidenceFile = evidenceFile
		verified, verifyErr := observer.compose(ctx, time.Minute, "--profile", "verify", "run", "--no-deps", "--rm", "verifier")
		verifierJSON := jsonLine(verified, "verdict")
		if writeErr := os.WriteFile(filepath.Join(filepath.Dir(evidenceFile), "verifier.json"), verifierJSON, 0o600); writeErr != nil {
			return writeErr
		}
		var verdict struct{ Verdict string }
		if json.Unmarshal(verifierJSON, &verdict) != nil || verdict.Verdict != "pass" || verifyErr != nil {
			return errors.New("independent Stage 3 verifier did not pass after cleanup")
		}
	}
	return nil
}

func jsonLine(raw []byte, required string) []byte {
	for _, line := range splitLines(raw) {
		var value map[string]any
		if json.Unmarshal(bytes.TrimSpace(line), &value) == nil {
			if _, ok := value[required]; ok {
				return append(bytes.TrimSpace(line), '\n')
			}
		}
	}
	return nil
}

func (observer dockerObserver) invalid(err error) Result {
	return Result{Verdict: "invalid", Reason: err.Error(), EvidenceRoot: observer.input.EvidenceRoot,
		SourceCommit: observer.sourceCommit, ImageID: observer.image}
}

func newAttemptEvidence(fixture prepared, commit, image, topology string, generations []generationEvidence,
	negative negativeReceipt, shortcuts map[string]bool) attemptEvidence {
	cleanup := map[string]bool{}
	for _, name := range []string{"containers", "network", "listeners", "sockets", "processes", "sessions", "publications"} {
		cleanup[name] = false
	}
	return attemptEvidence{Schema: "ardents-h3-service-evidence-v1", SourceCommit: commit, ImageID: image,
		ManifestDigest: hex32(fixture.manifest), NetworkID: fixture.network, AuthorityPublic: fixture.authority,
		IntroductionPublic: fixture.introduction, RouteManifestDigest: fixture.routeManifest,
		Target: fixture.target, Topology: topology, Generations: generations, Negatives: negative.Negatives,
		NegativeMechanisms: negative.Mechanisms, OperationObservations: negative.Operations,
		OperationClasses: negative.Classes, OperationCounts: negative.Counts, ShortcutsAbsent: shortcuts,
		Cleanup: cleanup, PrivateMaterialAbsent: false}
}
