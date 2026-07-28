package daemon

import (
	"context"
	"fmt"
	"sync"

	appdata "ardents/internal/content"
	identityprincipal "ardents/internal/identity/principal"
	"ardents/internal/replication"
	"ardents/internal/replication/placement"
	"ardents/internal/transfer"
)

// remoteContent composes the transfer and replication owners at the process
// boundary. Content remains the persistence owner and does not start network
// protocols or construct sibling services.
type remoteContent struct {
	mu                sync.RWMutex
	content           *appdata.Service
	transfer          transfer.ExchangeConfig
	replicationConfig replication.Config
	replication       *replication.Service
	private           *transfer.PrivateExchange
}

func newRemoteContent(cfg ownerAssemblyConfig) *remoteContent {
	identity := func() transfer.IdentitySummary {
		summary := cfg.Identity.NodeSummary()
		return transfer.IdentitySummary{Principal: summary.Principal, PublicKey: summary.PublicKey}
	}
	recordEvent := func(domain, eventType, resource, message, reasonCode string, payload map[string]any) {
		cfg.Diag.RecordEvent(domain, eventType, resource, message, reasonCode, payload)
	}
	exchange := transfer.NewPrivateExchange(cfg.DataPrivacy, messagingCarrier{cfg.Transport})
	transferConfig := transfer.ExchangeConfig{
		ConfigName: cfg.NodeName, RecordEvent: recordEvent, Discovery: cfg.Discovery,
		Identity: identity, Trust: cfg.Trust, Policy: cfg.Policy, Data: cfg.Data,
		History: cfg.Transfer, Replicas: cfg.Replica,
		PrivateKey: cfg.GetPrivate, Private: exchange, Publish: cfg.Publish,
		PolicyDenied: func(resource, action string, err error) {
			if cfg.Publish == nil {
				return
			}
			reason := ""
			if err != nil {
				reason = err.Error()
			}
			cfg.Publish("policy.denied", map[string]any{
				"id": resource, "action": action, "reason": reason, "resource": resource,
			})
		},
	}
	return &remoteContent{
		content:  cfg.Data,
		transfer: transferConfig,
		private:  exchange,
		replicationConfig: replication.Config{
			Data: cfg.Replica, Policy: cfg.Policy, Discovery: cfg.Discovery, Trust: cfg.Trust,
			Exchange: exchange, RecordEvent: recordEvent, Identity: identity, PrivateKey: cfg.GetPrivate,
		},
	}
}

func (r *remoteContent) SetLocalNodePrincipal(principal identityprincipal.ID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	config := r.replicationConfig
	config.LocalNodePrincipal = principal
	r.replicationConfig = config
	if principal.String() == "" {
		r.replication = nil
		return
	}
	r.replication = replication.New(config)
}

func (r *remoteContent) Start(ctx context.Context) error {
	if err := transfer.StartBlobExchange(ctx, r.transfer); err != nil {
		return err
	}
	service, err := r.replicationService()
	if err != nil {
		return err
	}
	return service.Start(ctx)
}

func (r *remoteContent) FetchBlob(ctx context.Context, id string) (appdata.Blob, error) {
	return transfer.FetchBlob(ctx, r.transfer, id)
}

func (r *remoteContent) FetchChunked(ctx context.Context, owner identityprincipal.ID, rootID string) (appdata.ChunkFetchResult, error) {
	result, err := transfer.FetchChunked(ctx, r.transfer, rootID, transfer.ChunkFetchOptions{Owner: owner})
	if err != nil {
		return appdata.ChunkFetchResult{}, err
	}
	root, ok := r.content.GetManifest(owner, rootID)
	if !ok {
		return appdata.ChunkFetchResult{}, fmt.Errorf("fetched root manifest is unavailable")
	}
	return appdata.ChunkFetchResult{
		Root: root, ChunkCount: result.ChunkCount, FetchedCount: result.FetchedCount,
		ResumedCount: result.ResumedCount, TotalBytes: result.TotalBytes,
	}, nil
}

func (r *remoteContent) PlaceBlob(ctx context.Context, blobID, target string, intentVersion uint64) (replication.ReplicaCommitment, error) {
	service, err := r.replicationService()
	if err != nil {
		return replication.ReplicaCommitment{}, err
	}
	targetPrincipal, err := identityprincipal.Parse(target)
	if err != nil {
		return replication.ReplicaCommitment{}, fmt.Errorf("replica target Principal is invalid")
	}
	commitment, err := service.PlaceBlob(ctx, blobID, targetPrincipal, intentVersion)
	return replicaCommitmentSnapshot(commitment), err
}

