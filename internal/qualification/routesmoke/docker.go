package routesmoke

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
)

type dockerObserver struct {
	input                        Config
	project, image, sourceDigest string
}

func runDocker(ctx context.Context, input Config, fixture prepared) (result Result) {
	digest, err := cleanSourceDigest(ctx, input.SourceRoot)
	if err != nil {
		return Result{Verdict: "invalid", Reason: err.Error(), EvidenceRoot: input.EvidenceRoot, DockerCleanup: true}
	}
	observer := dockerObserver{input: input, project: projectName(digest, fixture.manifest),
		image: "ardents-route-smoke:" + digest[:12], sourceDigest: digest}
	defer func() {
		if cleanupErr := observer.cleanup(context.Background()); cleanupErr != nil {
			result.Verdict, result.Reason = "invalid", cleanupErr.Error()
			result.EvidenceRoot = input.EvidenceRoot
			return
		}
		result.DockerCleanup = true
	}()
	if _, err := observer.compose(ctx, 5*time.Minute, "config", "--quiet"); err != nil {
		return observer.invalid(err)
	}
	if _, err := observer.compose(ctx, 10*time.Minute, "build", "publisher"); err != nil {
		return observer.invalid(err)
	}
	imageID, err := observer.imageReceipt(ctx)
	if err != nil {
		return observer.invalid(err)
	}
	if err := byteio.WriteJSON(filepath.Join(input.EvidenceRoot, "preflight.json"), map[string]any{
		"schema": "ardents-h3-route-smoke-preflight-v1", "source_digest": digest, "image_id": imageID,
		"compose_file": input.ComposeFile, "claim": "local development evidence only"}, 64<<10); err != nil {
		return observer.invalid(err)
	}
	started, attempts := time.Now(), 0
	for {
		attempts++
		_, err := observer.attempt(ctx, fixture, attempts)
		if err != nil {
			return Result{Verdict: verdict(err), Reason: err.Error(), EvidenceRoot: input.EvidenceRoot,
				Attempts: attempts, SourceDigest: digest, ImageID: imageID}
		}
		if time.Since(started) >= input.Duration {
			break
		}
	}
	return Result{Verdict: "pass", Reason: fmt.Sprintf("local Docker H3 Stage 2 smoke passed %d attempts", attempts),
		EvidenceRoot: input.EvidenceRoot, Attempts: attempts, SourceDigest: digest, ImageID: imageID}
}

