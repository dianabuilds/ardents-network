package reachability

import (
	"github.com/dianabuilds/ardents-network/internal/service/publication"
)

// Descriptor is an Instance-signed, immutable candidate for current Target
// reachability. The signed bytes include the complete Publication and the
// private recipient exactly; a decoder never merges facts from a caller-built
// plan. ADR-0109 (F-32) retired the generation-2 Introduction member together
// with the whole v1/v2 decode grammar: the private v3 recipient below is the
// only signed live-slot fact this package can compose or verify.
type Descriptor struct {
	Version           uint16
	NetworkID         [32]byte
	Target            [32]byte
	AuthorityPublic   [32]byte
	Publication       []byte
	PublicationDigest [32]byte
	ProfileDigest     [32]byte
	Private           PrivateIntroduction
	Signature         [64]byte
}

// Verified is the exact descriptor fact a User may pass into route
// composition. It exposes no private Instance material or Gateway state.
type Verified struct {
	Descriptor Descriptor
	Current    publication.Current
}
