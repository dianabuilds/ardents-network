package blockedverify

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path/filepath"
	"sort"
	"time"
)

// Verify independently recomputes and writes one canonical pass, fail, or invalid verdict.
func Verify(config Config) (Result, error) {
	bundleRoot, rootErr := filepath.Abs(filepath.Dir(config.PublishableRoot))
	canonicalOutput := filepath.Join(bundleRoot, "verdict.json")
	canonicalPublishable := filepath.Join(bundleRoot, "publishable")
	canonicalSecret := filepath.Join(bundleRoot, "secret")
	if rootErr != nil || !samePath(config.OutputPath, canonicalOutput) ||
		!samePath(config.PublishableRoot, canonicalPublishable) || !samePath(config.SecretRoot, canonicalSecret) {
		return Result{}, errors.New("verifier output is not the canonical bundle verdict path")
	}
	publishableAliased, publishableAliasErr := pathHasSymlink(canonicalPublishable)
	secretAliased, secretAliasErr := pathHasSymlink(canonicalSecret)
	if publishableAliasErr != nil || secretAliasErr != nil || publishableAliased || secretAliased {
		return Result{}, errors.New("canonical publishable or secret root is unavailable or symlink-aliased")
	}
	var manifestValue manifest
	manifestRaw, manifestErr := decodeStrict(config.ManifestPath, &manifestValue)
	var evidenceValue evidence
	evidenceRaw, evidenceErr := decodeStrict(config.EvidencePath, &evidenceValue)
	var closureValue evidenceClosure
	closureRaw, closureErr := decodeStrict(config.ClosurePath, &closureValue)
	var canaries canaryCorpus
	canaryRaw, canaryErr := decodeStrict(config.CanaryPath, &canaries)
	verifierHash, executableErr := verifierExecutableHash()
	result := Result{Schema: "ardents-h3-blocked-entry-verdict-v1", Scope: manifestValue.CampaignKind,
		RunID:          manifestValue.RunID,
		ManifestSHA256: digest(manifestRaw), EvidenceSHA256: digest(evidenceRaw), VerifierSHA256: verifierHash,
		VerifiedUnixNano: time.Now().UnixNano()}
	if err := errors.Join(manifestErr, evidenceErr, closureErr, canaryErr, executableErr); err != nil {
		result.Verdict, result.Reasons = "invalid", []string{"one or more verifier inputs cannot be decoded or hashed"}
		return finish(config.OutputPath, result, nil)
	}
	foundationInvalid := verifyManifest(manifestValue, canaryRaw, verifierHash)
	if manifestValue.CampaignKind == "final-local" && manifestValue.FinalSpec != nil {
		foundationInvalid = append(foundationInvalid,
			verifyFinalRepositorySupply(config.WorkspaceRoot, *manifestValue.FinalSpec)...)
	}
	rootHash := sha256.Sum256([]byte(filepath.Clean(bundleRoot)))
	if manifestValue.EvidenceRootHash != hex.EncodeToString(rootHash[:]) {
		foundationInvalid = append(foundationInvalid, "bundle root does not match its immutable replay binding")
	}
	registryRoot, registryErr := filepath.Abs(config.RegistryRoot)
	registryHash := sha256.Sum256([]byte(filepath.Clean(registryRoot)))
	if registryErr != nil || manifestValue.RegistryRootHash != hex.EncodeToString(registryHash[:]) {
		foundationInvalid = append(foundationInvalid, "registry root does not match its immutable manifest binding")
	}
	if evidenceValue.Schema != "ardents-h3-blocked-entry-evidence-v1" ||
		evidenceValue.CampaignKind != manifestValue.CampaignKind || evidenceValue.Profile != manifestValue.Profile ||
		evidenceValue.RunID != manifestValue.RunID || evidenceValue.ManifestSHA256 != result.ManifestSHA256 ||
		!evidenceValue.CollectionClosed {
		foundationInvalid = append(foundationInvalid, "evidence identity or manifest binding is invalid")
	}
	if closureValue.Schema != "ardents-h3-blocked-entry-closure-v1" || closureValue.RunID != manifestValue.RunID ||
		closureValue.ManifestSHA256 != result.ManifestSHA256 || closureValue.EvidenceSHA256 != result.EvidenceSHA256 ||
		closureValue.ClosedUnixNano <= manifestValue.CreatedUnixNano || len(closureRaw) == 0 {
		foundationInvalid = append(foundationInvalid, "evidence closure is invalid")
	}
	foundationInvalid = append(foundationInvalid,
		verifyArtifacts(config.SecretRoot, manifestValue.SecretArtifacts, evidenceValue.SecretArtifacts)...)
	foundationInvalid = append(foundationInvalid, verifySupplyArtifacts(manifestValue)...)
	rawCanaries, encodedCanaries, canaryCommitments, canaryErr := validateCanaries(canaries)
	if canaryErr != nil {
		foundationInvalid = append(foundationInvalid, canaryErr.Error())
	}
	foundationInvalid = append(foundationInvalid,
		verifySupplementalArtifacts(config.PublishableRoot, manifestValue, evidenceValue, canaries)...)
	attributions, attributionInvalid := verifyAttributions(config.SecretRoot, evidenceValue.AttributionArtifacts,
		manifestValue)
	eventInvalid, failures, candidateCanaries := verifyEvents(evidenceValue.Events, canaryCommitments, attributions,
		encodedCanaries, manifestValue.CampaignKind == "final-local")
	operationalInvalid := append([]string(nil), eventInvalid...)
	var mutationInvalid []string
	operationalInvalid = append(operationalInvalid,
		verifyCandidateCanaryExercise(manifestValue.FixtureMode, canaries, candidateCanaries)...)
	operationalInvalid = append(operationalInvalid, attributionInvalid...)
	observerInvalid, observerFailures := verifyObservers(evidenceValue.Observers)
	operationalInvalid = append(operationalInvalid, observerInvalid...)
	failures = append(failures, observerFailures...)
	cleanupInvalid, cleanupFailures := verifyCleanup(evidenceValue.Cleanup, attributions)
	operationalInvalid, failures = append(operationalInvalid, cleanupInvalid...), append(failures, cleanupFailures...)
	if manifestValue.CampaignKind == "final-local" {
		components, invalidMutations := verifyFinalMutationCampaigns(config.SecretRoot, manifestValue.FinalSpec,
			evidenceValue.FinalSummary, canaries)
		result.Components = components
		mutationInvalid = invalidMutations
		finalSummaryValue, measurementInvalid := verifyMeasurementArtifacts(config.SecretRoot, evidenceValue.FinalSummary)
		operationalInvalid = append(operationalInvalid, measurementInvalid...)
		if finalSummaryValue == nil {
			finalSummaryValue = evidenceValue.FinalSummary
		}
		finalInvalid, finalFailures := verifyFinalCampaign(manifestValue.FinalSpec, finalSummaryValue)
		operationalInvalid = append(operationalInvalid, finalInvalid...)
		if finalSummaryValue != nil {
			operationalInvalid = append(operationalInvalid,
				verifyHostileCellBindings(evidenceValue.Events, finalSummaryValue.Cells)...)
		}
		failures = append(failures, finalFailures...)
		if finalSummaryValue != nil {
			operationalInvalid = append(operationalInvalid,
				verifyFinalObserverEvidence(config.SecretRoot, finalSummaryValue.Cells)...)
			operationalInvalid = append(operationalInvalid,
				verifyFinalTelemetryEvidence(config.SecretRoot, finalSummaryValue.Cells)...)
			operationalInvalid = append(operationalInvalid,
				verifyFinalTelemetryAggregates(config.SecretRoot, finalSummaryValue)...)
		}
	} else if evidenceValue.FinalSummary != nil {
		operationalInvalid = append(operationalInvalid, "development evidence contains an unsupported final summary")
	}
	if canaryErr == nil {
		canaryInvalid, canaryFailures := scanPublishable(config.PublishableRoot, config.EvidencePath,
			config.OutputPath, rawCanaries, encodedCanaries, candidateCanaries)
		operationalInvalid, failures = append(operationalInvalid, canaryInvalid...), append(failures, canaryFailures...)
	}
	if !sameDirectory(config.ManifestPath, config.EvidencePath, config.ClosurePath, config.PublishableRoot) {
		foundationInvalid = append(foundationInvalid, "manifest, evidence, and closure are outside the publishable root")
	}
	if expected, safe := safeArtifactPath(config.SecretRoot, "canaries.json"); !safe || !samePath(expected, config.CanaryPath) {
		foundationInvalid = append(foundationInvalid, "private canary corpus is outside the committed secret tree")
	}
	if len(result.Components) > 0 {
		result.Components[0] = finalCandidateComponent(foundationInvalid, failures, operationalInvalid)
	}
	operationalInvalid = append(operationalInvalid, mutationInvalid...)
	replayEligible := len(foundationInvalid) == 0
	switch {
	case len(foundationInvalid) > 0:
		result.Verdict, result.Reasons = "invalid", unique(foundationInvalid)
	case len(failures) > 0:
		result.Verdict, result.Reasons = "fail", unique(failures)
	case len(operationalInvalid) > 0:
		result.Verdict, result.Reasons = "invalid", unique(operationalInvalid)
	default:
		result.Verdict = "pass"
	}
	var transaction *replayTransaction
	if replayEligible {
		plannedHash, err := canonicalDecisionHash(result)
		if err != nil {
			return Result{}, err
		}
		var published bool
		var replayReason string
		transaction, published, replayReason = beginRun(config.RegistryRoot, manifestValue.RunID,
			manifestValue.ManifestNonceHash, result.ManifestSHA256, plannedHash, bundleRoot, config.OutputPath)
		if replayReason != "" {
			result.Verdict, result.Reasons = "invalid", []string{replayReason}
		} else if published {
			prior, err := recoverPublished(config.OutputPath, result.ManifestSHA256, manifestValue.RunID,
				transaction.decisionHash())
			if err != nil {
				transaction.abandon()
				return Result{}, err
			}
			if err := transaction.commit(); err != nil {
				return prior, err
			}
			return prior, nil
		}
	}
	finished, finishErr := finish(config.OutputPath, result, nil)
	if finishErr != nil {
		if transaction != nil {
			transaction.abandon()
		}
		return finished, finishErr
	}
	if transaction != nil {
		if err := transaction.commit(); err != nil {
			return finished, err
		}
	}
	return finished, nil
}

