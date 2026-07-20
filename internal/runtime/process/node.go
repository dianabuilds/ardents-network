package process

import (
	"context"
	"crypto/ed25519"
	"sync"
	"time"

	appdata "ardents/internal/data"
	"ardents/internal/diagnostics"
	diagapi "ardents/internal/diagnostics/api"
	discovery "ardents/internal/discovery"
	discoveryapi "ardents/internal/discovery/api"
	hostingapi "ardents/internal/hosting/api"
	hostingregistry "ardents/internal/hosting/registry"
	identityapi "ardents/internal/identity/api"
	transport "ardents/internal/network/api"
	networkprivacy "ardents/internal/network/privacy"
	noderoute "ardents/internal/network/route"
	nodeapi "ardents/internal/node/api"
	nodelifecycle "ardents/internal/node/lifecycle"
	noderecovery "ardents/internal/node/recovery"
	apppolicy "ardents/internal/policy"
	policyapi "ardents/internal/policy/api"
	publicationapi "ardents/internal/publication/api"
	runtimeauthority "ardents/internal/runtime/authority"
	runtimeorchestration "ardents/internal/runtime/orchestration"
	workloadcontroller "ardents/internal/workload/controller"
)

type querySurface interface {
	SnapshotLocked() nodeapi.Snapshot
	RoutingDetailsLocked() discoveryapi.RouteSnapshot
	DiagnosticsSnapshotLocked() diagapi.DiagSnapshot
	PendingOperationsLocked() []diagapi.OperationSnapshot
	CapabilitiesSnapshotLocked() nodeapi.CapabilitiesSnapshot
	NodeRuntimeSnapshotLocked() nodeapi.NodeRuntimeSnapshot
	NetworkStatusSnapshotLocked() nodeapi.NetworkStatusSnapshot
	DiscoveryStatusSnapshotLocked(time time.Time) nodeapi.DiscoveryStatusSnapshot
	PeerSnapshotsLocked() []nodeapi.PeerSnapshot
	SyncDiagnosticsLocked() error
	ListHostedServicesLocked() ([]hostingapi.HostedServiceSnapshot, error)
}

type Node struct {
	mu          sync.Mutex
	cfg         Config
	life        *nodelifecycle.Machine
	diag        *diagnostics.Recorder
	state       *Store
	keys        identityapi.KeyStore
	boot        *noderecovery.BootStatus
	ident       identityapi.Service
	trust       *discovery.TrustEvaluator
	disco       *discovery.Service
	trans       transport.Service
	privacy     *networkprivacy.Channel
	dataPrivacy *networkprivacy.Channel
	route       *noderoute.State
	policy      policyapi.Service
	policyLive  *apppolicy.Service
	data        *appdata.Service
	srv         *hostingregistry.Registry
	workload    *workloadcontroller.Service
	private     ed25519.PrivateKey
	network     context.Context
	cancel      context.CancelFunc
	refreshStop context.CancelFunc
	seq         int64
	subs        map[chan nodeapi.Event]struct{}

	startBlobExchange func(context.Context) error

	publicationMgr publicationapi.Service
	authorityCtl   *runtimeauthority.Controller
	queryService   querySurface
	commandService *runtimeorchestration.CommandService
	runtimeMgr     *nodelifecycle.Manager
}
