package recoverysmoke

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
	"github.com/dianabuilds/ardents-network/internal/qualification/recovery"
)

func (observer dockerObserver) runRecoveryCell(ctx context.Context, fixture prepared,
	imageID string, topology []byte) (result Result) {
	hostClock := time.Now()
	path := filepath.Join(observer.input.EvidenceRoot, "recovery-evidence.json")
	claim := "S4.1 local development evidence only"
	if observer.input.Slice == "s4.2" {
		claim = "S4.2 four-position local development tracer only; does not qualify split-leg/Introduction topology"
	}
	evidence := recovery.Evidence{Schema: "ardents-h3-recovery-evidence-v1", SourceCommit: observer.sourceCommit,
		ImageID: imageID, TopologyDigest: digestText(topology), ManifestDigest: hex32(fixture.manifest),
		VerifierImageID: imageID, Claim: claim, Negatives: make(map[string]recovery.Negative),
		Target: fixture.target, Instance: fixture.credentials[0].InstancePublic, NetworkID: fixture.network,
		CandidateView: fixture.routeManifest, AuthorityPublic: fixture.authority,
		ClientPrincipal: fixture.bindings[0][0].Principal, PublisherPrincipal: fixture.bindings[0][1].Principal,
		RouteProfile: "h3-route-tracer-v1", CredentialGeneration: fixture.credentials[0].Generation,
		CredentialNotBefore: fixture.credentials[0].NotBefore, CredentialNotAfter: fixture.credentials[0].NotAfter,
		WorkSafetyNotAfter: fixture.credentials[0].NotAfter, WorkSafetyMaximum: fixture.credentials[0].NotAfter,
		NoNewRecoveryAfter: fixture.credentials[0].NotAfter}
	evidence.Topology = append([]byte(nil), topology...)
	extension := replacementEvidence{RouteCase: append(json.RawMessage(nil), fixture.routeCase...)}
	if observer.input.Slice == "s4.2" {
		var scopeErr error
		extension.HostScope, scopeErr = observer.observeDockerHostScope(ctx, fixture.manifest, imageID)
		if scopeErr != nil {
			return observer.invalid(scopeErr)
		}
	}
	evidence.Manifest = recovery.PublicManifest{RouteManifest: fixture.routeManifest, NetworkID: fixture.network,
		AuthorityPublic: fixture.authority, IntroductionPublic: fixture.introduction, Target: fixture.target,
		InstancePublic: fixture.credentials[0].InstancePublic, ClientPrincipal: fixture.bindings[0][0].Principal,
		PublisherPrincipal: fixture.bindings[0][1].Principal, CredentialSignature: fixture.credentials[0].Signature,
		CredentialGeneration: fixture.credentials[0].Generation, CredentialNotBefore: fixture.credentials[0].NotBefore,
		CredentialNotAfter: fixture.credentials[0].NotAfter, CredentialCapabilities: fixture.credentials[0].Capabilities,
		RouteProfile: "h3-route-tracer-v1", WorkSafetyNotAfter: fixture.credentials[0].NotAfter,
		WorkSafetyMaximum: fixture.credentials[0].NotAfter, NoNewRecoveryAfter: fixture.credentials[0].NotAfter}
	evidence.IsolationContext = sha256.Sum256(append([]byte("isolation\x00"), fixture.manifest[:]...))
	evidence.DestinationBinding = sha256.Sum256(append([]byte("destination\x00"), fixture.target[:]...))
	for _, candidate := range fixture.candidates {
		extension.Candidates = append(extension.Candidates, replacementCandidate{Role: candidate.Role, Family: candidate.Family,
			Endpoint: candidate.Endpoint, NodeID: candidate.NodeID, PublicKey: candidate.PublicKey})
	}
	var err error
	evidence.BinaryDigests, err = observer.binaryIdentities(ctx)
	if err != nil {
		return observer.invalid(err)
	}
	observer.generation = filepath.Join(observer.input.FixtureRoot, "generations", "1")
	observer.evidenceFile = filepath.Join(observer.input.EvidenceRoot, "empty.json")
	campaignStarted := time.Now()
	if observer.input.Slice == "s4.1" {
		for {
			for _, direction := range []string{"client-to-publisher", "publisher-to-client"} {
				observer.direction = direction
				baseline, err := observer.runNoFailureBaseline(ctx, direction)
				if err != nil {
					return observer.invalid(err)
				}
				cell, err := observer.runPositiveRecovery(ctx, direction, baseline)
				if err != nil {
					return Result{Verdict: "fail", Reason: direction + ": " + err.Error(), EvidenceRoot: observer.input.EvidenceRoot,
						SourceCommit: observer.sourceCommit, ImageID: imageID}
				}
				evidence.Cells = append(evidence.Cells, cell)
			}
			if time.Since(campaignStarted) >= observer.input.Duration {
				break
			}
		}
	} else {
		for _, direction := range []string{"client-to-publisher", "publisher-to-client"} {
			baseline, err := observer.runNoFailureBaseline(ctx, direction)
			if err != nil {
				return observer.invalid(err)
			}
			cell, err := observer.runPositiveRecovery(ctx, direction, baseline)
			if err != nil {
				return Result{Verdict: "fail", Reason: direction + ": " + err.Error(), EvidenceRoot: observer.input.EvidenceRoot,
					SourceCommit: observer.sourceCommit, ImageID: imageID}
			}
			evidence.Cells = append(evidence.Cells, cell)
			for _, role := range replacementRoles {
				baseline, err = observer.runNoFailureBaseline(ctx, direction)
				if err != nil {
					return observer.invalid(err)
				}
				replacement, replacementErr := observer.runReplacementRecovery(ctx, fixture, direction,
					[]string{role}, baseline, false, extension.HostScope, hostClock)
				if replacementErr != nil {
					return Result{Verdict: "fail", Reason: direction + " " + role + ": " + replacementErr.Error(),
						EvidenceRoot: observer.input.EvidenceRoot, SourceCommit: observer.sourceCommit, ImageID: imageID}
				}
				extension.Cells = append(extension.Cells, replacement)
			}
			baseline, err = observer.runNoFailureBaseline(ctx, direction)
			if err != nil {
				return observer.invalid(err)
			}
			sequential, sequentialErr := observer.runReplacementRecovery(ctx, fixture, direction,
				[]string{"initiator", "rendezvous", "responder"}, baseline, true, extension.HostScope, hostClock)
			if sequentialErr != nil {
				return Result{Verdict: "fail", Reason: direction + " sequential: " + sequentialErr.Error(),
					EvidenceRoot: observer.input.EvidenceRoot, SourceCommit: observer.sourceCommit, ImageID: imageID}
			}
			extension.Cells = append(extension.Cells, sequential)
		}
		raw, marshalErr := json.Marshal(extension)
		if marshalErr != nil {
			return observer.invalid(marshalErr)
		}
		evidence.S42 = raw
	}
	evidence.RequestedNanos = observer.input.Duration.Nanoseconds()
	if observer.input.Slice == "s4.2" {
		evidence.RequestedNanos = max(evidence.RequestedNanos, int64(20*time.Minute))
	}
	evidence.CampaignNanos = time.Since(campaignStarted).Nanoseconds()
	negatives, err := observer.recoveryNegatives(ctx)
	if err != nil {
		return observer.invalid(err)
	}
	evidence.Negatives = negatives
	if err := observer.resetRecoveryTopology(ctx, time.Minute); err != nil {
		return observer.invalid(err)
	}
	if err := observer.assertDockerEmpty(ctx); err != nil {
		return observer.invalid(err)
	}
	evidence.Cleanup.DockerEmpty = true
	if err := removePrivateFixture(observer.input.FixtureRoot); err != nil {
		return observer.invalid(err)
	}
	evidence.Cleanup.FixtureAbsent, evidence.Cleanup.PrivateMaterialAbsent = true, true
	if err := byteio.WriteJSON(path, evidence, 4<<20); err != nil {
		return observer.invalid(err)
	}
	observer.evidenceFile = path
	verdict, err := observer.invokeRecoveryVerifier(ctx)
	if writeErr := byteio.WriteJSON(filepath.Join(observer.input.EvidenceRoot, "verifier.json"), verdict, 64<<10); writeErr != nil {
		return observer.invalid(writeErr)
	}
	if err != nil || verdict.Verdict != "pass" {
		if err == nil {
			err = errors.New(verdict.Reason)
		}
		return observer.invalid(err)
	}
	if err := observer.resetRecoveryTopology(ctx, time.Minute); err != nil {
		return observer.invalid(err)
	}
	if err := observer.assertDockerEmpty(ctx); err != nil {
		return observer.invalid(err)
	}
	return Result{Verdict: "pass", Reason: verdict.Reason, EvidenceRoot: observer.input.EvidenceRoot, Attempts: len(evidence.Cells),
		SourceCommit: observer.sourceCommit, ImageID: imageID, attemptFiles: []string{path}, dockerProject: observer.project,
		imageTag: observer.image, DockerCleanup: true, FixtureCleanup: true}
}

