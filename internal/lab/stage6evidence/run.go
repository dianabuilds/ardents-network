package stage6evidence

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"runtime"
	"time"
)

const (
	profile        = "ardents-h3-stage-6-evidence-v1"
	campaignSchema = "ardents-stage-6-campaign-v1"
)

// Run launches one bounded process per A0-D6 cell, admits its handoff, and
// publishes a complete S6E1 campaign without verdict authority.
func Run(base, sourceCommit, dirtyDigest, workerExecutable string) error {
	if sourceCommit == "" || dirtyDigest == "" || workerExecutable == "" {
		return errors.New("S6E1 source identity is incomplete")
	}
	privateRoot, manifestRoot, evidenceRoot, err := prepareRoots(base)
	if err != nil {
		return err
	}
	var runID, admissionSecret [32]byte
	if _, err := rand.Read(runID[:]); err != nil {
		return err
	}
	if _, err := rand.Read(admissionSecret[:]); err != nil {
		return err
	}
	if err := os.WriteFile(privateRoot+string(os.PathSeparator)+"admission-secret.bin", admissionSecret[:], 0o600); err != nil {
		return err
	}
	launcherExecutable, err := os.Executable()
	if err != nil {
		return err
	}
	launcherDigest, err := executableDigest(launcherExecutable)
	if err != nil {
		return err
	}
	workerDigest, err := executableDigest(workerExecutable)
	if err != nil {
		return err
	}
	origin := time.Now()
	secretDigest := sha256.Sum256(admissionSecret[:])
	manifest := campaignManifest{Schema: campaignSchema, Profile: profile,
		RunID: hex.EncodeToString(runID[:]), SourceCommit: sourceCommit, DirtyDigest: dirtyDigest,
		LauncherSHA256: launcherDigest, WorkerSHA256: workerDigest,
		Platform:  runtime.GOOS + "/" + runtime.GOARCH,
		Toolchain: runtime.Version(), ClockOrigin: 0,
		AdmissionSecretHash: hex.EncodeToString(secretDigest[:]),
		Decisions:           []string{"R-041", "R-042-O1b", "R-043", "R-044-O2", "R-045-O1b", "R-046-O1", "R-047-O1", "R-055-S6E1", "R-057-O1"}}
	for ordinal, spec := range stage6Cells {
		cell := cellManifest{Schema: "ardents-stage-6-cell-manifest-v1", ID: spec.id, Ordinal: uint32(ordinal),
			Scenario: spec.scenario, ExpectedClass: spec.class, Predicate: spec.predicate,
			RequiredStreams: []string{"trace"}}
		artifact, writeErr := writeJSON(manifestRoot, cellPath(ordinal), cell.Schema, cell, false)
		if writeErr != nil {
			return writeErr
		}
		manifest.Cells = append(manifest.Cells, artifact)
	}
	campaignArtifact, err := writeJSON(manifestRoot, "campaign.json", campaignSchema, manifest, false)
	if err != nil {
		return err
	}
	index := evidenceIndex{Schema: "ardents-stage-6-evidence-index-v1", CampaignSHA256: campaignArtifact.SHA256}
	for ordinal, spec := range stage6Cells {
		start := time.Since(origin).Milliseconds()
		secret := [32]byte{}
		if spec.id == "C7" || spec.id == "D2" {
			secret = admissionSecret
		}
		input := workerInput{Schema: workerInputSchema, Cell: spec.id, Ordinal: uint32(ordinal),
			Scenario: spec.scenario, ExpectedClass: spec.class, Predicate: spec.predicate,
			RequiredStreams: []string{"trace"}, StartOffset: start, AdmissionSecret: secret}
		result, runErr := executeWorker(workerExecutable, input)
		if runErr != nil {
			return runErr
		}
		trace, class := result.Trace, result.Class
		trace.EndOffset = time.Since(origin).Milliseconds()
		prefix := evidenceCellPrefix(ordinal)
		stream, writeErr := writeJSON(evidenceRoot, prefix+"/observations/trace.jsonl", trace.Schema, trace, true)
		if writeErr != nil {
			return writeErr
		}
		terminal := terminalRecord{Schema: "ardents-stage-6-terminal-v1", Cell: spec.id, Ordinal: uint32(ordinal),
			Class: class, WorkerPID: result.WorkerPID, WorkerSHA: workerDigest,
			StartOffset: trace.StartOffset, EndOffset: trace.EndOffset}
		terminalArtifact, writeErr := writeJSON(evidenceRoot, prefix+"/terminal.json", terminal.Schema, terminal, false)
		if writeErr != nil {
			return writeErr
		}
		cleanup := cleanupRecord{Schema: "ardents-stage-6-cleanup-v1", Cell: spec.id, Ordinal: uint32(ordinal),
			Processes: []string{}, Listeners: []string{}, Temporary: []string{}}
		cleanupArtifact, writeErr := writeJSON(evidenceRoot, prefix+"/cleanup.json", cleanup.Schema, cleanup, false)
		if writeErr != nil {
			return writeErr
		}
		index.Cells = append(index.Cells, cellEvidence{ID: spec.id, Ordinal: uint32(ordinal),
			EpisodeOrdinal: 0, TerminalClass: class,
			Streams: []observationArtifact{{Path: stream.Path, Schema: stream.Schema, Role: "cell-worker",
				EpisodeOrdinal: 0, StreamOrdinal: 0, ObservationStart: trace.StartOffset,
				ObservationEnd: trace.EndOffset, Size: stream.Size, SHA256: stream.SHA256}},
			Terminal: terminalArtifact, Cleanup: cleanupArtifact})
	}
	_, err = writeJSON(evidenceRoot, "index.json", index.Schema, index, false)
	return err
}

func cellPath(ordinal int) string           { return "cells/" + twoDigits(ordinal) + ".json" }
func evidenceCellPrefix(ordinal int) string { return "cells/" + twoDigits(ordinal) }

func twoDigits(value int) string { return string([]byte{'0' + byte(value/10), '0' + byte(value%10)}) }
