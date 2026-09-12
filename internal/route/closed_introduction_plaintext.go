package route

import (
	"time"
)

const closedIntroductionPlaintextSize = 344
const closedIntroductionCiphertextSize = closedIntroductionPlaintextSize + 16

// ClosedIntroductionPlaintext is recipient-only. These fields are candidate
// input, never caller-established authority or permission to dial a Node.
type ClosedIntroductionPlaintext struct {
	Network, Target, PublicationDigest                           [32]byte
	Revision                                                     uint64
	RendezvousNode                                               [32]byte
	RendezvousDutyGeneration                                     uint64
	JoinSecret, HandshakeContext, ProfileDigest, ConnectionNonce [32]byte
	AttachmentGeneration                                         uint64
	Deadline                                                     time.Time
	InitiatorBinding                                             [32]byte
	WorkSafetyNotAfter, WorkSafetyMaximum, NoNewRecoveryAfter    int64
}
