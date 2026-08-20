package stage6verify_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/lab/stage6verify"
)

func TestStage6VerifierRejectsFrozenStructuralMutationMatrix(t *testing.T) {
	base := t.TempDir()
	writeEvidenceCampaign(t, base, "source-commit", "clean")
	tests := map[string]func(string) error{
		"campaign schema": func(root string) error {
			return rewriteCampaign(root, func(value *mutationCampaign) { value.Schema = "wrong" })
		},
		"secret commitment": func(root string) error {
			return rewriteCampaign(root, func(value *mutationCampaign) { value.AdmissionSecretHash = zeroDigest })
		},
		"manifest path": func(root string) error {
			return rewriteCampaign(root, func(value *mutationCampaign) { value.Cells[0].Path = "cells/01.json" })
		},
		"manifest hash": func(root string) error {
			return rewriteCampaign(root, func(value *mutationCampaign) { value.Cells[0].SHA256 = zeroDigest })
		},
		"cell ordinal":  mutateCellOrdinal,
		"cell scenario": mutateCellScenario,
		"evidence schema": func(root string) error {
			return rewriteIndex(root, func(value *mutationIndex) { value.Schema = "wrong" })
		},
		"campaign binding": func(root string) error {
			return rewriteIndex(root, func(value *mutationIndex) { value.CampaignSHA256 = zeroDigest })
		},
		"missing stream": func(root string) error {
			return rewriteIndex(root, func(value *mutationIndex) { value.Cells[0].Streams = nil })
		},
		"episode ordinal": func(root string) error {
			return rewriteIndex(root, func(value *mutationIndex) { value.Cells[0].EpisodeOrdinal = 1 })
		},
		"stream ordinal": func(root string) error {
			return rewriteIndex(root, func(value *mutationIndex) { value.Cells[0].Streams[0].StreamOrdinal = 1 })
		},
		"stream path": func(root string) error {
			return rewriteIndex(root, func(value *mutationIndex) { value.Cells[0].Streams[0].Path = "cells/01/observations/trace.jsonl" })
		},
		"stream hash": func(root string) error {
			return rewriteIndex(root, func(value *mutationIndex) { value.Cells[0].Streams[0].SHA256 = zeroDigest })
		},
		"stream size bound": func(root string) error {
			return rewriteIndex(root, func(value *mutationIndex) { value.Cells[0].Streams[0].Size = 4<<20 + 1 })
		},
		"observation boundary": func(root string) error {
			return rewriteIndex(root, func(value *mutationIndex) {
				value.Cells[0].Streams[0].ObservationStart = value.Cells[0].Streams[0].ObservationEnd + 1
			})
		},
		"trace schema":           mutateTraceSchema,
		"unknown campaign field": mutateUnknownCampaignField,
	}
	for name, mutate := range frozenSchemaMutations() {
		tests[name] = mutate
	}
	for name, mutate := range frozenArtifactMutations() {
		tests[name] = mutate
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			if err := cloneBundle(base, root); err != nil {
				t.Fatal(err)
			}
			if err := mutate(root); err != nil {
				t.Fatal(err)
			}
			verdict := (stage6verify.Stage6Verifier{}).Verify(filepath.Join(root, "manifest"),
				filepath.Join(root, "evidence"), filepath.Join(root, "private"), filepath.Join(root, "verdict"))
			if verdict.Status != "invalid" {
				t.Fatalf("verdict=%+v", verdict)
			}
		})
	}
}

const zeroDigest = "0000000000000000000000000000000000000000000000000000000000000000"

type mutationCampaign struct {
	Schema              string             `json:"schema"`
	Profile             string             `json:"profile"`
	RunID               string             `json:"run_id"`
	SourceCommit        string             `json:"source_commit"`
	DirtyDigest         string             `json:"dirty_digest"`
	LauncherSHA256      string             `json:"launcher_sha256"`
	WorkerSHA256        string             `json:"worker_sha256"`
	Platform            string             `json:"platform"`
	Toolchain           string             `json:"toolchain"`
	ClockOrigin         int64              `json:"clock_origin"`
	AdmissionSecretHash string             `json:"admission_secret_sha256"`
	Decisions           []string           `json:"decisions"`
	Cells               []mutationArtifact `json:"cells"`
}