func (observer dockerObserver) invokeRecoveryVerifier(ctx context.Context) (recovery.Result, error) {
	if err := os.Chmod(observer.evidenceFile, 0o444); err != nil {
		return recovery.Result{}, err
	}
	defer os.Chmod(observer.evidenceFile, 0o600)
	raw, err := observer.docker(ctx, time.Minute, "run", "--rm", "--network", "none", "--read-only",
		"--cap-drop", "ALL", "--security-opt", "no-new-privileges", "--user", "65532:65532",
		"--mount", "type=bind,src="+observer.evidenceFile+",dst=/run/ardents/evidence.json,readonly",
		resultImage(observer), "/usr/local/bin/ardents-recovery-qualify", "/run/ardents/evidence.json")
	if err != nil {
		return recovery.Result{}, err
	}
	var result recovery.Result
	for _, line := range splitLines(raw) {
		if json.Unmarshal(line, &result) == nil && result.Verdict != "" {
			return result, nil
		}
	}
	return result, errors.New("independent recovery verifier verdict is missing")
}

func resultImage(observer dockerObserver) string {
	if observer.imageID != "" {
		return observer.imageID
	}
	return observer.image
}

func (observer dockerObserver) assertDockerEmpty(ctx context.Context) error {
	for _, kind := range []string{"container", "network", "volume"} {
		raw, err := observer.docker(ctx, time.Minute, kind, "ls", "-q", "--filter",
			"label=com.docker.compose.project="+observer.project)
		if err != nil || strings.TrimSpace(string(raw)) != "" {
			return errors.New("recovery Docker ownership is not empty")
		}
	}
	return nil
}

func digestText(raw []byte) string {
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func recoveryReceiptPath(root string) string { return filepath.Join(root, "recovery-negative.json") }

func writeRecoveryReceipt(root string, value any) error {
	return byteio.WriteJSON(recoveryReceiptPath(root), value, 1<<20)
}
