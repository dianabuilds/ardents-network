package assembly

import (
	"crypto/ed25519"

	controlprojection "ardents/internal/control/projection"
	appdata "ardents/internal/data"
	"ardents/internal/diagnostics"
	discovery "ardents/internal/discovery"
	hostingregistry "ardents/internal/hosting/registry"
	identityapi "ardents/internal/identity/api"
	transport "ardents/internal/network/api"
	networkprivacy "ardents/internal/network/privacy"
	noderoute "ardents/internal/network/route"
	nodelifecycle "ardents/internal/node/lifecycle"
	noderecovery "ardents/internal/node/recovery"
	policyapi "ardents/internal/policy/api"
	runtimepublication "ardents/internal/publication"
	publicationapi "ardents/internal/publication/api"
	runtimeauthority "ardents/internal/runtime/authority"
	runtimeorchestration "ardents/internal/runtime/orchestration"
	workloadcontroller "ardents/internal/workload/controller"
	domainworkload "ardents/internal/workload/workload"
)

type Config struct {
	NodeName    string
	NodeProfile transport.NodeProfile
	BootSources []string
	Workloads   []domainworkload.Spec
	Life        *nodelifecycle.Machine
	Diag        *diagnostics.Recorder
	State       *noderecovery.Store
	Keys        identityapi.KeyStore
	Boot        *noderecovery.BootStatus
	Identity    identityapi.Service
	Trust       *discovery.TrustEvaluator
	Discovery   *discovery.Service
	Transport   transport.Service
	Privacy     *networkprivacy.Channel
	DataPrivacy *networkprivacy.Channel
	Route       *noderoute.State
	Policy      policyapi.Service
	Data        *appdata.Service
	Hosting     *hostingregistry.Registry
	Workload    *workloadcontroller.Service
	GetPrivate  func() ed25519.PrivateKey
	SetPrivate  func(ed25519.PrivateKey)
	Publish     func(string, map[string]any)
}

type Collaborators struct {
	Publication publicationapi.Service
	Authority   *runtimeauthority.Controller
	Runtime     *nodelifecycle.Manager
	Query       *controlprojection.QueryService
	Command     *runtimeorchestration.CommandService
}

func New(cfg Config) Collaborators {
	publicationMgr := newPublicationManager(cfg)
	authorityCtl := newAuthorityController(cfg, publicationMgr)
	reader := newReader(cfg)
	runtimeMgr := newRuntimeManager(cfg, authorityCtl, publicationMgr)
	return Collaborators{
		Publication: publicationMgr,
		Authority:   authorityCtl,
		Runtime:     runtimeMgr,
		Query:       controlprojection.NewQueryService(runtimeMgr, authorityCtl, reader),
		Command:     runtimeorchestration.NewCommandService(runtimeMgr, cfg.Transport),
	}
}

func newPublicationManager(cfg Config) publicationapi.Service {
	return runtimepublication.NewManager(
		cfg.NodeName,
		cfg.Diag,
		cfg.Life,
		cfg.Discovery,
		cfg.Policy,
		cfg.Hosting,
		cfg.Workload,
		cfg.Transport,
		cfg.Identity,
		cfg.Trust,
		cfg.GetPrivate,
		cfg.Publish,
		cfg.Privacy,
	)
}

func newAuthorityController(cfg Config, publicationMgr publicationapi.Service) *runtimeauthority.Controller {
	return runtimeauthority.NewController(
		cfg.NodeName,
		cfg.Life,
		cfg.Diag,
		cfg.Discovery,
		cfg.Identity,
		cfg.Trust,
		cfg.Transport,
		cfg.Route,
		cfg.Policy,
		cfg.Data,
		cfg.Workload,
		publicationMgr,
		cfg.GetPrivate,
		cfg.Publish,
		cfg.DataPrivacy,
	)
}

func newReader(cfg Config) *controlprojection.Reader {
	return controlprojection.NewReader(
		cfg.NodeName,
		cfg.NodeProfile,
		cfg.Boot,
		cfg.Life,
		cfg.Diag,
		cfg.Identity,
		cfg.Trust,
		cfg.Discovery,
		cfg.Transport,
		cfg.Privacy,
		cfg.DataPrivacy,
		cfg.Route,
		cfg.Policy,
		cfg.Data,
		cfg.Workload,
		cfg.Hosting,
	)
}

func newRuntimeManager(cfg Config, authorityCtl *runtimeauthority.Controller, publicationMgr publicationapi.Service) *nodelifecycle.Manager {
	return nodelifecycle.NewRuntimeCoordinator(
		cfg.NodeName,
		cfg.BootSources,
		cfg.Workloads,
		cfg.Life,
		cfg.Diag,
		cfg.State,
		cfg.Keys,
		cfg.Boot,
		cfg.Identity,
		cfg.Trust,
		cfg.Discovery,
		cfg.Transport,
		authorityCtl,
		publicationMgr,
		cfg.GetPrivate,
		cfg.SetPrivate,
		cfg.Publish,
		cfg.Privacy,
	)
}
