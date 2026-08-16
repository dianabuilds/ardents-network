package blockedentry

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
)

func createCanaries(secretRoot string) (canaryCorpus, string, error) {
	var corpus canaryCorpus
	for _, variant := range secretVariants() {
		values := make([][]byte, 4)
		for index := range values {
			values[index] = make([]byte, 32)
			if _, err := rand.Read(values[index]); err != nil {
				return canaryCorpus{}, "", err
			}
		}
		corpus.Sets = append(corpus.Sets, canarySet{Variant: variant,
			Invite: hex.EncodeToString(values[0]), Address: hex.EncodeToString(values[1]),
			Path: hex.EncodeToString(values[2]), Certificate: hex.EncodeToString(values[3])})
	}
	path := filepath.Join(secretRoot, "canaries.json")
	if err := writeJSON(path, corpus); err != nil {
		return canaryCorpus{}, "", err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return canaryCorpus{}, "", err
	}
	digest := sha256.Sum256(raw)
	return corpus, hex.EncodeToString(digest[:]), nil
}

func secretVariants() []string {
	return []string{"candidate-leak-invite", "candidate-leak-address", "candidate-leak-path",
		"candidate-leak-certificate", "pipeline-contamination-invite", "pipeline-contamination-address",
		"pipeline-contamination-path", "pipeline-contamination-certificate"}
}

func canaryCommitments(corpus canaryCorpus) map[string]string {
	result := make(map[string]string, len(corpus.Sets))
	for _, set := range corpus.Sets {
		raw := []byte(set.Invite + set.Address + set.Path + set.Certificate)
		value := sha256.Sum256(raw)
		result[set.Variant] = hex.EncodeToString(value[:])
	}
	return result
}
