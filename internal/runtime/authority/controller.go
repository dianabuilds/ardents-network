package authority

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"time"

	controlprojection "ardents/internal/control/projection"
	appdata "ardents/internal/data"
	dataapi "ardents/internal/data/api"
	dataplacement "ardents/internal/data/placement"
	datareplication "ardents/internal/data/replication"
	datatransfer "ardents/internal/data/transfer"
	"ardents/internal/diagnostics"
	discovery "ardents/internal/discovery"
	discoveryapi "ardents/internal/discovery/api"
	identityapi "ardents/internal/identity/api"
	transport "ardents/internal/network/api"
	networkprivacy "ardents/internal/network/privacy"
	noderoute "ardents/internal/network/route"
	nodelifecycle "ardents/internal/node/lifecycle"
	policyapi "ardents/internal/policy/api"
	publicationapi "ardents/internal/publication/api"
	workloadcontroller "ardents/internal/workload/controller"
	domainworkload "ardents/internal/workload/workload"
)

type dataAuthority interface {
	datatransfer.DataExchange
	Load() error
	SetLocalNodeID(string)
	PublishObject(appdata.Object) (appdata.Object, error)
	GetObject(string) (appdata.Object, bool)
	ListObjects() []appdata.Object
	StoreBlob(appdata.Blob, []byte) (appdata.Blob, error)
	GetBlob(string) (appdata.Blob, bool)
	ListBlobs() []appdata.Blob
	PublishManifest(appdata.Manifest) (appdata.Manifest, error)
	GetManifest(string) (appdata.Manifest, bool)
	ListManifests() []appdata.Manifest
	RetainBlob(string, time.Time) (appdata.Blob, error)
	PinBlob(string) (appdata.Blob, error)
	DropBlob(string) (appdata.Blob, error)
	DataInventory() dataapi.DataInventorySnapshot
	ReserveReplica(dataplacement.ReservationOffer, dataplacement.PeerAuthorization) (dataplacement.ReservationResult, error)
	CommitReplica(dataplacement.CommitRequest, dataplacement.PeerAuthorization) (dataplacement.Commitment, error)
	ObserveReplicaCommitment(dataplacement.Commitment, time.Time) (dataplacement.Commitment, error)
	ReplicaPlacementState() dataplacement.State
	RenewReplicaCommitment(string, time.Time, time.Time) (dataplacement.Commitment, error)
	MarkReplicaCommitment(string, string, time.Time, string) (dataplacement.Commitment, error)
	SetReplicaIntent(appdata.ReplicaIntent) (appdata.ReplicaIntent, error)
	ListReplicaIntents() []appdata.ReplicaIntent
	GetAvailability(string) (appdata.AvailabilitySnapshot, bool)
	ListReplicaRepairs(string) []appdata.RepairRecord
	ReconcileAvailability(string, time.Time) (appdata.AvailabilityReconcileResult, error)
	RecordRepairFailure(string, time.Time, string) (appdata.RepairRecord, error)
	ReplicaCapacity() dataplacement.Capacity
}

type Controller struct {
	cfgName     string
	life        *nodelifecycle.Machine
	diag        *diagnostics.Recorder
	disco       *discovery.Service
	ident       identityapi.Service
	trust       *discovery.TrustEvaluator
	trans       transport.Service
	route       *noderoute.State
	policy      policyapi.Service
	data        dataAuthority
	workload    *workloadcontroller.Service
	publication publicationapi.Service
	privateKey  func() ed25519.PrivateKey
	publish     func(string, map[string]any)
	privateData *datatransfer.PrivateExchange
	replication *datareplication.Service
}

func NewController(
	cfgName string,
	life *nodelifecycle.Machine,
	diag *diagnostics.Recorder,
	disco *discovery.Service,
	ident identityapi.Service,
	trustSvc *discovery.TrustEvaluator,
	trans transport.Service,
	route *noderoute.State,
	policySvc policyapi.Service,
	dataSvc dataAuthority,
	workloadSvc *workloadcontroller.Service,
	publicationMgr publicationapi.Service,
	privateKey func() ed25519.PrivateKey,
	publish func(string, map[string]any),
	dataChannels ...*networkprivacy.Channel,
) *Controller {
	var dataChannel *networkprivacy.Channel
	if len(dataChannels) > 0 {
		dataChannel = dataChannels[0]
	}
	return &Controller{
		cfgName:     cfgName,
		life:        life,
		diag:        diag,
		disco:       disco,
		ident:       ident,
		trust:       trustSvc,
		trans:       trans,
		route:       route,
		policy:      policySvc,
		data:        dataSvc,
		workload:    workloadSvc,
		publication: publicationMgr,
		privateKey:  privateKey,
		publish:     publish,
		privateData: datatransfer.NewPrivateExchange(dataChannel, trans),
	}
}

func (c *Controller) SeedWorkloadsAndReconcileLocked(ctx context.Context, specs []domainworkload.Spec) error {
	if err := c.workload.Seed(specs); err != nil {
		return err
	}
	return c.ReconcileWorkloadsLocked(ctx)
}

func (c *Controller) Data() dataAuthority {
	return c.data
}

func (c *Controller) Workload() *workloadcontroller.Service {
	return c.workload
}

func (c *Controller) LoadData() error {
	return c.data.Load()
}

func (c *Controller) LoadWorkloads() error {
	return c.workload.Load()
}

func (c *Controller) SetLocalDataNodeID(id string) {
	c.data.SetLocalNodeID(id)
	c.replication = datareplication.New(datareplication.Config{
		LocalNodeID: id, Data: c.data, Policy: c.policy, Discovery: c.disco,
		Trust: c.trust, Exchange: c.privateData, Diagnostics: c.diag,
		Identity: c.ident, PrivateKey: c.privateKey,
	})
}

func (c *Controller) requireWorkloadRuntimeMutableLocked(action string) error {
	switch c.life.State() {
	case nodelifecycle.Stopped:
		return fmt.Errorf("%s rejected: node is stopped", action)
	case nodelifecycle.Failed:
		return fmt.Errorf("%s rejected: node is failed", action)
	}
	return nil
}

func (c *Controller) requireAuthoritativeStateMutableLocked(action string) error {
	switch c.life.State() {
	case nodelifecycle.Stopped:
		return fmt.Errorf("%s rejected: node is stopped", action)
	case nodelifecycle.Failed:
		return fmt.Errorf("%s rejected: node is failed", action)
	}
	return nil
}

func (c *Controller) policyDeniedLocked(resource, action string, err error) {
	reason := ""
	if err != nil {
		reason = err.Error()
	}
	c.publish("policy.denied", map[string]any{
		"id":       resource,
		"action":   action,
		"reason":   reason,
		"resource": resource,
	})
}

func (c *Controller) filterRouteCandidatesLocked(resource string, candidates []transport.Candidate) []transport.Candidate {
	if len(candidates) == 0 {
		return nil
	}
	allowed := make([]transport.Candidate, 0, len(candidates))
	for _, candidate := range candidates {
		if err := c.policy.AllowRouteUse(candidate); err != nil {
			c.policyDeniedLocked(resource, "route.use", err)
			continue
		}
		allowed = append(allowed, candidate)
	}
	return allowed
}

func (c *Controller) transportAllowsRoutingLocked() bool {
	return c.trans.State() == "ready"
}

func (c *Controller) unavailableRouteLocked(reason string) discoveryapi.RouteSnapshot {
	return controlprojection.Route(c.route.PreviewUnavailable(reason))
}
