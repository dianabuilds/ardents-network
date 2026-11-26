package epoch

import "time"

// Header is a bounded parsed Epoch envelope identity. Inspect checks grammar
// and framing only; it does not authenticate signatures or commitments.
type Header struct {
	Version    byte
	NetworkID  [32]byte
	Number     uint64
	Digest     [32]byte
	Previous   [32]byte
	ValidFrom  time.Time
	ValidUntil time.Time
	Profile    string
	Cutoff     uint32
}

// Inspect parses one untrusted Epoch envelope for bounded identity comparison.
// Only Verify returns an authenticated Decision.
func Inspect(raw []byte) (Header, error) {
	parsed, err := parseEpoch(raw)
	if err != nil {
		return Header{}, err
	}
	return headerFromEnvelope(parsed), nil
}

func headerFromEnvelope(value epochEnvelope) Header {
	return Header{Version: value.version, NetworkID: value.networkID,
		Number: value.number, Digest: value.digest, Previous: value.previous,
		ValidFrom: value.validFrom, ValidUntil: value.validUntil,
		Profile: value.profile, Cutoff: value.cutoff}
}
