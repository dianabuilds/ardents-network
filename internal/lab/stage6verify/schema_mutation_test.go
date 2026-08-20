package stage6verify_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
)

type mutationTerminal struct {
	Schema      string `json:"schema"`
	Cell        string `json:"cell"`
	Ordinal     uint32 `json:"ordinal"`
	Class       string `json:"class"`
	WorkerPID   int64  `json:"worker_pid"`
	WorkerSHA   string `json:"worker_sha256"`
	StartOffset int64  `json:"start_offset_millis"`
	EndOffset   int64  `json:"end_offset_millis"`
}

type mutationCleanup struct {
	Schema    string   `json:"schema"`
	Cell      string   `json:"cell"`
	Ordinal   uint32   `json:"ordinal"`
	Processes []string `json:"processes"`
	Listeners []string `json:"listeners"`
	Temporary []string `json:"temporary"`
}

func frozenSchemaMutations() map[string]func(string) error {
	return map[string]func(string) error{
		"campaign profile":         campaignMutation(func(v *mutationCampaign) { v.Profile = "wrong" }),
		"campaign run id":          campaignMutation(func(v *mutationCampaign) { v.RunID = "00" }),
		"campaign source":          campaignMutation(func(v *mutationCampaign) { v.SourceCommit = "" }),
		"campaign dirty digest":    campaignMutation(func(v *mutationCampaign) { v.DirtyDigest = "" }),
		"campaign launcher hash":   campaignMutation(func(v *mutationCampaign) { v.LauncherSHA256 = "bad" }),
		"campaign worker hash":     campaignMutation(func(v *mutationCampaign) { v.WorkerSHA256 = "bad" }),
		"campaign platform":        campaignMutation(func(v *mutationCampaign) { v.Platform = "" }),
		"campaign toolchain":       campaignMutation(func(v *mutationCampaign) { v.Toolchain = "" }),
		"campaign clock origin":    campaignMutation(func(v *mutationCampaign) { v.ClockOrigin = 1 }),
		"campaign decisions":       campaignMutation(func(v *mutationCampaign) { v.Decisions = v.Decisions[1:] }),
		"campaign decision order":  campaignMutation(func(v *mutationCampaign) { v.Decisions[0], v.Decisions[1] = v.Decisions[1], v.Decisions[0] }),
		"campaign cell count":      campaignMutation(func(v *mutationCampaign) { v.Cells = v.Cells[:len(v.Cells)-1] }),
		"campaign cell order":      campaignMutation(func(v *mutationCampaign) { v.Cells[0], v.Cells[1] = v.Cells[1], v.Cells[0] }),
		"manifest artifact schema": campaignMutation(func(v *mutationCampaign) { v.Cells[0].Schema = "wrong" }),
		"manifest artifact size":   campaignMutation(func(v *mutationCampaign) { v.Cells[0].Size++ }),

		"cell schema":           cellManifestMutation(func(v *mutationCellManifest) { v.Schema = "wrong" }),
		"cell id":               cellManifestMutation(func(v *mutationCellManifest) { v.ID = "A1" }),
		"cell expected class":   cellManifestMutation(func(v *mutationCellManifest) { v.ExpectedClass = "wrong" }),
		"cell predicate":        cellManifestMutation(func(v *mutationCellManifest) { v.Predicate = "wrong" }),
		"cell required streams": cellManifestMutation(func(v *mutationCellManifest) { v.RequiredStreams = nil }),

		"evidence cell count":     indexMutation(func(v *mutationIndex) { v.Cells = v.Cells[:len(v.Cells)-1] }),
		"evidence cell order":     indexMutation(func(v *mutationIndex) { v.Cells[0], v.Cells[1] = v.Cells[1], v.Cells[0] }),
		"evidence cell id":        indexMutation(func(v *mutationIndex) { v.Cells[0].ID = "A1" }),
		"evidence cell ordinal":   indexMutation(func(v *mutationIndex) { v.Cells[0].Ordinal = 1 }),
		"evidence terminal class": indexMutation(func(v *mutationIndex) { v.Cells[0].TerminalClass = "wrong" }),
		"stream count":            indexMutation(func(v *mutationIndex) { v.Cells[0].Streams = append(v.Cells[0].Streams, v.Cells[0].Streams[0]) }),
		"stream schema":           indexMutation(func(v *mutationIndex) { v.Cells[0].Streams[0].Schema = "wrong" }),
		"stream role":             indexMutation(func(v *mutationIndex) { v.Cells[0].Streams[0].Role = "launcher" }),
		"stream episode":          indexMutation(func(v *mutationIndex) { v.Cells[0].Streams[0].EpisodeOrdinal = 1 }),
		"stream negative start":   indexMutation(func(v *mutationIndex) { v.Cells[0].Streams[0].ObservationStart = -1 }),
		"stream reversed boundary": indexMutation(func(v *mutationIndex) {
			v.Cells[0].Streams[0].ObservationEnd = v.Cells[0].Streams[0].ObservationStart - 1
		}),
		"terminal artifact path":   indexMutation(func(v *mutationIndex) { v.Cells[0].Terminal.Path = "cells/01/terminal.json" }),
		"terminal artifact schema": indexMutation(func(v *mutationIndex) { v.Cells[0].Terminal.Schema = "wrong" }),
		"terminal artifact size":   indexMutation(func(v *mutationIndex) { v.Cells[0].Terminal.Size++ }),
		"terminal artifact hash":   indexMutation(func(v *mutationIndex) { v.Cells[0].Terminal.SHA256 = zeroDigest }),
		"cleanup artifact path":    indexMutation(func(v *mutationIndex) { v.Cells[0].Cleanup.Path = "cells/01/cleanup.json" }),
		"cleanup artifact schema":  indexMutation(func(v *mutationIndex) { v.Cells[0].Cleanup.Schema = "wrong" }),
		"cleanup artifact size":    indexMutation(func(v *mutationIndex) { v.Cells[0].Cleanup.Size++ }),
		"cleanup artifact hash":    indexMutation(func(v *mutationIndex) { v.Cells[0].Cleanup.SHA256 = zeroDigest }),

		"terminal schema":           terminalMutation(func(v *mutationTerminal) { v.Schema = "wrong" }),
		"terminal cell":             terminalMutation(func(v *mutationTerminal) { v.Cell = "A1" }),
		"terminal ordinal":          terminalMutation(func(v *mutationTerminal) { v.Ordinal = 1 }),
		"terminal worker pid":       terminalMutation(func(v *mutationTerminal) { v.WorkerPID = 0 }),
		"terminal worker hash":      terminalMutation(func(v *mutationTerminal) { v.WorkerSHA = zeroDigest }),
		"terminal start offset":     terminalMutation(func(v *mutationTerminal) { v.StartOffset++ }),
		"terminal end offset":       terminalMutation(func(v *mutationTerminal) { v.EndOffset++ }),
		"cleanup schema":            cleanupMutation(func(v *mutationCleanup) { v.Schema = "wrong" }),
		"cleanup cell":              cleanupMutation(func(v *mutationCleanup) { v.Cell = "A1" }),
		"cleanup ordinal":           cleanupMutation(func(v *mutationCleanup) { v.Ordinal = 1 }),
		"cleanup process residue":   cleanupMutation(func(v *mutationCleanup) { v.Processes = []string{"worker"} }),
		"cleanup listener residue":  cleanupMutation(func(v *mutationCleanup) { v.Listeners = []string{"listener"} }),
		"cleanup temporary residue": cleanupMutation(func(v *mutationCleanup) { v.Temporary = []string{"root"} }),
		"trace cell":                traceMutation(func(v *mutationTrace) { v.Cell = "A1" }),
		"trace ordinal":             traceMutation(func(v *mutationTrace) { v.Ordinal = 1 }),
	}
}

