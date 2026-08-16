package blockedverify

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
)

const (
	maximumPublishableFiles = 128
	maximumPublishableBytes = 64 << 20
	maximumPublishableDepth = 4
)

func validateCanaries(corpus canaryCorpus) ([][]byte, []string, map[string]string, error) {
	if len(corpus.Sets) != 8 {
		return nil, nil, nil, errors.New("private canary corpus omits a secret variant")
	}
	decoded := make([][]byte, 0, 32)
	encoded := make([]string, 0, 32)
	seen := make(map[string]bool, 32)
	commitments := make(map[string]string, 8)
	for index, set := range corpus.Sets {
		if set.Variant != secretVariants()[index] {
			return nil, nil, nil, errors.New("private canary variant order is invalid")
		}
		for _, value := range []string{set.Invite, set.Address, set.Path, set.Certificate} {
			raw, err := hex.DecodeString(value)
			if err != nil || len(raw) != 32 || seen[value] {
				return nil, nil, nil, errors.New("private canary corpus is malformed or repeated")
			}
			seen[value] = true
			decoded = append(decoded, raw)
			encoded = append(encoded, value)
		}
		value := sha256.Sum256([]byte(set.Invite + set.Address + set.Path + set.Certificate))
		commitments[set.Variant] = hex.EncodeToString(value[:])
	}
	return decoded, encoded, commitments, nil
}

func secretVariants() []string {
	return []string{"candidate-leak-invite", "candidate-leak-address", "candidate-leak-path",
		"candidate-leak-certificate", "pipeline-contamination-invite", "pipeline-contamination-address",
		"pipeline-contamination-path", "pipeline-contamination-certificate"}
}

func canaryModeVariant(mode string) (string, string, bool) {
	aliases := map[string]string{"candidate-canary": "candidate-leak-invite",
		"pipeline-canary": "pipeline-contamination-path"}
	if variant := aliases[mode]; variant != "" {
		if strings.HasPrefix(variant, "candidate-") {
			return variant, "candidate", true
		}
		return variant, "pipeline", true
	}
	for _, variant := range secretVariants() {
		owner, prefix := "candidate", "candidate-canary-"
		if strings.HasPrefix(variant, "pipeline-") {
			owner, prefix = "pipeline", "pipeline-canary-"
		}
		field := variant[strings.LastIndexByte(variant, '-')+1:]
		if mode == prefix+field {
			return variant, owner, true
		}
	}
	return "", "", false
}

func canarySetForVariant(corpus canaryCorpus, variant string) (canarySet, bool) {
	for _, set := range corpus.Sets {
		if set.Variant == variant {
			return set, true
		}
	}
	return canarySet{}, false
}

func encodedCanarySet(set canarySet) []byte {
	return []byte(strings.Join([]string{set.Invite, set.Address, set.Path, set.Certificate}, "\n") + "\n")
}

func verifyCandidateCanaryExercise(mode string, corpus canaryCorpus, observed map[string]bool) []string {
	variant, owner, ok := canaryModeVariant(mode)
	if !ok || owner != "candidate" {
		return nil
	}
	set, found := canarySetForVariant(corpus, variant)
	if !found {
		return []string{"candidate canary fixture variant is absent"}
	}
	for _, value := range []string{set.Invite, set.Address, set.Path, set.Certificate} {
		if !observed[value] {
			return []string{"candidate canary fixture did not exercise all four bound secrets"}
		}
	}
	return nil
}

func scanPublishable(root, evidencePath, outputPath string, rawCanaries [][]byte, encodedCanaries []string,
	candidateCanaries map[string]bool,
) (invalid, failures []string) {
	cleanEvidence, _ := filepath.Abs(evidencePath)
	cleanOutput, _ := filepath.Abs(outputPath)
	cleanRoot, _ := filepath.Abs(root)
	fileCount := 0
	var aggregateBytes int64
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			relative, err := filepath.Rel(cleanRoot, path)
			if err != nil || relative != "." && len(splitPath(relative)) > maximumPublishableDepth {
				return errors.New("publishable tree exceeds its depth bound")
			}
			return nil
		}
		fileCount++
		absolute, _ := filepath.Abs(path)
		if absolute == cleanOutput {
			return nil
		}
		data, err := readStableFile(path)
		aggregateBytes += int64(len(data))
		if err != nil || fileCount > maximumPublishableFiles || aggregateBytes > maximumPublishableBytes {
			return errors.New("publishable tree contains an unreadable or oversized artifact")
		}
		for index, raw := range rawCanaries {
			encoded := []byte(encodedCanaries[index])
			rawFound, encodedFound := bytes.Contains(data, raw), bytes.Contains(data, encoded)
			if !rawFound && !encodedFound {
				continue
			}
			if absolute == cleanEvidence && candidateCanaries[encodedCanaries[index]] && !rawFound {
				failures = append(failures, "candidate leaked a private canary into publishable diagnostics")
				continue
			}
			invalid = append(invalid, "publishable evidence contains an unattributed private canary")
		}
		return nil
	})
	if err != nil {
		invalid = append(invalid, err.Error())
	}
	return invalid, failures
}

func splitPath(path string) [][]byte {
	return bytes.FieldsFunc([]byte(path), func(value rune) bool {
		return value == '/' || value == '\\'
	})
}
