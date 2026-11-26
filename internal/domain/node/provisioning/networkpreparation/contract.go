package networkpreparation

import "errors"

// ErrOutput reports an incomplete or refused output attempt.
var ErrOutput = errors.New("network preparation output unavailable")

// Artifact identifies the exact completed unsigned artifact.
type Artifact struct {
	Name   string
	Digest [32]byte
}
