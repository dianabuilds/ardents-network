package stage6verify

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

const verdictSchema = "ardents-stage-6-verdict-v1"

// Verify independently admits one complete S6E1 bundle, recomputes its
// predicates, writes a verifier-only verdict, and returns the same value.
func (Stage6Verifier) Verify(manifestRoot, evidenceRoot, privateRoot, verdictRoot string) Verdict {
	verdict := Verdict{Schema: verdictSchema, Status: "invalid", Diagnostics: []string{}}
	verdict.VerifierSHA256, _ = executableDigest()
	if err := distinctRoots(manifestRoot, evidenceRoot, privateRoot, verdictRoot); err != nil {
		verdict.Diagnostics = []string{"root-separation"}
		return verdict
	}
	campaignRaw, cells, campaign, err := admitManifest(manifestRoot)
	if err != nil {
		verdict.Diagnostics = []string{"manifest-invalid"}
		return writeVerdict(verdictRoot, verdict)
	}
	campaignDigest := sha256.Sum256(campaignRaw)
	verdict.CampaignSHA256 = hex.EncodeToString(campaignDigest[:])
	secret, err := readSecret(privateRoot, campaign.AdmissionSecretHash)
	if err != nil {
		verdict.Diagnostics = []string{"private-input-invalid"}
		return writeVerdict(verdictRoot, verdict)
	}
	indexRaw, failures, err := admitEvidence(evidenceRoot, verdict.CampaignSHA256, cells, secret, campaign.WorkerSHA256)
	if err != nil {
		verdict.Diagnostics = []string{"evidence-invalid"}
		return writeVerdict(verdictRoot, verdict)
	}
	indexDigest := sha256.Sum256(indexRaw)
	verdict.EvidenceSHA256 = hex.EncodeToString(indexDigest[:])
	if len(failures) > 0 {
		verdict.Status, verdict.Diagnostics = "fail", failures
	} else {
		verdict.Status = "pass"
	}
	return writeVerdict(verdictRoot, verdict)
}

func admitManifest(root string) ([]byte, []cellManifest, campaignManifest, error) {
	if err := exactRootInventory(root, manifestInventory()); err != nil {
		return nil, nil, campaignManifest{}, err
	}
	var campaign campaignManifest
	raw, err := readCanonical(root, "campaign.json", maximumIndexBytes, &campaign, false)
	if err != nil || campaign.Schema != "ardents-stage-6-campaign-v1" ||
		campaign.Profile != "ardents-h3-stage-6-evidence-v1" || !validHex(campaign.RunID, 32) ||
		campaign.SourceCommit == "" || campaign.DirtyDigest == "" || !validHex(campaign.LauncherSHA256, 32) ||
		!validHex(campaign.WorkerSHA256, 32) ||
		campaign.Platform == "" || campaign.Toolchain == "" || campaign.ClockOrigin != 0 ||
		!validHex(campaign.AdmissionSecretHash, 32) || len(campaign.Cells) != len(expectedCells) ||
		!equalStrings(campaign.Decisions, expectedDecisions) {
		return nil, nil, campaign, errors.New("campaign manifest is invalid")
	}
	cells := make([]cellManifest, len(expectedCells))
	for ordinal, expected := range expectedCells {
		path := "cells/" + twoDigits(ordinal) + ".json"
		if _, err := verifyArtifact(root, campaign.Cells[ordinal], path, "ardents-stage-6-cell-manifest-v1",
			maximumIndexBytes, &cells[ordinal], false); err != nil {
			return nil, nil, campaign, err
		}
		cell := cells[ordinal]
		if cell.Schema != "ardents-stage-6-cell-manifest-v1" || cell.ID != expected.id ||
			cell.Ordinal != uint32(ordinal) || cell.Scenario != expected.scenario ||
			cell.ExpectedClass != expected.class || cell.Predicate != expected.predicate ||
			len(cell.RequiredStreams) != 1 || cell.RequiredStreams[0] != "trace" {
			return nil, nil, campaign, errors.New("cell manifest inventory is invalid")
		}
	}
	return raw, cells, campaign, nil
}

