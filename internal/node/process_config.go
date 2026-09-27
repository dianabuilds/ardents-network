package node

import (
	"context"
	"crypto/ed25519"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node/probe"
	"github.com/dianabuilds/ardents-network/internal/resource"
)

// RendezvousDedicatedHostResourceProfile is the only selected native Node
// resource profile and is restricted to the dedicated Rendezvous duty.
const RendezvousDedicatedHostResourceProfile = resource.RendezvousDedicatedHostProfile

// Config binds one local identity, authenticated duty facts, and private role-probe listener.
type ProbeConfig = probe.Config

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
