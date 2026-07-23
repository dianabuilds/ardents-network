// Package daemon owns process composition, startup order, rollback, and shutdown.
// It does not own product state machines or presentation.
package daemon

import (
	appdata "ardents/internal/content"
	"ardents/internal/diagnostics"
	discoveryapi "ardents/internal/discovery"
	"ardents/internal/identity"
	"ardents/internal/identity/principal"
	"ardents/internal/network"
	"ardents/internal/publication"
	"ardents/internal/replication/availability"
	"ardents/internal/workload"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func (n *Node) SetReplicaIntent(intent availability.ReplicaIntent) (availability.ReplicaIntent, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if err := n.requireDataMutableLocked("data set replica intent"); err != nil {
		return availability.ReplicaIntent{}, err
	}
	return n.replica.SetReplicaIntent(intent)
}

func (n *Node) ReconcileDataAvailability(ctx context.Context) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if err := n.requireDataMutableLocked("data reconcile availability"); err != nil {
		return err
	}
	return n.remoteData.Reconcile(ctx)
}

func (n *Node) GetAvailability(rootManifestID string) (availability.Snapshot, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	snapshot, ok := n.replica.GetAvailability(rootManifestID)
	if !ok {
		return availability.Snapshot{}, errors.New("data availability not found")
	}
	return snapshot, nil
}

func (n *Node) ListReplicaRepairs(rootManifestID string) []availability.RepairRecord {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.replica.ListReplicaRepairs(rootManifestID)
}

func (n *Node) ObjectPart() appdata.PartSnapshot {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.data.ObjectPart()
}

func (n *Node) BlobPart() appdata.PartSnapshot {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.data.BlobPart()
}

func (n *Node) SyncDiagnostics(_ context.Context) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.queryService.SyncDiagnosticsLocked()
}

func (n *Node) DiagnosticsRecorder() *diagnostics.Recorder {
	return n.diag
}

func (n *Node) GetNodeRuntime() RuntimeSnapshot {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.queryService.NodeRuntimeSnapshotLocked()
}

func (n *Node) GetNetworkStatus() network.StatusSnapshot {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.queryService.NetworkStatusSnapshotLocked()
}

func (n *Node) GetDiscoveryStatus() discoveryapi.StatusSnapshot {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.queryService.DiscoveryStatusSnapshotLocked(time.Now().UTC())
}

func (n *Node) GetLocalPresence() publication.LocalPresenceSnapshot {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.runtimeMgr.SyncObservedTruthLocked()
	return n.publicationMgr.LocalPresenceSnapshotLocked()
}

func (n *Node) ListPeers() []discoveryapi.PeerSnapshot {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.queryService.PeerSnapshotsLocked()
}

func (n *Node) ListRouteCandidates(query discoveryapi.ListRouteCandidatesQuery) ([]discoveryapi.RouteCandidateSnapshot, discoveryapi.RouteSnapshot, error) {
	switch {
	case query.Service != "":
		result, err := n.ResolveService(query.Service)
		if err != nil {
			return nil, discoveryapi.RouteSnapshot{}, err
		}
		items := make([]discoveryapi.TransportTarget, 0)
		for _, match := range result.Matches {
			items = append(items, match.Candidates...)
		}
		return routeCandidateSnapshots(items), result.Route, nil
	case query.Subject != "" || query.Kind != "":
		result, err := n.ResolveRecord(query.Subject, query.Kind)
		if err != nil {
			return nil, discoveryapi.RouteSnapshot{}, err
		}
		return routeCandidateSnapshots(result.Candidates), result.Route, nil
	default:
		return nil, discoveryapi.RouteSnapshot{}, errors.New("route candidate query requires subject/kind or service")
	}
}

func routeCandidateSnapshots(items []discoveryapi.TransportTarget) []discoveryapi.RouteCandidateSnapshot {
	out := make([]discoveryapi.RouteCandidateSnapshot, 0, len(items))
	for _, item := range items {
		state := "candidate"
		reason := ""
		if !item.Usable {
			state = "degraded"
			reason = "route candidate is not usable"
		}
		out = append(out, discoveryapi.RouteCandidateSnapshot{
			Subject:     item.Subject,
			Service:     item.Service,
			Endpoint:    item.Endpoint,
			Scheme:      item.Scheme,
			Mode:        item.Mode,
			Trusted:     item.Trusted,
			Usable:      item.Usable,
			Cost:        item.Cost,
			Privacy:     item.Privacy,
			Reliability: item.Reliability,
			State:       state,
			Reason:      reason,
		})
	}
	return out
}

func (n *Node) Start(ctx context.Context) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if err := ValidateConfig(n.cfg); err != nil {
		return err
	}
	if n.cancel != nil {
		return nil
	}
	startBlobExchange := func(ctx context.Context) error {
		err := n.remoteData.Start(ctx)
		if n.handleDataPrivacyFailureLocked(err) {
			return nil
		}
		return err
	}
	if n.startBlobExchange != nil {
		startBlobExchange = n.startBlobExchange
	}
	networkCtx, cancel, err := n.runtimeMgr.StartProcessLocked(ctx, startBlobExchange)
	if err != nil {
		return err
	}
	n.network = networkCtx
	n.cancel = cancel
	n.restartDiscoveryRefreshLocked()
	return nil
}

func (n *Node) Stop(ctx context.Context) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.refreshStop != nil {
		n.refreshStop()
		n.refreshStop = nil
	}
	err := n.runtimeMgr.StopProcessLocked(ctx, n.cancel)
	n.cancel = nil
	n.network = nil
	return err
}

