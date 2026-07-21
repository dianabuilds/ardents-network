package replication

import (
	"time"

	"ardents/internal/content"
	"ardents/internal/replication/availability"
	"ardents/internal/replication/placement"
	"ardents/internal/storage"
)

type Blob = content.Blob
type BlobSourceRecord = content.BlobSourceRecord
type Manifest = content.Manifest
type Ref = content.Ref
type SourceTrust = content.SourceTrust
type ReplicaIntent = availability.ReplicaIntent
type ContentConfig = content.Config

type repositoryFixture struct {
	*content.Service
	repository *Repository
}

func newInDir(dir string) *repositoryFixture { return newInDirWithConfig(dir, ContentConfig{}) }

func newInDirWithConfig(dir string, config ContentConfig) *repositoryFixture {
	store := content.NewInDirWithConfig(dir, config)
	maxBytes := config.MaxReplicaRetentionBytes
	if maxBytes <= 0 {
		maxBytes = config.MaxRelayRetentionBytes
	}
	return &repositoryFixture{Service: store, repository: NewRepository(RepositoryConfig{
		Path: storage.PathInDir(dir), Content: store, MaxRetentionBytes: maxBytes,
		DefaultDesiredReplicas: config.DefaultDesiredReplicas,
		DefaultMinimumReplicas: config.DefaultMinimumReplicas,
	})}
}

func (s *repositoryFixture) Load() error {
	if err := s.Service.Load(); err != nil {
		return err
	}
	return s.repository.Load()
}

func (s *repositoryFixture) SetLocalNodeID(id string) {
	s.Service.SetLocalNodeID(id)
	s.repository.SetLocalNodeID(id)
}

func (s *repositoryFixture) SetReplicaIntent(intent ReplicaIntent) (ReplicaIntent, error) {
	return s.repository.SetReplicaIntent(intent)
}

func (s *repositoryFixture) ReconcileAvailability(id string, now time.Time) (availability.ReconcileResult, error) {
	return s.repository.ReconcileAvailability(id, now)
}

func (s *repositoryFixture) GetAvailability(id string) (availability.Snapshot, bool) {
	return s.repository.GetAvailability(id)
}

func (s *repositoryFixture) RecordRepairFailure(id string, at time.Time, reason string) (availability.RepairRecord, error) {
	return s.repository.RecordRepairFailure(id, at, reason)
}

func (s *repositoryFixture) ReplicaCapacity() placement.Capacity {
	return s.repository.ReplicaCapacity()
}

func (s *repositoryFixture) ReserveReplica(offer placement.ReservationOffer, auth placement.PeerAuthorization) (placement.ReservationResult, error) {
	return s.repository.ReserveReplica(offer, auth)
}

func (s *repositoryFixture) CommitReplica(request placement.CommitRequest, auth placement.PeerAuthorization) (placement.Commitment, error) {
	return s.repository.CommitReplica(request, auth)
}

func (s *repositoryFixture) ReplicaPlacementState() placement.State {
	return s.repository.ReplicaPlacementState()
}

func (s *repositoryFixture) ObserveReplicaCommitment(commitment placement.Commitment, at time.Time) (placement.Commitment, error) {
	return s.repository.ObserveReplicaCommitment(commitment, at)
}

func (s *repositoryFixture) RenewReplicaCommitment(id string, observedAt, expiresAt time.Time) (placement.Commitment, error) {
	return s.repository.RenewReplicaCommitment(id, observedAt, expiresAt)
}

func (s *repositoryFixture) MarkReplicaCommitment(id, state string, at time.Time, reason string) (placement.Commitment, error) {
	return s.repository.MarkReplicaCommitment(id, state, at, reason)
}

func (s *repositoryFixture) HasCurrentReplicaCommitment(id string) bool {
	return s.repository.HasCurrentReplicaCommitment(id)
}