func admitEvidence(root, campaignDigest string, cells []cellManifest, secret [32]byte,
	workerDigest string,
) ([]byte, []string, error) {
	if err := exactRootInventory(root, evidenceInventory()); err != nil {
		return nil, nil, err
	}
	var index evidenceIndex
	raw, err := readCanonical(root, "index.json", maximumIndexBytes, &index, false)
	if err != nil || index.Schema != "ardents-stage-6-evidence-index-v1" ||
		index.CampaignSHA256 != campaignDigest || len(index.Cells) != len(expectedCells) {
		return nil, nil, errors.New("evidence index is invalid")
	}
	failures, cursor, total := []string{}, int64(0), int64(len(raw))
	for ordinal, expected := range expectedCells {
		entry := index.Cells[ordinal]
		if entry.ID != expected.id || entry.Ordinal != uint32(ordinal) || entry.EpisodeOrdinal != 0 ||
			entry.TerminalClass == "" || len(entry.Streams) != 1 {
			return nil, nil, errors.New("evidence cell inventory is invalid")
		}
		for _, reference := range []artifact{{Size: entry.Streams[0].Size}, entry.Terminal, entry.Cleanup} {
			if reference.Size <= 0 || total > maximumEvidenceBytes-reference.Size {
				return nil, nil, errors.New("evidence aggregate exceeds its bound")
			}
			total += reference.Size
		}
		prefix := "cells/" + twoDigits(ordinal)
		var trace traceRecord
		if _, err := verifyObservation(root, entry.Streams[0], prefix+"/observations/trace.jsonl", &trace); err != nil {
			return nil, nil, err
		}
		var terminal terminalRecord
		if _, err := verifyArtifact(root, entry.Terminal, prefix+"/terminal.json",
			"ardents-stage-6-terminal-v1", maximumIndexBytes, &terminal, false); err != nil {
			return nil, nil, err
		}
		var cleanup cleanupRecord
		if _, err := verifyArtifact(root, entry.Cleanup, prefix+"/cleanup.json",
			"ardents-stage-6-cleanup-v1", maximumIndexBytes, &cleanup, false); err != nil {
			return nil, nil, err
		}
		stream := entry.Streams[0]
		if trace.Schema != "ardents-stage-6-trace-v1" || trace.Cell != expected.id || trace.Ordinal != uint32(ordinal) ||
			trace.StartOffset < cursor || trace.EndOffset < trace.StartOffset || stream.ObservationStart != trace.StartOffset ||
			stream.ObservationEnd != trace.EndOffset || terminal.Schema != "ardents-stage-6-terminal-v1" ||
			terminal.Cell != expected.id || terminal.Ordinal != uint32(ordinal) || terminal.WorkerPID <= 0 ||
			terminal.WorkerSHA != workerDigest || entry.TerminalClass != terminal.Class ||
			terminal.StartOffset != trace.StartOffset ||
			terminal.EndOffset != trace.EndOffset || cleanup.Schema != "ardents-stage-6-cleanup-v1" ||
			cleanup.Cell != expected.id || cleanup.Ordinal != uint32(ordinal) || len(cleanup.Processes) != 0 ||
			len(cleanup.Listeners) != 0 || len(cleanup.Temporary) != 0 {
			return nil, nil, errors.New("cell terminal or cleanup evidence is invalid")
		}
		if terminal.Class != cells[ordinal].ExpectedClass || !verifyTrace(trace, expected, secret) {
			failures = append(failures, expected.id+":predicate-false")
		}
		cursor = trace.EndOffset
	}
	return raw, failures, nil
}

func readSecret(root, expected string) ([32]byte, error) {
	var value [32]byte
	if err := exactRootInventory(root, []string{"admission-secret.bin"}); err != nil {
		return value, err
	}
	raw, err := readStableFile(filepath.Join(root, "admission-secret.bin"), 32)
	if err != nil || len(raw) != 32 {
		return value, errors.New("admission fixture secret is invalid")
	}
	digest := sha256.Sum256(raw)
	if hex.EncodeToString(digest[:]) != expected {
		return value, errors.New("admission fixture commitment is invalid")
	}
	copy(value[:], raw)
	return value, nil
}

func writeVerdict(root string, verdict Verdict) Verdict {
	if verdict.Diagnostics == nil {
		verdict.Diagnostics = []string{}
	}
	if err := os.Mkdir(root, 0o700); err != nil {
		verdict.Status, verdict.Diagnostics = "invalid", []string{"verdict-root-invalid"}
		return verdict
	}
	raw, err := json.Marshal(verdict)
	if err != nil {
		verdict.Status, verdict.Diagnostics = "invalid", []string{"verdict-encode"}
		return verdict
	}
	file, err := os.OpenFile(filepath.Join(root, "verdict.json"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		verdict.Status, verdict.Diagnostics = "invalid", []string{"verdict-publish"}
		return verdict
	}
	_, writeErr := file.Write(raw)
	if writeErr == nil {
		writeErr = file.Sync()
	}
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		verdict.Status, verdict.Diagnostics = "invalid", []string{"verdict-publish"}
		return verdict
	}
	directory, openErr := os.Open(root)
	if openErr != nil {
		verdict.Status, verdict.Diagnostics = "invalid", []string{"verdict-publish"}
		return verdict
	}
	syncErr := error(nil)
	if runtime.GOOS != "windows" {
		syncErr = directory.Sync()
	}
	closeErr = directory.Close()
	if syncErr != nil || closeErr != nil {
		verdict.Status, verdict.Diagnostics = "invalid", []string{"verdict-publish"}
	}
	return verdict
}

func distinctRoots(values ...string) error {
	seen := []string{}
	for index, value := range values {
		absolute, err := filepath.Abs(value)
		if err != nil {
			return errors.New("S6E1 roots overlap")
		}
		for _, prior := range seen {
			if pathsOverlap(prior, absolute) {
				return errors.New("S6E1 roots overlap")
			}
		}
		seen = append(seen, absolute)
		if index < 3 {
			if err := inspectRoot(absolute); err != nil {
				return err
			}
		}
	}
	return nil
}

func pathsOverlap(left, right string) bool {
	for _, pair := range [][2]string{{left, right}, {right, left}} {
		relative, err := filepath.Rel(pair[0], pair[1])
		if err == nil && (relative == "." || relative != ".." &&
			(len(relative) < 3 || relative[:3] != ".."+string(filepath.Separator))) {
			return true
		}
	}
	return false
}

func executableDigest() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), nil
}

func twoDigits(value int) string { return string([]byte{'0' + byte(value/10), '0' + byte(value%10)}) }