func (n *Node) restartDiscoveryRefreshLocked() {
	if n.refreshStop != nil {
		n.refreshStop()
		n.refreshStop = nil
	}
	if n.network == nil {
		return
	}
	refreshCtx, cancel := context.WithCancel(n.network)
	n.refreshStop = cancel
	n.runtimeMgr.StartDiscoveryRefreshLoop(refreshCtx, n.cfg.DiscoveryRefreshInterval, n.refreshDiscoveryPublication)
}

func (n *Node) refreshDiscoveryPublication(ctx context.Context) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.cancel == nil {
		return
	}
	n.runtimeMgr.RefreshDiscoveryPublicationLocked(ctx)
}

const (
	localAPIMaxBodyBytes   int64 = 1 << 20
	localAPIRequestTimeout       = 30 * time.Second
	localAPIReadTimeout          = 15 * time.Second
	localAPIWriteTimeout         = 35 * time.Second
	localAPIIdleTimeout          = 60 * time.Second
	localAPIHeaderTimeout        = 5 * time.Second
	localAPIMaxHeaderBytes       = 16 << 10
)

func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: localAPIHeaderTimeout,
		ReadTimeout:       localAPIReadTimeout,
		WriteTimeout:      localAPIWriteTimeout,
		IdleTimeout:       localAPIIdleTimeout,
		MaxHeaderBytes:    localAPIMaxHeaderBytes,
	}
}

func limitLocalAPIHandler(handler http.Handler, maxBodyBytes int64, timeout time.Duration) http.Handler {
	timed := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/StreamNodeEvents") {
			if err := http.NewResponseController(w).SetWriteDeadline(time.Time{}); err != nil && !errors.Is(err, http.ErrNotSupported) {
				http.Error(w, "streaming is unavailable", http.StatusInternalServerError)
				return
			}
			handler.ServeHTTP(w, r)
			return
		}
		http.TimeoutHandler(handler, timeout, "request timeout").ServeHTTP(w, r)
	})
	limited := http.MaxBytesHandler(timed, maxBodyBytes)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ContentLength > maxBodyBytes {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		limited.ServeHTTP(w, r)
	})
}

func (n *Node) Snapshot() SystemSnapshot {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.runtimeMgr.SyncObservedTruthLocked()
	return n.queryService.SnapshotLocked()
}

func (n *Node) IdentitySnapshot() identity.Snapshot       { return n.Snapshot().Ident }
func (n *Node) TrustSnapshot() discoveryapi.TrustSnapshot { return n.Snapshot().Trust }
func (n *Node) DiscoverySnapshot() discoveryapi.SummarySnapshot {
	return n.Snapshot().Disco
}
func (n *Node) TransportSnapshot() PartSnapshot { return n.Snapshot().Trans }
func (n *Node) RoutingPartSnapshot() PartSnapshot {
	return n.Snapshot().Route
}
func (n *Node) DataSnapshot() (PartSnapshot, PartSnapshot) {
	snap := n.Snapshot()
	return snap.Object, snap.Blob
}
func (n *Node) WorkloadStateSnapshot() workload.StateSnapshot { return n.Snapshot().Workload }

func (n *Node) RoutingDetails() discoveryapi.RouteSnapshot {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.queryService.RoutingDetailsLocked()
}

func (n *Node) Capabilities() CapabilitiesSnapshot {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.queryService.CapabilitiesSnapshotLocked()
}

func (n *Node) PublishObject(object appdata.Object) (appdata.Object, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	owner, err := n.nodeContentOwnerLocked()
	if err != nil {
		return appdata.Object{}, err
	}
	object.Owner = owner
	return n.dataCommands.PublishObject(object)
}

func (n *Node) PublishBlob(command appdata.PublishBlobCommand) (appdata.Blob, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.dataCommands.PublishBlob(command)
}

func (n *Node) PublishBlobForOwner(owner principal.ID, command appdata.PublishBlobCommand) (appdata.Blob, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.dataCommands.PublishBlobForOwner(owner, command)
}

func (n *Node) FetchBlob(ctx context.Context, id string) (appdata.Blob, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if err := n.requireDataMutableLocked("data fetch blob"); err != nil {
		return appdata.Blob{}, err
	}
	blob, err := n.remoteData.FetchBlob(ctx, id)
	if err != nil {
		n.handleDataPrivacyFailureLocked(err)
	}
	return blob, err
}

func (n *Node) FetchChunked(ctx context.Context, rootID string) (appdata.ChunkFetchResult, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if err := n.requireDataMutableLocked("data fetch chunked manifest"); err != nil {
		return appdata.ChunkFetchResult{}, err
	}
	result, err := n.remoteData.FetchChunked(ctx, rootID)
	if err != nil {
		n.handleDataPrivacyFailureLocked(err)
	}
	return result, err
}

func (n *Node) PublishManifest(manifest appdata.Manifest) (appdata.Manifest, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	owner, err := n.nodeContentOwnerLocked()
	if err != nil {
		return appdata.Manifest{}, err
	}
	manifest.Owner = owner
	return n.dataCommands.PublishManifest(manifest)
}

func (n *Node) nodeContentOwnerLocked() (principal.ID, error) {
	owner, err := principal.Parse(n.ident.NodeSummary().Principal)
	if err != nil {
		return principal.ID{}, fmt.Errorf("canonical Node content owner is unavailable")
	}
	return owner, nil
}

func (n *Node) RetainBlob(id string, expiresAt time.Time) (appdata.Blob, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.dataCommands.RetainBlob(id, expiresAt)
}

func (n *Node) PinBlob(id string) (appdata.Blob, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.dataCommands.PinBlob(id)
}

func (n *Node) DropBlob(id string) (appdata.Blob, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.dataCommands.DropBlob(id)
}
