package connection

// This receipt is created only after both proofs pass and consumed once by Run.
type initialAuthentication struct {
	attachment *Attachment
	peer       ContinuityPeer
}
