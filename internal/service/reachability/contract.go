package reachability

import (
	"crypto"
	"crypto/ed25519"
	"time"

	"github.com/dianabuilds/ardents-network/internal/service/publication"
)

const (
	// MaximumDescriptorSize bounds one complete Descriptor before any caller
	// allocates a Gateway or private-resolution message.
	MaximumDescriptorSize = 4096
	maximumAuthorization  = 1024
)

// SubmissionMode is the closed Introduction-admission declaration signed in a
// Reachability Descriptor. It never grants Service access: the sealed
// JoinHandle and Publisher Application retain that decision.
type SubmissionMode byte

const (
	// SubmissionFixedGrant means that the Descriptor carries exactly one
	// pre-issued Introduction Transit Grant. It is Descriptor v1 only.
	SubmissionFixedGrant SubmissionMode = 1
	// SubmissionMembershipGrant means that an Endpoint must obtain one fresh
	// membership-level Transit Grant through the selected Credential Relay. It
	// is Descriptor v2 only and carries no embedded per-Service credential.
	SubmissionMembershipGrant SubmissionMode = 2
)

// Introduction is the target-scoped, State-bound live-slot fact published by
// a current Service Instance. It deliberately contains Node identities but no
// endpoint literals: the User obtains those only from its own State view.
type Introduction struct {
	StateDigest                          [32]byte
	Epoch                                uint64
	IntroductionNodeID, RendezvousNodeID [32]byte
	Reachability, JoinHandle             [32]byte
	NotAfter                             time.Time
	SubmissionMode                       SubmissionMode
	SubmissionAuthorization              []byte
}

// Descriptor is an Instance-signed, immutable candidate for current Target
// reachability. The signed bytes include Publication and Introduction exactly;
// a decoder never merges facts from a caller-built plan.
type Descriptor struct {
	Version           uint16
	NetworkID         [32]byte
	Target            [32]byte
	AuthorityPublic   [32]byte
	Publication       []byte
	PublicationDigest [32]byte
	Introduction      Introduction
	Signature         [64]byte
}

// IssueInput contains a Publisher's current immutable Publication and a
// bounded live Introduction slot. InstanceSigner must be the private key of
// Publication.Credential.InstancePublic.
type IssueInput struct {
	Current        publication.Current
	Introduction   Introduction
	InstanceSigner crypto.Signer
}

// Verified is the exact descriptor fact a User may pass into route
// composition. It exposes no private Instance material or Gateway state.
type Verified struct {
	Descriptor Descriptor
	Current    publication.Current
}

// Authority returns a copy of the authenticated Service Authority public key.
func (value Verified) Authority() ed25519.PublicKey {
	return ed25519.PublicKey(append([]byte(nil), value.Descriptor.AuthorityPublic[:]...))
}
