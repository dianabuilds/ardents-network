package recoverysmoke

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
	"github.com/dianabuilds/ardents-network/internal/qualification/campaign"
	"github.com/dianabuilds/ardents-network/internal/qualification/recovery"
)

const replacementCampaignManifestSchema = "ardents-h3-s42-attempt-manifest-v1"

type replacementCampaignManifest struct {
	Schema, SourceCommit, ImageID, TopologyDigest string
	Topology                                      []byte
	HostScope                                     json.RawMessage
	RouteCase                                     json.RawMessage
	Candidates                                    []replacementCandidate
	RouteManifest                                 [32]byte
	Prerequisites                                 []qualificationPrerequisite
	Cells                                         []replacementCampaignCell
}

type replacementCampaignCell struct {
	CellID, Direction, Mode, ManifestDigest string
}

func prepareReplacementCampaignManifest(observer dockerObserver, fixture prepared, hostScope hostScopeEvidence,
	imageID string, topology []byte, candidates []replacementCandidate) (json.RawMessage, error) {
	seeds := make(map[string][32]byte, 2)
	for _, direction := range []string{"client-to-publisher", "publisher-to-client"} {
		seed, err := recoveryDirectionSeed(observer.generation, direction)
		if err != nil {
			return nil, err
		}
		seeds[direction] = seed
	}
	cells, err := replacementCampaignCells(seeds)
	if err != nil {
		return nil, err
	}
	scopeRaw, err := json.Marshal(hostScope)
	if err != nil {
		return nil, fmt.Errorf("encode replacement campaign HostScope: %w", err)
	}
	manifest := replacementCampaignManifest{Schema: replacementCampaignManifestSchema,
		SourceCommit: observer.sourceCommit, ImageID: imageID, HostScope: scopeRaw,
		Topology: append([]byte(nil), topology...), TopologyDigest: digestText(topology),
		RouteCase:  append(json.RawMessage(nil), fixture.routeCase...),
		Candidates: append([]replacementCandidate(nil), candidates...), RouteManifest: fixture.routeManifest,
		Prerequisites: append([]qualificationPrerequisite(nil), observer.input.Prerequisites...), Cells: cells}
	raw, err := json.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("encode replacement campaign manifest: %w", err)
	}
	if err := campaign.PublishManifest(observer.input.EvidenceRoot, raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func replacementCampaignCells(seeds map[string][32]byte) ([]replacementCampaignCell, error) {
	result := make([]replacementCampaignCell, 0, 10)
	for _, direction := range []string{"client-to-publisher", "publisher-to-client"} {
		prefix := map[string]string{"client-to-publisher": "c2p", "publisher-to-client": "p2c"}[direction]
		for _, role := range replacementRoles {
			offsets, lifetime, delay, mode := isolatedReplacementSchedule([]string{role})
			manifest, err := buildReplacementManifest(direction, mode, seeds[direction], []string{role}, offsets,
				lifetime, delay)
			if err != nil {
				return nil, err
			}
			result = append(result, replacementCampaignCell{CellID: prefix + "-" + mode,
				Direction: direction, Mode: mode, ManifestDigest: manifest.Digest})
		}
		offsets, lifetime, delay, mode := sequentialReplacementSchedule()
		failures := []string{"initiator", "rendezvous", "responder"}
		manifest, err := buildReplacementManifest(direction, mode, seeds[direction], failures, offsets, lifetime, delay)
		if err != nil {
			return nil, err
		}
		result = append(result, replacementCampaignCell{CellID: prefix + "-" + mode,
			Direction: direction, Mode: mode, ManifestDigest: manifest.Digest})
	}
	return result, nil
}

func verifyReplacementAttemptReceipt(ctx context.Context, observer dockerObserver, manifest json.RawMessage,
	receipt campaign.CellReceipt) (recovery.Result, string, error) {
	attemptRoot := filepath.Join(observer.input.EvidenceRoot, "cells", receipt.CellID, receipt.AttemptID)
	receiptPath := filepath.Join(attemptRoot, "receipt.json")
	receiptRaw, err := byteio.ReadFile(receiptPath, 5<<20)
	if err != nil {
		return recovery.Result{}, receiptPath, fmt.Errorf("read durable replacement attempt receipt: %w", err)
	}
	envelope := recovery.Evidence{Schema: "ardents-qualification-attempt-envelope-v1",
		AttemptManifest: manifest, AttemptReceipt: receiptRaw}
	path := filepath.Join(attemptRoot, "verification-input.json")
	if err := byteio.WriteJSON(path, envelope, 4<<20); err != nil {
		return recovery.Result{}, receiptPath, fmt.Errorf("write replacement attempt verification input: %w", err)
	}
	verifier := observer
	verifier.evidenceFile = path
	result, verifyErr := verifier.invokeRecoveryVerifier(ctx)
	if err := byteio.WriteJSON(filepath.Join(attemptRoot, "verifier.json"), result, 64<<10); err != nil {
		return result, receiptPath, fmt.Errorf("write replacement attempt verifier result: %w", err)
	}
	return result, receiptPath, verifyErr
}

func runReplacementCampaignCell(ctx context.Context, observer dockerObserver, fixture prepared, direction string,
	failures []string, sequential bool, hostScope hostScopeEvidence, hostClock time.Time,
	manifest json.RawMessage, label string) (string, *Result) {
	for retry := 0; retry < 3; retry++ {
		_, receipt, err := runReplacementAttempt(ctx, observer, fixture, direction, failures,
			sequential, hostScope, hostClock)
		if err != nil {
			result := observer.invalid(fmt.Errorf("%s receipt: %w", label, err))
			return "", &result
		}
		if receipt.Candidate == "not-run" && receipt.Observation == "invalid" && retry < 2 {
			continue
		}
		if receipt.Observation != "complete" || receipt.Cleanup != "complete" || receipt.Candidate == "not-run" {
			result := observer.invalid(errors.New(label + ": " + receipt.Reason))
			return "", &result
		}
		var verdict recovery.Result
		var path string
		var verifyErr error
		for verifyAttempt := 0; verifyAttempt < 3; verifyAttempt++ {
			verdict, path, verifyErr = verifyReplacementAttemptReceipt(ctx, observer, manifest, receipt)
			if verifyErr == nil {
				break
			}
		}
		if verifyErr != nil || verdict.Verdict != receipt.Candidate {
			if verifyErr == nil {
				verifyErr = errors.New(verdict.Reason)
			}
			result := observer.invalid(fmt.Errorf("verify %s receipt: %w", label, verifyErr))
			return "", &result
		}
		if receipt.Candidate == "fail" {
			return path, &Result{Verdict: "fail", Reason: label + ": " + receipt.Reason,
				EvidenceRoot: observer.input.EvidenceRoot, SourceCommit: observer.sourceCommit, ImageID: observer.imageID}
		}
		return path, nil
	}
	result := observer.invalid(errors.New(label + ": infrastructure retry bound was exhausted"))
	return "", &result
}

func (observer dockerObserver) finishReplacementCampaign(ctx context.Context, imageID string,
	manifest json.RawMessage, attemptFiles []string) Result {
	indexRaw, artifacts, err := buildReplacementCampaignIndex(observer.input.EvidenceRoot, manifest, attemptFiles)
	if err != nil {
		return observer.invalid(err)
	}
	indexPath := filepath.Join(observer.input.EvidenceRoot, "campaign-index.json")
	if err := byteio.WriteJSON(indexPath, json.RawMessage(indexRaw), 48<<20); err != nil {
		return observer.invalid(err)
	}
	cleanupErr := errors.Join(observer.resetRecoveryTopology(ctx, time.Minute), observer.assertDockerEmpty(ctx),
		removePrivateFixture(observer.input.FixtureRoot))
	if cleanupErr != nil {
		return observer.invalid(cleanupErr)
	}
	envelope := recovery.Evidence{Schema: "ardents-qualification-campaign-envelope-v1",
		AttemptManifest: manifest, AttemptCampaign: indexRaw}
	inputPath := filepath.Join(observer.input.EvidenceRoot, "campaign-verification-input.json")
	if err := byteio.WriteJSON(inputPath, envelope, 52<<20); err != nil {
		return observer.invalid(err)
	}
	verifier := observer
	verifier.evidenceFile = inputPath
	verdict, verifyErr := verifier.invokeRecoveryVerifier(ctx)
	verdictPath := filepath.Join(observer.input.EvidenceRoot, "campaign-verdict.json")
	writeErr := byteio.WriteJSON(verdictPath, verdict, 64<<10)
	if err := errors.Join(verifyErr, writeErr); err != nil {
		return observer.invalid(err)
	}
	if verdict.Verdict != "pass" && verdict.Verdict != "fail" {
		return observer.invalid(errors.New(verdict.Reason))
	}
	files := []string{filepath.Join(observer.input.EvidenceRoot, "campaign-manifest.json")}
	files = append(files, artifacts...)
	files = append(files, indexPath, inputPath, verdictPath)
	return Result{Verdict: verdict.Verdict, Reason: verdict.Reason, EvidenceRoot: observer.input.EvidenceRoot, Attempts: len(attemptFiles),
		SourceCommit: observer.sourceCommit, ImageID: imageID, attemptFiles: files,
		dockerProject: observer.project, imageTag: observer.image, DockerCleanup: true, FixtureCleanup: true}
}
