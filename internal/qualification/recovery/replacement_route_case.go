package recovery

import (
	"crypto/sha256"
	"encoding/json"
)

type routeCase struct {
	RawEvidence        []byte
	EvidenceDigest     [32]byte
	ManifestDigest     [32]byte
	NetworkID          [32]byte
	Generation         string
	Epoch              uint64
	EpochDigest        [32]byte
	Profile            string
	ViewRoot           [32]byte
	SelectionSeed      [32]byte
	SelectionAt        int64
	Candidates         []routeCandidate
	ExcludedIdentities [][32]byte
	ExcludedFamilies   []string
	ExcludedDomains    []string
	NodeIDs            [4][32]byte
	PublicKeys         [4][32]byte
	Families           [4]string
	Endpoints          [4]string
	ClientPin          [32]byte
	PublisherID        [32]byte
	SourceID           string
	BuildDigest        [32]byte
	ExitedPIDs         [6]int
	ExitedRuntimeIDs   [6]string
	ContainerIDs       [6]string
	CleanupVerified    bool
}

type routeCandidate struct {
	NodeID, PublicKey        [32]byte
	Family, Endpoint, Domain string
	Capacity                 uint16
	ValidFrom, ValidUntil    int64
}

func commitRouteCase(input routeCase) ([32]byte, error) {
	input.RawEvidence = nil
	input.EvidenceDigest, input.ManifestDigest = [32]byte{}, [32]byte{}
	input.SourceID, input.BuildDigest = "", [32]byte{}
	input.ExitedPIDs, input.ExitedRuntimeIDs, input.ContainerIDs = [6]int{}, [6]string{}, [6]string{}
	input.CleanupVerified = false
	raw, err := json.Marshal(input)
	return sha256.Sum256(raw), err
}
