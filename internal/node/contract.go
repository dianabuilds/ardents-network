package node

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/resource"
)

const eventSchema = "ardents-node-event-v1"

// RendezvousDedicatedHostResourceProfile is the only selected native Node
// resource profile and is restricted to the dedicated Rendezvous duty.
const RendezvousDedicatedHostResourceProfile = resource.RendezvousDedicatedHostProfile

// Config binds one local identity, authenticated duty facts, and private role-probe listener.
type Config struct {
	// HostingRoot is the single installed provider period shared by all closed duties on this host.
	HostingRoot string
	// ClosedListenOverride is an optional private local bind address behind an
	// operator-owned relay. State remains authoritative for the advertised
	// endpoint used by every peer.
	ClosedListenOverride string
	NetworkID            [32]byte
	NodeID               [32]byte
	IdentityKey          ed25519.PrivateKey
	// Current supplies the one State-created copied duty value per poll.
	// Production wires the State owner's CurrentNodeDuty; the value carries no
	// Network State persistence, source, retry, or pending metadata. Node
	// revalidates its bounds on receipt and retains its per-poll copy.
	Current      func() (state.NodeDuty, error)
	Probe        ProbeConfig
	ClosedIssuer ClosedIssuerProfile
	// ClosedForwarding supplies the isolated receiving spend journal and Node
	// TLS key for an accepted generation-3 adjacent/interior forwarding duty.
	// State still selects the endpoint, peer and recipient assignment.
	ClosedForwarding   ClosedForwardingProfile
	ClosedResolution   ClosedResolutionProfile
	ClosedIntroduction ClosedIntroductionProfile
	ClosedDataJoin     ClosedDataJoinProfile
	// CurrentClosedProfile exposes only State's already accepted closed
	// profile. It is unavailable instead of choosing profile bytes or a trust
	// root from the Node plan.
	CurrentClosedProfile func() (state.ClosedProfileView, bool)
	// CurrentClosedRoute exposes the same accepted profile's recipient facts.
	// It is unavailable rather than permitting Node to manufacture a recipient
	// digest, role-domain or duty generation.
	CurrentClosedRoute func() (state.ClosedRouteView, error)
	PollInterval       time.Duration
	Quarantine         time.Duration
	ResourceProfile    string
	NetworkStateRoot   string
	LocalRoleStateRoot string
	// ResourceMeasure and CheckPlacement are behavior-test seams. Maintained
	// runtime callers leave them nil and use ResourceProfile's platform adapter.
	ResourceMeasure func() (resource.Sample, error)
	Now             func() time.Time
	CheckPlacement  func() error
	// Emit must honor ctx cancellation and return before its deadline.
	Emit func(context.Context, Event) error
}

// ClosedIssuerProfile contains the isolated RSA-PSS issuer root and bounded
// direct role listener reservation. State selects its endpoint, carrier and
// current issuer profile; this local profile cannot select a recipient.
type ClosedIssuerProfile struct {
	Root            string
	AdmissionRoot   string
	Certificate     tls.Certificate
	ConnectionLimit uint16
	DrainTimeout    time.Duration
}

// ClosedForwardingProfile contains the local material for one closed Route
// forwarding duty. Root is exclusively owned by its current receiving duty;
// it must not share an issuer root or survive a changed duty generation.
type ClosedForwardingProfile struct {
	Root            string
	Certificate     tls.Certificate
	ConnectionLimit uint16
	DrainTimeout    time.Duration
	// HostingRoot names the installed shared provider-period ledger. It is
	// local operator configuration, never State or token material.
	HostingRoot string
	// CarrierRelayEndpoint is an optional operator-owned transparent relay
	// address for this forwarding Node's State-selected next Carrier. It cannot
	// change the selected Node identity, key, duty, profile, or TLS verification.
	CarrierRelayEndpoint string
	AdmissionTraffic     resource.HostingTraffic
	TerminationTraffic   resource.HostingTraffic
	host                 closedForwardingHost
}

// Event is one bounded external observation of Node lifecycle state.
type Event struct {
	Elapsed          time.Duration           `json:"elapsed,omitempty"`
	Hosting          *resource.HostingSample `json:"hosting,omitempty"`
	Schema           string                  `json:"schema"`
	Kind             string                  `json:"kind"`
	State            string                  `json:"state"`
	At               time.Time               `json:"at"`
	Epoch            uint64                  `json:"epoch,omitempty"`
	Generation       string                  `json:"generation,omitempty"`
	Assignment       string                  `json:"assignment,omitempty"`
	CarrierProfile   string                  `json:"carrier_profile,omitempty"`
	AssignmentDigest [32]byte                `json:"assignment_digest,omitempty"`
	Reason           string                  `json:"reason,omitempty"`
	Resource         *resource.Sample        `json:"resource,omitempty"`
}

// Result describes the observed terminal lifecycle outcome. FAILED may report
// cleanup that did not complete or could not be proven inside its bound.
type Result struct {
	State            string
	Epoch            uint64
	Assignment       string
	CarrierProfile   string
	AssignmentDigest [32]byte
	Reason           string
}

type runtimeConfig struct {
	measurementOrigin time.Time
	hostingSample     *resource.HostingSample
	hostingUsage      resource.Sample
	host              closedForwardingHost
	hostingNext       time.Time
	hostingLevel      pressureLevel
	Config
	now      func() time.Time
	probe    *probePlan
	pressure *resource.Guard
}
