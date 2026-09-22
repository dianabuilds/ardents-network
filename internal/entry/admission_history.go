package entry

// maximumAdmissions bounds decoding of retained Initiator admission history.
// The accepting engine is retired, but existing owner roots remain immutable
// compatibility evidence until their separate data disposition.
const maximumAdmissions = 64

type admissionRecord struct {
	InviteID        [32]byte `json:"invite_id"`
	AttachmentID    [32]byte `json:"attachment_id"`
	ClientKeyDigest [32]byte `json:"client_key_digest"`
	NotAfter        int64    `json:"not_after_unix"`
}
