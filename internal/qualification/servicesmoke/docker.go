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
		negatives, negativeErr := observer.negativeReceipt(ctx)
		if negativeErr != nil {
			return Result{Verdict: "fail", Reason: negativeErr.Error(), EvidenceRoot: input.EvidenceRoot,
				Attempts: attempts, SourceCommit: commit, ImageID: imageID}
		}
		negatives["ungranted-sibling"] = ungranted
		evidence := newAttemptEvidence(fixture, commit, imageID, generations, negatives, shortcuts)
		observer.evidenceFile, err = writeAttempt(attemptRoot, evidence)
		if err != nil {
			return observer.invalid(err)
		}
		verified, verifyErr := observer.compose(ctx, time.Minute, "--profile", "verify", "run", "--no-deps", "--rm", "verifier")
		verifierJSON := jsonLine(verified, "verdict")
		if writeErr := os.WriteFile(filepath.Join(attemptRoot, "verifier.json"), verifierJSON, 0o600); writeErr != nil {
			return observer.invalid(writeErr)
		}
		var verdict struct{ Verdict string }
		if json.Unmarshal(verifierJSON, &verdict) != nil || verdict.Verdict != "pass" || verifyErr != nil {
			return Result{Verdict: "fail", Reason: "independent Stage 3 verifier did not pass", EvidenceRoot: input.EvidenceRoot,
				Attempts: attempts, SourceCommit: commit, ImageID: imageID}
		}
		if time.Since(started) >= input.Duration {
			break
		}
	}
	return Result{Verdict: "pass", Reason: fmt.Sprintf("local Docker H3 Stage 3 smoke passed %d migration attempts", attempts),
		EvidenceRoot: input.EvidenceRoot, Attempts: attempts, SourceCommit: commit, ImageID: imageID}
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

func newAttemptEvidence(fixture prepared, commit, image string, generations []generationEvidence,
	negatives, shortcuts map[string]bool) attemptEvidence {
	cleanup := map[string]bool{}
	for _, name := range []string{"containers", "network", "listeners", "sockets", "processes", "sessions", "publications"} {
		cleanup[name] = true
	}
	return attemptEvidence{Schema: "ardents-h3-service-evidence-v1", SourceCommit: commit, ImageID: image,
		ManifestDigest: hex32(fixture.manifest), NetworkID: fixture.network, AuthorityPublic: fixture.authority,
		Target: fixture.target, Generations: generations, Negatives: negatives, ShortcutsAbsent: shortcuts,
		Cleanup: cleanup, PrivateMaterialAbsent: true}
}
