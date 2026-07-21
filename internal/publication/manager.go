package publication

import (
	"context"
	"crypto/ed25519"

	"ardents/internal/diagnostics"
	"ardents/internal/discovery"
	identityapi "ardents/internal/identity"
	networkprivacy "ardents/internal/messaging"
	transport "ardents/internal/network"
	workloadcontroller "ardents/internal/workload/execution"
	hostingregistry "ardents/internal/workload/registry"
)

const Subsystem = "publication"

type Manager struct {
	cfgName    string
	diag       *diagnostics.Recorder
	life       Lifecycle
	disco      *discovery.Service
	policy     Policy
	srv        *hostingregistry.Registry
	workload   *workloadcontroller.Service
	trans      transport.Service
	ident      identityapi.Service
	trust      *discovery.TrustEvaluator
	privateKey func() ed25519.PrivateKey
	publish    func(string, map[string]any)

	publishDiscoveryEntries func(context.Context, []discovery.Entry) error
	networkPublished        bool
	publicationAttempted    bool
}

type Policy interface {
	AllowServicePublication(hostingregistry.ServiceSpec) error
}

type Lifecycle interface {
	State() string
	Move(string) error
}

func NewManager(
	cfgName string,
	diag *diagnostics.Recorder,
	life Lifecycle,
	disco *discovery.Service,
	policySvc Policy,
	srv *hostingregistry.Registry,
	workloadSvc *workloadcontroller.Service,
	trans transport.Service,
	privateCarrier networkprivacy.Carrier,
	ident identityapi.Service,
	trustSvc *discovery.TrustEvaluator,
	privateKey func() ed25519.PrivateKey,
	publish func(string, map[string]any),
	privateChannels ...*networkprivacy.Channel,
) *Manager {
	mgr := &Manager{
		cfgName:    cfgName,
		diag:       diag,
		life:       life,
		disco:      disco,
		policy:     policySvc,
		srv:        srv,
		workload:   workloadSvc,
		trans:      trans,
		ident:      ident,
		trust:      trustSvc,
		privateKey: privateKey,
		publish:    publish,
	}
	mgr.publishDiscoveryEntries = privateDiscoveryPublisher(privateCarrier, privateChannels)
	return mgr
}

func (m *Manager) EffectiveWorkloadStatus(item workloadcontroller.Status) workloadcontroller.Status {
	item.PublishedServices = EffectivePublishedServices(item.PublishedServices, m.policy.AllowServicePublication)
	return item
}

func privateDiscoveryPublisher(carrier networkprivacy.Carrier, channels []*networkprivacy.Channel) func(context.Context, []discovery.Entry) error {
	var channel *networkprivacy.Channel
	if len(channels) > 0 {
		channel = channels[0]
	}
	return func(ctx context.Context, entries []discovery.Entry) error {
		return PublishPrivateDiscoveryEntries(ctx, entries, channel, carrier)
	}
}