func (r *remoteContent) PlaceAvailable(ctx context.Context, blobID string, count int, intentVersion uint64) (replication.ReplicaPlacementOutcome, error) {
	service, err := r.replicationService()
	if err != nil {
		return replication.ReplicaPlacementOutcome{}, err
	}
	outcome, err := service.PlaceAvailable(ctx, blobID, count, intentVersion)
	return replicaPlacementOutcomeSnapshot(outcome), err
}

func (r *remoteContent) ProbeReplica(ctx context.Context, commitment replication.ReplicaCommitment) (replication.ReplicaCommitment, error) {
	service, err := r.replicationService()
	if err != nil {
		return replication.ReplicaCommitment{}, err
	}
	observed, err := service.ProbeReplica(ctx, replicaCommitmentModel(commitment))
	return replicaCommitmentSnapshot(observed), err
}

func (r *remoteContent) Reconcile(ctx context.Context) error {
	service, err := r.replicationService()
	if err != nil {
		return err
	}
	return service.ReconcileOnce(ctx)
}

func (r *remoteContent) replicationService() (*replication.Service, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.replication == nil {
		return nil, fmt.Errorf("data replica control is not configured")
	}
	return r.replication, nil
}

func replicaCommitmentModel(in replication.ReplicaCommitment) placement.Commitment {
	return placement.Commitment{
		OperationID: in.OperationID, IntentVersion: in.IntentVersion,
		ContentReference: in.ContentReference, TargetNode: in.TargetNode, Size: in.Size,
		State: in.State, HealthReason: in.HealthReason,
		LeaseStartsAt: in.LeaseStartsAt, LastObservedAt: in.LastObservedAt,
		LeaseExpiresAt: in.LeaseExpiresAt,
	}
}

func replicaCommitmentSnapshot(in placement.Commitment) replication.ReplicaCommitment {
	return replication.ReplicaCommitment{
		OperationID: in.OperationID, IntentVersion: in.IntentVersion,
		ContentReference: in.ContentReference, TargetNode: in.TargetNode, Size: in.Size,
		State: in.State, HealthReason: in.HealthReason,
		LeaseStartsAt: in.LeaseStartsAt, LastObservedAt: in.LastObservedAt,
		LeaseExpiresAt: in.LeaseExpiresAt,
	}
}

func replicaPlacementOutcomeSnapshot(in replication.PlacementOutcome) replication.ReplicaPlacementOutcome {
	out := replication.ReplicaPlacementOutcome{
		Decision: replication.ReplicaPlacementDecision{
			Selected: make([]identityprincipal.ID, 0, len(in.Decision.Selected)),
			Denials:  make([]replication.ReplicaPlacementDenial, 0, len(in.Decision.Denials)),
		},
		Commitments: make([]replication.ReplicaCommitment, 0, len(in.Commitments)),
	}
	for _, selected := range in.Decision.Selected {
		out.Decision.Selected = append(out.Decision.Selected, selected.NodePrincipal)
	}
	for _, denial := range in.Decision.Denials {
		out.Decision.Denials = append(out.Decision.Denials, replication.ReplicaPlacementDenial{NodePrincipal: denial.NodePrincipal, Reason: denial.Reason})
	}
	for _, commitment := range in.Commitments {
		out.Commitments = append(out.Commitments, replicaCommitmentSnapshot(commitment))
	}
	return out
}

func replicaPlacementSnapshot(state placement.State) replication.ReplicaPlacementSnapshot {
	snapshot := replication.ReplicaPlacementSnapshot{
		Reserved: state.Reserved, Used: state.Used,
		Commitments: make(map[string]replication.ReplicaCommitment, len(state.Commitments)),
	}
	for id, commitment := range state.Commitments {
		snapshot.Commitments[id] = replicaCommitmentSnapshot(commitment)
	}
	return snapshot
}

type contentLifecycle struct {
	content  *appdata.Service
	replica  *replication.Repository
	transfer *transfer.Journal
	remote   *remoteContent
}

func (c contentLifecycle) Load() error {
	if err := c.content.Load(); err != nil {
		return err
	}
	if err := c.transfer.Load(); err != nil {
		return err
	}
	return c.replica.Load()
}

func (c contentLifecycle) SetLocalNodeID(id string) error {
	principal, err := identityprincipal.Parse(id)
	if err != nil {
		return fmt.Errorf("local Node Principal is invalid")
	}
	c.content.SetLocalNodeID(id)
	c.replica.SetLocalNodePrincipal(principal)
	c.remote.SetLocalNodePrincipal(principal)
	return nil
}