func (observer dockerObserver) attempt(ctx context.Context, fixture prepared, number int) ([32]byte, error) {
	var zero [32]byte
	observer.down(ctx)
	roles := []string{"publisher", "responder", "rendezvous", "introduction", "initiator"}
	args := append([]string{"up", "-d", "--force-recreate"}, roles...)
	if _, err := observer.compose(ctx, time.Minute, args...); err != nil {
		return zero, err
	}
	if err := observer.waitReady(ctx, roles); err != nil {
		return zero, err
	}
	roleIDs := make(map[string]string, len(roles))
	for _, role := range roles {
		containerID, serviceErr := observer.serviceID(ctx, role)
		if serviceErr != nil {
			return zero, serviceErr
		}
		roleIDs[role] = containerID
	}
	clientIDRaw, err := observer.compose(ctx, time.Minute, "run", "-d", "--no-deps", "client")
	if err != nil {
		return zero, err
	}
	clientID := strings.TrimSpace(string(clientIDRaw))
	if !validContainerID(clientID) {
		return zero, errors.New("client container identity is invalid")
	}
	if err := observer.waitContainer(ctx, clientID); err != nil {
		return zero, err
	}
	clientRaw, err := observer.docker(ctx, time.Minute, "logs", clientID)
	if err != nil {
		return zero, err
	}
	for _, role := range roles {
		if err := observer.waitContainer(ctx, roleIDs[role]); err != nil {
			return zero, err
		}
	}
	values := make([][]byte, 0, 6)
	client, clientLine, err := terminalEvidence(clientRaw)
	if err != nil {
		return zero, err
	}
	values = append(values, clientLine)
	all := []evidenceIdentity{{client.PID, client.RuntimeID, clientID}}
	for _, role := range []string{"initiator", "introduction", "rendezvous", "responder", "publisher"} {
		raw, logErr := observer.compose(ctx, time.Minute, "logs", "--no-color", "--no-log-prefix", role)
		if logErr != nil {
			return zero, logErr
		}
		value, line, decodeErr := terminalEvidence(raw)
		if decodeErr != nil {
			return zero, decodeErr
		}
		values, all = append(values, line), append(all, evidenceIdentity{value.PID, value.RuntimeID, roleIDs[role]})
	}
	observer.down(ctx)
	remaining, err := observer.compose(ctx, time.Minute, "ps", "-q")
	if err != nil || len(bytes.TrimSpace(remaining)) != 0 {
		return zero, errors.New("route smoke cleanup left candidate containers")
	}
	attemptRoot := filepath.Join(observer.input.EvidenceRoot, fmt.Sprintf("attempt-%06d", number))
	if err := os.Mkdir(attemptRoot, 0o700); err != nil {
		return zero, err
	}
	caseInput := fixture.base
	caseInput.RawEvidence = joinEvidence(values)
	caseInput.EvidenceDigest = sha256.Sum256(caseInput.RawEvidence)
	caseInput.ManifestDigest = fixture.manifest
	caseInput.SourceID, caseInput.BuildDigest = client.SourceID, client.BuildDigest
	caseInput.CleanupVerified = true
	for index, identity := range all {
		caseInput.ExitedPIDs[index], caseInput.ExitedRuntimeIDs[index] = identity.pid, identity.runtimeID
		caseInput.ContainerIDs[index] = identity.containerID
	}
	if err := writeAttempt(attemptRoot, caseInput); err != nil {
		return zero, err
	}
	manifestPath := "/run/ardents/evidence/" + filepath.Base(attemptRoot) + "/manifest.json"
	verified, verifyErr := observer.compose(ctx, time.Minute, "--profile", "verify", "run", "--no-deps", "--rm", "verifier",
		"/usr/local/bin/ardents-route-qualify", manifestPath)
	verdict, err := acceptVerifier(attemptRoot, verified)
	if err != nil {
		return zero, err
	}
	switch verdict {
	case "pass":
		if verifyErr != nil {
			return zero, fmt.Errorf("independent Route verifier pass exited unsuccessfully: %w", verifyErr)
		}
	case "fail":
		return zero, candidateFailure(errors.New("independent Route verifier returned fail"))
	default:
		return zero, fmt.Errorf("independent Route verifier returned invalid: %w", verifyErr)
	}
	observer.down(ctx)
	return caseInput.EvidenceDigest, nil
}

func (observer dockerObserver) waitReady(ctx context.Context, roles []string) error {
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		ready := 0
		for _, role := range roles {
			raw, _ := observer.compose(ctx, 15*time.Second, "logs", "--no-color", "--no-log-prefix", role)
			if bytes.Contains(raw, []byte(`"kind":"ready"`)) {
				ready++
			}
		}
		if ready == len(roles) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return errors.New("route roles did not become ready within 20s")
}

func (observer dockerObserver) invalid(err error) Result {
	return Result{Verdict: "invalid", Reason: err.Error(), EvidenceRoot: observer.input.EvidenceRoot,
		SourceDigest: observer.sourceDigest}
}
func candidateFailure(err error) error { return fmt.Errorf("candidate failure: %w", err) }
func verdict(err error) string {
	if strings.Contains(err.Error(), "candidate failure") {
		return "fail"
	}
	return "invalid"
}

type evidenceIdentity struct {
	pid                    int
	runtimeID, containerID string
}

func projectName(source string, manifest [32]byte) string {
	return "ardents-route-" + source[:8] + "-" + hex.EncodeToString(manifest[:4])
}