func campaignMutation(mutate func(*mutationCampaign)) func(string) error {
	return func(root string) error {
		if err := rewriteCampaign(root, mutate); err != nil {
			return err
		}
		return rebindEvidenceCampaign(root)
	}
}

func indexMutation(mutate func(*mutationIndex)) func(string) error {
	return func(root string) error { return rewriteIndex(root, mutate) }
}

func traceMutation(mutate func(*mutationTrace)) func(string) error {
	return func(root string) error { return committedTraceMutation(root, 0, mutate) }
}

func cellManifestMutation(mutate func(*mutationCellManifest)) func(string) error {
	return func(root string) error {
		path := filepath.Join(root, "manifest", "cells", "00.json")
		var value mutationCellManifest
		if err := readMutationJSON(path, &value); err != nil {
			return err
		}
		mutate(&value)
		if err := writeMutationJSON(path, value, false); err != nil {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err = rewriteCampaign(root, func(campaign *mutationCampaign) {
			updateMutationArtifact(&campaign.Cells[0], raw)
		}); err != nil {
			return err
		}
		return rebindEvidenceCampaign(root)
	}
}

func terminalMutation(mutate func(*mutationTerminal)) func(string) error {
	return committedCellFileMutation("terminal.json", func(index *mutationIndex) *mutationArtifact {
		return &index.Cells[0].Terminal
	}, func(path string) error {
		var value mutationTerminal
		if err := readMutationJSON(path, &value); err != nil {
			return err
		}
		mutate(&value)
		return writeMutationJSON(path, value, false)
	})
}

func cleanupMutation(mutate func(*mutationCleanup)) func(string) error {
	return committedCellFileMutation("cleanup.json", func(index *mutationIndex) *mutationArtifact {
		return &index.Cells[0].Cleanup
	}, func(path string) error {
		var value mutationCleanup
		if err := readMutationJSON(path, &value); err != nil {
			return err
		}
		mutate(&value)
		return writeMutationJSON(path, value, false)
	})
}

func committedCellFileMutation(name string, reference func(*mutationIndex) *mutationArtifact,
	mutate func(string) error,
) func(string) error {
	return func(root string) error {
		path := filepath.Join(root, "evidence", "cells", "00", name)
		if err := mutate(path); err != nil {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		digest := sha256.Sum256(raw)
		return rewriteIndex(root, func(index *mutationIndex) {
			artifact := reference(index)
			artifact.Size = int64(len(raw))
			artifact.SHA256 = hex.EncodeToString(digest[:])
		})
	}
}
