package custody

import "fmt"

// inspectTestEnvelope preserves the predecessor codec oracle after the real
// public inspect command moved to the new Custody owner.
func inspectTestEnvelope(path string) (EnvelopeInfo, error) {
	raw, err := readEnvelopeFile(path)
	if err != nil {
		return EnvelopeInfo{}, fmt.Errorf("read custody envelope: %w", err)
	}
	return inspectEnvelope(raw)
}