func finalCandidateComponent(foundation, failures, operational []string) string {
	switch {
	case len(foundation) > 0 || len(operational) > 0:
		return "candidate:invalid:564"
	case len(failures) > 0:
		return "candidate:fail:564"
	default:
		return "candidate:pass:564"
	}
}

func recoverPublished(path, manifestHash, runID, decisionHash string) (Result, error) {
	var result Result
	_, err := decodeStrict(path, &result)
	observedDecision, decisionErr := canonicalDecisionHash(result)
	if err != nil || decisionErr != nil || result.Schema != "ardents-h3-blocked-entry-verdict-v1" ||
		result.ManifestSHA256 != manifestHash || result.RunID != runID ||
		observedDecision != decisionHash ||
		(result.Verdict != "pass" && result.Verdict != "fail" && result.Verdict != "invalid") {
		return Result{}, errors.Join(err, errors.New("pending replay transaction has no recoverable canonical verdict"))
	}
	return result, nil
}

func canonicalDecisionHash(result Result) (string, error) {
	result.VerifiedUnixNano = 0
	result.Reasons = unique(result.Reasons)
	raw, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}
	return digest(append(raw, '\n')), nil
}

func samePath(left, right string) bool {
	left, leftErr := filepath.Abs(left)
	right, rightErr := filepath.Abs(right)
	return leftErr == nil && rightErr == nil && left == right
}

func sameDirectory(manifestPath, evidencePath, closurePath, root string) bool {
	root, rootErr := filepath.Abs(root)
	manifestDirectory, manifestErr := filepath.Abs(filepath.Dir(manifestPath))
	evidenceDirectory, evidenceErr := filepath.Abs(filepath.Dir(evidencePath))
	closureDirectory, closureErr := filepath.Abs(filepath.Dir(closurePath))
	return rootErr == nil && manifestErr == nil && evidenceErr == nil && closureErr == nil &&
		root == manifestDirectory && root == evidenceDirectory && root == closureDirectory
}

func unique(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}
