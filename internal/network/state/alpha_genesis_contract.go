package state

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrAlphaGenesisInvalid reports an unsafe root or unsupported operation input.
	ErrAlphaGenesisInvalid = errors.New("functional alpha State genesis input invalid")
	// ErrAlphaGenesisExists reports the fixed output directory already present.
	ErrAlphaGenesisExists = errors.New("functional alpha State genesis already exists")
	// ErrAlphaGenesisPasswordLength reports a passphrase outside the fixed custody policy.
	ErrAlphaGenesisPasswordLength = errors.New("functional alpha State passphrase must contain 16 to 1024 bytes")
	// ErrAlphaGenesisConfirmation reports two distinct local passphrase entries.
	ErrAlphaGenesisConfirmation = errors.New("functional alpha State passphrase confirmation does not match")
)

const (
	alphaGenesisDirectory  = "functional-alpha-state"
	alphaGenesisSeedFile   = "state-seeds.json"
	alphaGenesisPublicFile = "alpha-network-state.json"
)

// AlphaGenesisConfig identifies one existing Product Owner-controlled,
// owner-only local directory outside the repository. The fixed child
// "functional-alpha-state" must be absent. No caller selects the Network
// identifier, key, Epoch, threshold, profile, topology, validity, or output
// names.
type AlphaGenesisConfig struct {
	Root string
}

// AlphaGenesisPrompt tells a trusted terminal Adapter why it requests a secret.
type AlphaGenesisPrompt string

const (
	AlphaGenesisCreate  AlphaGenesisPrompt = "functional-alpha-state-create"
	AlphaGenesisConfirm AlphaGenesisPrompt = "functional-alpha-state-confirm"
)

// AlphaGenesisSecretInput obtains a local secret without accepting it through
// arguments, environment, configuration, or shared application stdin.
type AlphaGenesisSecretInput interface {
	ReadSecret(context.Context, AlphaGenesisPrompt) ([]byte, error)
}

// AlphaGenesisReceipt contains the exact public Network State request fragment
// and the digest of its separately encrypted authority record. Slices are owned
// copies. Inputs and Materials are deliberately empty for the initial topology.
type AlphaGenesisReceipt struct {
	EnvelopeDigest  [32]byte
	NetworkID       [32]byte
	AuthorityPublic [32]byte
	EpochDigest     [32]byte
	Profile         string
	Threshold       uint8
	NotBefore       time.Time
	NotAfter        time.Time
	Epoch           []byte
	Inputs          [][]byte
	Materials       [][]byte
}

// InitializeAlphaGenesis first validates the root and fixed absence, generates
// and verifier-preflights one 30-day empty-topology alpha genesis, then
// reads a new passphrase and confirmation. Passphrases contain 16 to 1024
// bytes. The Implementation uses the fixed 256 MiB, three-pass, four-lane
// Argon2id profile and atomically publishes exactly one encrypted seed record
// plus one public request fragment. It never overwrites or returns private
// material. Cancellation observed before the atomic rename, invalid input,
// failed preflight, secret rejection, or staging failure leaves the fixed child
// absent; once that rename commits, publication wins a concurrent cancellation.
func InitializeAlphaGenesis(ctx context.Context, config AlphaGenesisConfig, secrets AlphaGenesisSecretInput) (AlphaGenesisReceipt, error) {
	return initializeAlphaGenesis(ctx, config, secrets, defaultAlphaGenesisPolicy())
}
