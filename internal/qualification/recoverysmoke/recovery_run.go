package recoverysmoke

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"path/filepath"
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
	var replacementCandidates []replacementCandidate
	hostScope, scopeErr := observer.observeDockerHostScope(ctx, fixture.manifest, imageID)
	if scopeErr != nil {
		return observer.invalid(scopeErr)
	}
	evidence.HostScope, scopeErr = json.Marshal(hostScope)
	if scopeErr != nil {
		return observer.invalid(scopeErr)
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
		replacementCandidates = append(replacementCandidates, replacementCandidate{Role: candidate.Role, Family: candidate.Family,
			Endpoint: candidate.Endpoint, NodeID: candidate.NodeID, PublicKey: candidate.PublicKey})
	}
	var err error
	evidence.BinaryDigests, err = observer.binaryIdentities(ctx)
	if err != nil {
		return observer.invalid(err)
	}
	observer.generation = filepath.Join(observer.input.FixtureRoot, "generations", "1")
	observer.evidenceFile = filepath.Join(observer.input.EvidenceRoot, "empty.json")
	observer.fixedWorkload = observer.input.Slice != "s4.1"
	if err := initializeRecoveryWorkload(observer.generation, observer.fixedWorkload); err != nil {
		return observer.invalid(err)
	}
	campaignStarted := time.Now()
	var replacementManifest json.RawMessage
	var replacementAttemptFiles []string
	if observer.input.Slice == "s4.2" {
		replacementManifest, err = prepareReplacementCampaignManifest(observer, fixture, hostScope, imageID, topology,
			replacementCandidates)
		if err != nil {
			return observer.invalid(err)
		}
	} else if observer.input.Slice == "s4.3" {
		replacementManifest, err = prepareStressCampaignManifest(observer, fixture, hostScope, imageID, topology,
			replacementCandidates)
		if err != nil {
			return observer.invalid(err)
		}
	}
	if observer.input.Slice == "s4.1" {
		for {
			for _, direction := range []string{"client-to-publisher", "publisher-to-client"} {
				observer.direction = direction
				baseline, err := observer.runNoFailureBaseline(ctx, direction)
				if err != nil {
					return observer.invalid(err)
				}
				cell, err := observer.runPositiveRecovery(ctx, direction, baseline, hostScope, hostClock)
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
	} else if observer.input.Slice == "s4.2" {
		for _, direction := range []string{"client-to-publisher", "publisher-to-client"} {
			for _, role := range replacementRoles {
				verificationPath, result := runReplacementCampaignCell(ctx, observer, fixture, direction,
					[]string{role}, false, hostScope, hostClock, replacementManifest, direction+" "+role)
				if result != nil {
					if result.Verdict == "fail" && verificationPath != "" {
						replacementAttemptFiles = append(replacementAttemptFiles, verificationPath)
						return observer.finishReplacementCampaign(ctx, imageID, replacementManifest,
							replacementAttemptFiles)
					}
					return *result
				}
				replacementAttemptFiles = append(replacementAttemptFiles, verificationPath)
			}
			verificationPath, result := runReplacementCampaignCell(ctx, observer, fixture, direction,
				[]string{"initiator", "rendezvous", "responder"}, true, hostScope, hostClock,
				replacementManifest, direction+" sequential")
			if result != nil {
				if result.Verdict == "fail" && verificationPath != "" {
					replacementAttemptFiles = append(replacementAttemptFiles, verificationPath)
					return observer.finishReplacementCampaign(ctx, imageID, replacementManifest,
						replacementAttemptFiles)
				}
				return *result
			}
			replacementAttemptFiles = append(replacementAttemptFiles, verificationPath)
		}
		return observer.finishReplacementCampaign(ctx, imageID, replacementManifest,
			replacementAttemptFiles)
	} else {
		verificationPath, result := runOverlapCampaignCell(ctx, observer, fixture, hostScope, hostClock,
			replacementManifest)
		if result != nil {
			if result.Verdict == "fail" && verificationPath != "" {
				replacementAttemptFiles = append(replacementAttemptFiles, verificationPath)
				return observer.finishReplacementCampaign(ctx, imageID, replacementManifest,
					replacementAttemptFiles)
			}
			return *result
		}
		replacementAttemptFiles = append(replacementAttemptFiles, verificationPath)
		for _, direction := range []string{"client-to-publisher", "publisher-to-client"} {
			verificationPath, result = runImpairedCampaignCell(ctx, observer, fixture, direction,
				hostScope, hostClock, replacementManifest)
			if result != nil {
				if result.Verdict == "fail" && verificationPath != "" {
					replacementAttemptFiles = append(replacementAttemptFiles, verificationPath)
					return observer.finishReplacementCampaign(ctx, imageID, replacementManifest,
						replacementAttemptFiles)
				}
				return *result
			}
			replacementAttemptFiles = append(replacementAttemptFiles, verificationPath)
		}
		return observer.finishReplacementCampaign(ctx, imageID, replacementManifest,
			replacementAttemptFiles)
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
	evidence.CampaignCompletedAtNanos = max(evidence.CampaignNanos,
		max(int64(1), time.Since(hostClock).Nanoseconds()))
	if err := observer.resetRecoveryTopology(ctx, time.Minute); err != nil {
		return observer.invalid(err)
	}
	cleanup, err := observer.observeDockerCleanup(ctx, hostScope, hostClock)
	if err != nil {
		return observer.invalid(err)
	}
	evidence.Cleanup.Adapter, evidence.Cleanup.Scope = cleanup.adapter, cleanup.scope
	evidence.Cleanup.ObservedAtNanos, evidence.Cleanup.OwnedResources = cleanup.observedAt, cleanup.owned
	evidence.Cleanup.AdapterProjection, evidence.Cleanup.Observation = cleanup.adapterProjection, cleanup.commitment
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
