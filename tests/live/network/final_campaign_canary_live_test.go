//go:build live

package network_test

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type finalCandidateCanaryCorpus struct {
	Sets []finalCandidateCanarySet `json:"sets"`
}

type finalCandidateCanarySet struct {
	Variant     string `json:"variant"`
	Invite      string `json:"invite"`
	Address     string `json:"address"`
	Path        string `json:"path"`
	Certificate string `json:"certificate"`
}

func TestFinalCandidateCanaryPassesOnlyOneSelectedSecret(t *testing.T) {
	sets := make([]finalCandidateCanarySet, 0, 8)
	for index, variant := range []string{"candidate-leak-invite", "candidate-leak-address",
		"candidate-leak-path", "candidate-leak-certificate", "pipeline-contamination-invite",
		"pipeline-contamination-address", "pipeline-contamination-path", "pipeline-contamination-certificate"} {
		value := strings.Repeat(string(rune('a'+index)), 64)
		sets = append(sets, finalCandidateCanarySet{Variant: variant, Invite: value, Address: value,
			Path: value, Certificate: value})
	}
	raw, err := json.Marshal(finalCandidateCanaryCorpus{Sets: sets})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "canaries.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	value, err := finalCandidateCanary("hostile/G9-ledger-leakage/candidate-leak-path/2", path)
	if err != nil || value != strings.Repeat("c", 64) {
		t.Fatalf("canary=%q err=%v", value, err)
	}
}

func finalCandidateCanary(cell, path string) (string, error) {
	parts := strings.Split(cell, "/")
	if len(parts) != 4 || parts[1] != "G9-ledger-leakage" || !strings.HasPrefix(parts[2], "candidate-leak-") {
		return "", nil
	}
	input, err := os.Open(path)
	if err != nil {
		return "", errors.Join(err, errors.New("candidate canary corpus is unavailable"))
	}
	defer input.Close()
	var corpus finalCandidateCanaryCorpus
	decoder := json.NewDecoder(io.LimitReader(input, 64<<10))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&corpus) != nil || decoder.Decode(&struct{}{}) != io.EOF || len(corpus.Sets) != 8 {
		return "", errors.New("candidate canary corpus is invalid")
	}
	for _, set := range corpus.Sets {
		if set.Variant != parts[2] {
			continue
		}
		value := map[string]string{"invite": set.Invite, "address": set.Address,
			"path": set.Path, "certificate": set.Certificate}[parts[2][len("candidate-leak-"):]]
		raw, decodeErr := hex.DecodeString(value)
		if decodeErr != nil || len(raw) != 32 || bytes.Equal(raw, make([]byte, 32)) {
			return "", errors.New("candidate canary value is invalid")
		}
		return value, nil
	}
	return "", errors.New("candidate canary variant is absent")
}
