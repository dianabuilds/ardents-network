package enrollment

import "context"

// CandidateFacts are self-consistent declarations, never authenticated initial
// facts. ManifestSHA256 is an observed checksum, not an independent pin.
type CandidateFacts Facts

// Candidate retains untrusted frozen distribution bytes for a subsequent
// Release evaluation. It has neither first-pin nor running-program provenance;
// it cannot be converted to a verified Bundle. Its zero value has no snapshot.
type Candidate struct{ loaded *snapshot }

// ReadCandidate checks the same bounded inventory and native file identities
// as Verify, without accepting an initial pin or claiming the candidate is the
// running executable. Authenticity and successor admission remain elsewhere.
func ReadCandidate(ctx context.Context, root string, scope Scope) (Candidate, error) {
	if ctx == nil || root == "" || (scope != General && scope != Headless) {
		return Candidate{}, ErrInput
	}
	s, err := loadSnapshot(ctx, root, scope, nil)
	if err != nil {
		return Candidate{}, err
	}
	return Candidate{loaded: s}, nil
}

func (c Candidate) Facts() (CandidateFacts, bool) {
	if c.loaded == nil {
		return CandidateFacts{}, false
	}
	return CandidateFacts(c.loaded.facts), true
}

func (c Candidate) File(name string) ([]byte, bool) { return c.loaded.file(name) }

func (c Candidate) Names() []string { return c.loaded.names() }

// MetadataNames classifies frozen inventory only. The consumer owns metadata
// URLs; Release authenticates the bytes and retains its own durable floors.
func (c Candidate) MetadataNames() []string { return c.loaded.metadataNames() }
