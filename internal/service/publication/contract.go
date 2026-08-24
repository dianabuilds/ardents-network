package publication

import (
	"crypto"
	"crypto/ed25519"
	"sync"
	"time"
)

// Credential is the public Authority-signed delegation for one exclusive
// Service Instance generation.
type Credential struct {
	AuthorityPublic [32]byte
	Target          [32]byte
	InstancePublic  [32]byte
	// IntroductionHPKEPublic is the separate X25519 public recipient for
	// SealedIntroduction. It is not derived from InstancePublic.
	IntroductionHPKEPublic [32]byte
	Generation             uint64
	NotBefore              int64
	NotAfter               int64
	NetworkID              [32]byte
	Capabilities           uint32
	Signature              [64]byte
}

// Config owns one publication root. LegacyFloor is read only during the C1
// migration; publication never writes that former H3 generation file.
type Config struct {
	Root        string
	LegacyFloor string
	NetworkID   [32]byte
	Authority   ed25519.PublicKey
	Clock       func() time.Time
}

// PublishInput supplies one fresh, higher-generation live Instance.
type PublishInput struct {
	Credential      Credential
	InstanceSigner  crypto.Signer
	Acknowledgement []byte
	At              time.Time
}

// Current is the public, immutable acquisition fact for a live generation.
// It deliberately has no Instance private material or root path.
type Current struct {
	Credential Credential
	Digest     [32]byte
	Record     []byte
}

// Lease retains one live publication generation until Close. It implements
// crypto.Signer without revealing the volatile Instance private key.
type Lease struct {
	publication *Publication
	generation  *generation
	current     Current
}

// Publication owns the only writer, persistent floor, and live references for
// its root. Its Instance private material is always volatile.
type Publication struct {
	config Config
	root   *durableRoot
	opMu   sync.Mutex
}
