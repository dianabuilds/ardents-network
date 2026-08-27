package private

import (
	"crypto/ed25519"
	"net/http"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
	"github.com/openpcc/ohttp"
)

const (
	fixedMessageSize = 4096
	maxEnvelopeSize  = 8 << 10
	requestMediaType = ohttp.RequestMediaType
)

// GatewayConfig is the complete role-local input for one alpha OHTTP Gateway.
// It accepts only one already verified signed Alpha Name Corpus.
type GatewayConfig struct {
	Corpus             *alpha.Corpus
	NodeID             [32]byte
	Family             string
	AssignmentNotAfter time.Time
	IdentityKey        ed25519.PrivateKey
	Clock              func() time.Time
}

// GatewayProfile is the finite common OHTTP configuration authenticated by
// the Gateway's selected identity. It has no Endpoint location or name query.
type GatewayProfile struct {
	NetworkID          [32]byte
	Cohort             string
	NodeID             [32]byte
	Family             string
	KeyConfig          []byte
	KeyConfigDigest    [32]byte
	AssignmentNotAfter time.Time
	Signature          []byte
}

// ClientConfig is one bounded alpha private-resolution attempt. Relay and
// Gateway identities/families are explicit so a caller cannot silently use one
// role for both observations.
type ClientConfig struct {
	RelayURL        string
	RelayNodeID     [32]byte
	RelayFamily     string
	GatewayPublic   ed25519.PublicKey
	Gateway         GatewayProfile
	AuthorityPublic ed25519.PublicKey
	Cohort          string
	Network         [32]byte
	Floor           CorpusFloor
	Base            *http.Transport
}

// CorpusFloor retains accepted Alpha Name Corpus serial/digest facts. Session
// and durable Endpoint-local implementations share this narrow seam; neither
// implementation selects a corpus authority or a destination.
type CorpusFloor interface {
	Observe(*alpha.Corpus) error
}

type gateway struct {
	config  GatewayConfig
	profile GatewayProfile
	handler http.Handler
}

type relay struct {
	gateway string
	client  *http.Client
}

// Client is one single-use alpha private-resolution attempt.
type Client struct {
	config    ClientConfig
	transport *ohttp.Transport
	http      *http.Client

	mu   sync.Mutex
	used bool
}
