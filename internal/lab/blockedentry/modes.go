package blockedentry

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

func plannedSupplemental(mode string, canaries canaryCorpus) ([]artifactCommitment, []byte) {
	variant, owner, ok := canaryModeVariant(mode)
	if !ok || owner != "pipeline" {
		return nil, nil
	}
	raw := encodedCanarySet(canarySetForVariant(canaries, variant))
	digest := sha256.Sum256(raw)
	return []artifactCommitment{{Path: "pipeline-note.bin", SHA256: hex.EncodeToString(digest[:]),
		Bytes: int64(len(raw))}}, raw
}

func injectMode(result evidence, mode string, canaries canaryCorpus) (evidence, []byte, error) {
	var contamination []byte
	switch mode {
	case "inventory-missing":
		result.Cleanup.Items = result.Cleanup.Items[:len(result.Cleanup.Items)-1]
		result.Cleanup.Complete = false
	}
	if variant, owner, ok := canaryModeVariant(mode); ok && owner == "pipeline" {
		contamination = encodedCanarySet(canarySetForVariant(canaries, variant))
	}
	return result, contamination, nil
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
		owner := "candidate"
		prefix := "candidate-canary-"
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

func canarySetForVariant(corpus canaryCorpus, variant string) canarySet {
	for _, set := range corpus.Sets {
		if set.Variant == variant {
			return set
		}
	}
	return canarySet{}
}

func encodedCanarySet(set canarySet) []byte {
	return []byte(strings.Join([]string{set.Invite, set.Address, set.Path, set.Certificate}, "\n") + "\n")
}
