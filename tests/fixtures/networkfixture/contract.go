package networkfixture

import (
	"crypto/ed25519"
	"time"
)

// RecordSpec describes one signed Node Record fixture input.
type RecordSpec struct {
	NetworkID             [32]byte
	NodeID                [32]byte
	Generation            uint64
	ValidFrom, ValidUntil time.Time
	Family, Endpoint      string
	Capability            byte
	Capacity              uint16
	PrivateKey            ed25519.PrivateKey
}

// Record retains canonical bytes and fields needed to build an Epoch view.
type Record struct {
	Raw      []byte
	NodeID   [32]byte
	Family   string
	Capacity uint16
}

// EpochSpec describes one signed Epoch fixture and its expected view.
type EpochSpec struct {
	NetworkID             [32]byte
	Number                uint64
	Previous              [32]byte
	ValidFrom, ValidUntil time.Time
	Inputs                [][]byte
	Accepted              []Record
	Rejections            map[uint32]uint16
	AssignmentSeed        [32]byte
	Profile               string
	Domains               []string
	Authorities           []ed25519.PrivateKey
}

// Epoch contains canonical bytes, identity, and materializations.
type Epoch struct {
	Number    uint64
	Seed      [32]byte
	Raw       []byte
	Digest    [32]byte
	Inputs    [][]byte
	Materials [][]byte
}
