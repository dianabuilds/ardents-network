package node

// Result is the terminal machine outcome of one Node operation.
type Result struct {
	Verdict        string `json:"verdict"`
	Reason         string `json:"reason"`
	EvidenceRoot   string `json:"evidence_root,omitempty"`
	EvidenceDigest string `json:"evidence_digest,omitempty"`
}
