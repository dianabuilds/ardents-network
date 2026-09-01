package node

import (
	"crypto/tls"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// introductionConfig supplies one State-pinned Introduction listener and its
// narrow opaque transit-admission port. It never receives a Service Target,
// HPKE private key, Publisher identity, or route plan.
type introductionConfig struct {
	ListenAddress                  string
	Certificate                    tls.Certificate
	NetworkID, EpochDigest, NodeID [32]byte
	NodePublicKey                  [32]byte
	Epoch                          uint64
	NotAfter                       time.Time
	Admit                          route.EndpointTransitBindingAdmitter
	HandshakeLimit, SlotLimit      uint16
	DeliveryLimit                  uint16
	AdmissionTimeout               time.Duration
	DrainTimeout                   time.Duration
}

// IntroductionUsage is aggregate non-secret bounded work. A delivery count is
// sealed-byte forwarding only; it never asserts Publisher or Service success.
type introductionUsage struct {
	Handshakes, Slots, Deliveries, Connections uint16
	Registered, Delivered, Unavailable         uint64
	RefusedBeforeTLS                           uint64
}

type introductionPlan struct {
	introductionConfig
	now func() time.Time
}

func newIntroductionPlan(input introductionConfig) (introductionPlan, error) {
	if !literalNodeEndpoint(input.ListenAddress) || input.NetworkID == [32]byte{} || input.EpochDigest == [32]byte{} ||
		input.NodeID == [32]byte{} || input.NodePublicKey == [32]byte{} || input.Epoch == 0 || input.NotAfter.IsZero() ||
		input.Admit == nil || input.HandshakeLimit == 0 || input.SlotLimit == 0 || input.DeliveryLimit == 0 ||
		!validAdmissionTimeout(input.AdmissionTimeout) || input.DrainTimeout <= 0 || input.DrainTimeout > time.Minute || !input.NotAfter.Equal(input.NotAfter.UTC().Truncate(time.Second)) ||
		!time.Now().UTC().Before(input.NotAfter) {
		return introductionPlan{}, errors.New("Introduction duty configuration is incomplete or outside its implementation bound")
	}
	if err := validateNodeCertificate(input.Certificate, input.NodePublicKey); err != nil {
		return introductionPlan{}, err
	}
	return introductionPlan{introductionConfig: input, now: func() time.Time { return time.Now().UTC() }}, nil
}
