package custody

import "fmt"

// InspectEnvelope validates one bounded canonical envelope and returns only
// its public header. Inspection owns no Vault root and cannot create or mutate
// custody state.
func InspectEnvelope(path string) (EnvelopeInfo, error) {
	raw, err := readEnvelopeFile(path)
	if err != nil {
		return EnvelopeInfo{}, fmt.Errorf("read custody envelope: %w", err)
	}
	return inspectEnvelope(raw)
}
