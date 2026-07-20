package publication

import (
	"context"
	"crypto/ed25519"

	"ardents/internal/diagnostics"
	discovery "ardents/internal/discovery"
	hostingregistry "ardents/internal/hosting/registry"
	identityapi "ardents/internal/identity/api"
	transport "ardents/internal/network/api"
	networkprivacy "ardents/internal/network/privacy"
	nodelifecycle "ardents/internal/node/lifecycle"
	policyapi "ardents/internal/policy/api"
	workloadcontroller "ardents/internal/workload/controller"
)

const Subsystem = "publication"

type Manager struct {
	cfgName    string
	diag       *diagnostics.Recorder
	life       *nodelifecycle.Machine
	disco      *discovery.Service
	policy     policyapi.Service
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

func NewManager(
	cfgName string,
	diag *diagnostics.Recorder,
	life *nodelifecycle.Machine,
	disco *discovery.Service,
	policySvc policyapi.Service,
	srv *hostingregistry.Registry,
	workloadSvc *workloadcontroller.Service,
	trans transport.Service,
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
	mgr.publishDiscoveryEntries = privateDiscoveryPublisher(trans, privateChannels)
	return mgr
}

func privateDiscoveryPublisher(trans transport.Service, channels []*networkprivacy.Channel) func(context.Context, []discovery.Entry) error {
	var channel *networkprivacy.Channel
	if len(channels) > 0 {
		channel = channels[0]
	}
	return func(ctx context.Context, entries []discovery.Entry) error {
		return PublishPrivateDiscoveryEntries(ctx, entries, channel, trans)
	}
}