type mutationCellManifest struct {
	Schema          string   `json:"schema"`
	ID              string   `json:"id"`
	Ordinal         uint32   `json:"ordinal"`
	Scenario        string   `json:"scenario"`
	ExpectedClass   string   `json:"expected_class"`
	Predicate       string   `json:"predicate"`
	RequiredStreams []string `json:"required_streams"`
}

func rewriteCampaign(root string, mutate func(*mutationCampaign)) error {
	path := filepath.Join(root, "manifest", "campaign.json")
	var value mutationCampaign
	if err := readMutationJSON(path, &value); err != nil {
		return err
	}
	mutate(&value)
	return writeMutationJSON(path, value, false)
}

func rewriteIndex(root string, mutate func(*mutationIndex)) error {
	path := filepath.Join(root, "evidence", "index.json")
	var value mutationIndex
	if err := readMutationJSON(path, &value); err != nil {
		return err
	}
	mutate(&value)
	return writeMutationJSON(path, value, false)
}

func mutateCellOrdinal(root string) error {
	path := filepath.Join(root, "manifest", "cells", "00.json")
	var cell mutationCellManifest
	if err := readMutationJSON(path, &cell); err != nil {
		return err
	}
	cell.Ordinal = 1
	if err := writeMutationJSON(path, cell, false); err != nil {
		return err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := rewriteCampaign(root, func(value *mutationCampaign) { updateMutationArtifact(&value.Cells[0], raw) }); err != nil {
		return err
	}
	return rebindEvidenceCampaign(root)
}

func rebindEvidenceCampaign(root string) error {
	raw, err := os.ReadFile(filepath.Join(root, "manifest", "campaign.json"))
	if err != nil {
		return err
	}
	digest := sha256.Sum256(raw)
	return rewriteIndex(root, func(value *mutationIndex) {
		value.CampaignSHA256 = hex.EncodeToString(digest[:])
	})
}

func mutateCellScenario(root string) error {
	path := filepath.Join(root, "manifest", "cells", "00.json")
	var cell mutationCellManifest
	if err := readMutationJSON(path, &cell); err != nil {
		return err
	}
	cell.Scenario = "different scenario"
	if err := writeMutationJSON(path, cell, false); err != nil {
		return err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := rewriteCampaign(root, func(value *mutationCampaign) { updateMutationArtifact(&value.Cells[0], raw) }); err != nil {
		return err
	}
	return rebindEvidenceCampaign(root)
}

func mutateTraceSchema(root string) error {
	path := filepath.Join(root, "evidence", "cells", "00", "observations", "trace.jsonl")
	var trace mutationTrace
	if err := readMutationJSONL(path, &trace); err != nil {
		return err
	}
	trace.Schema = "wrong"
	if err := writeMutationJSON(path, trace, true); err != nil {
		return err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return rewriteIndex(root, func(value *mutationIndex) {
		value.Cells[0].Streams[0].Schema = "ardents-stage-6-trace-v1"
		value.Cells[0].Streams[0].Size = int64(len(raw))
		digest := sha256.Sum256(raw)
		value.Cells[0].Streams[0].SHA256 = hex.EncodeToString(digest[:])
	})
}

func mutateUnknownCampaignField(root string) error {
	path := filepath.Join(root, "manifest", "campaign.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	raw = append(raw[:len(raw)-1], []byte(`,"unexpected":true}`)...)
	return os.WriteFile(path, raw, 0o600)
}

func updateMutationArtifact(value *mutationArtifact, raw []byte) {
	digest := sha256.Sum256(raw)
	value.Size, value.SHA256 = int64(len(raw)), hex.EncodeToString(digest[:])
}

func readMutationJSON(path string, value any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, value)
}

func readMutationJSONL(path string, value any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw[:len(raw)-1], value)
}

func writeMutationJSON(path string, value any, jsonl bool) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if jsonl {
		raw = append(raw, '\n')
	}
	return os.WriteFile(path, raw, 0o600)
}

func cloneBundle(source, target string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil || relative == "." {
			return err
		}
		destination := filepath.Join(target, relative)
		if entry.IsDir() {
			return os.Mkdir(destination, 0o700)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(destination, raw, 0o600)
	})
}
