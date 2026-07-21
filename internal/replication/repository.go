package replication

import (
	"fmt"
	"sort"
	"sync"
	"time"

	model "ardents/internal/content/catalog"
	"ardents/internal/replication/availability"
	"ardents/internal/replication/placement"
	"ardents/internal/storage"
)

const defaultMaxReplicaRetentionBytes int64 = 1 << 30

type ContentRepository interface {
	GetBlob(string) (model.Blob, bool)
	GetBlobPayload(string) ([]byte, error)
	ReadTransferManifest(string) (model.Manifest, bool)
	RetainRelayBlob(model.Blob, []byte, time.Time) (model.Blob, error)
	ExtendRelayRetention(string, time.Time) (model.Blob, error)
}

type RepositoryConfig struct {
	Path                   string
	Content                ContentRepository
	MaxRetentionBytes      int64
	DefaultDesiredReplicas int
	DefaultMinimumReplicas int
}

type repositorySnapshot struct {
	Placement    placement.State    `json:"placement"`
	Availability availability.State `json:"availability"`
}

type Repository struct {
	mu           sync.Mutex
	path         string
	content      ContentRepository
	desired      int
	minimum      int
	localNodeID  string
	placement    *placement.Receiver
	availability availability.State
}

func NewRepository(config RepositoryConfig) *Repository {
	maxBytes := config.MaxRetentionBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxReplicaRetentionBytes
	}
	repository := &Repository{
		path: config.Path, content: config.Content,
		desired: config.DefaultDesiredReplicas, minimum: config.DefaultMinimumReplicas,
		availability: availability.NewState(),
	}
	repository.placement = placement.NewReceiver(placement.ReceiverConfig{
		MaxBytes: maxBytes,
		Store: func(blob model.Blob, ciphertext []byte, expiresAt time.Time) error {
			_, err := config.Content.RetainRelayBlob(blob, ciphertext, expiresAt)
			return err
		},
	})
	return repository
}

func (r *Repository) Load() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	var snapshot repositorySnapshot
	found, err := storage.LoadJSON(r.path, "replication", "state", &snapshot)
	if err != nil {
		return err
	}
	if !found {
		var legacy struct {
			Placement    placement.State    `json:"placement"`
			Availability availability.State `json:"availability"`
		}
		legacyFound, legacyErr := storage.LoadJSON(r.path, "data", "snapshot", &legacy)
		if legacyErr != nil {
			return legacyErr
		}
		if legacyFound {
			snapshot = repositorySnapshot{Placement: legacy.Placement, Availability: legacy.Availability}
		}
	}
	r.availability = availability.Normalize(snapshot.Availability)
	if err := r.placement.Restore(snapshot.Placement); err != nil {
		return err
	}
	if !found {
		return r.saveLocked()
	}
	return nil
}

func (r *Repository) SetLocalNodeID(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.localNodeID = id
	r.placement.SetNodeID(id)
}

func (r *Repository) saveLocked() error {
	return storage.SaveJSON(r.path, "replication", "state", repositorySnapshot{
		Placement: r.placement.Snapshot(), Availability: r.availability,
	})
}

func (r *Repository) GetBlob(id string) (model.Blob, bool) { return r.content.GetBlob(id) }

func (r *Repository) GetBlobPayload(id string) ([]byte, error) {
	return r.content.GetBlobPayload(id)
}

func (r *Repository) ReserveReplica(offer placement.ReservationOffer, auth placement.PeerAuthorization) (placement.ReservationResult, error) {
	result, err := r.placement.Reserve(offer, auth)
	if err != nil {
		return placement.ReservationResult{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return result, r.saveLocked()
}

func (r *Repository) CommitReplica(request placement.CommitRequest, auth placement.PeerAuthorization) (placement.Commitment, error) {
	commitment, err := r.placement.Commit(request, auth)
	if err != nil {
		return placement.Commitment{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return commitment, r.saveLocked()
}

func (r *Repository) ReplicaPlacementState() placement.State { return r.placement.Snapshot() }

func (r *Repository) ReplicaCapacity() placement.Capacity { return r.placement.Capacity() }

func (r *Repository) ObserveReplicaCommitment(commitment placement.Commitment, now time.Time) (placement.Commitment, error) {
	observed, err := r.placement.ObserveCommitment(commitment, now)
	if err != nil {
		return placement.Commitment{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return observed, r.saveLocked()
}

func (r *Repository) RenewReplicaCommitment(operationID string, observedAt, expiresAt time.Time) (placement.Commitment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	commitment, ok := r.placement.Snapshot().Commitments[operationID]
	if !ok {
		return placement.Commitment{}, fmt.Errorf("replica commitment not found")
	}
	if _, err := r.content.GetBlobPayload(commitment.BlobID); err != nil {
		return placement.Commitment{}, fmt.Errorf("replica payload is not locally available")
	}
	commitment, err := r.placement.RenewCommitment(operationID, observedAt, expiresAt)
	if err != nil {
		return placement.Commitment{}, err
	}
	if _, err := r.content.ExtendRelayRetention(commitment.BlobID, commitment.LeaseExpiresAt); err != nil {
		return placement.Commitment{}, err
	}
	return commitment, r.saveLocked()
}

func (r *Repository) MarkReplicaCommitment(operationID, state string, observedAt time.Time, _ string) (placement.Commitment, error) {
	marked, err := r.placement.MarkCommitment(operationID, state, observedAt)
	if err != nil {
		return placement.Commitment{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return marked, r.saveLocked()
}

func (r *Repository) HasCurrentReplicaCommitment(blobID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	for _, commitment := range r.placement.Snapshot().Commitments {
		if commitment.BlobID == blobID && commitment.PeerID == r.localNodeID &&
			commitment.State == placement.CommitmentActive && now.Before(commitment.LeaseExpiresAt) {
			if _, err := r.content.GetBlobPayload(blobID); err == nil {
				return true
			}
		}
	}
	return false
}

func (r *Repository) ListReplicaIntents() []availability.ReplicaIntent {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]availability.ReplicaIntent, 0, len(r.availability.Intents))
	for _, intent := range r.availability.Intents {
		out = append(out, intent)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].RootManifestID == out[j].RootManifestID {
			return out[i].Version < out[j].Version
		}
		return out[i].RootManifestID < out[j].RootManifestID
	})
	return out
}
