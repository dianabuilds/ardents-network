package authority

import (
	"errors"

	appdata "ardents/internal/data"
	dataapi "ardents/internal/data/api"
)

func (c *Controller) SetReplicaIntentSnapshotLocked(in dataapi.ReplicaIntentSnapshot) (dataapi.ReplicaIntentSnapshot, error) {
	intent, err := c.SetReplicaIntentLocked(appdata.ReplicaIntent{
		ID: in.ID, RootManifestID: in.RootManifestID, Version: in.Version,
		DesiredCopies: in.DesiredCopies, MinimumCopies: in.MinimumCopies,
		LeaseDuration: in.LeaseDuration, RenewalHorizon: in.RenewalHorizon,
		Retention: in.Retention, CreatedAt: in.CreatedAt, UpdatedAt: in.UpdatedAt, ExpiresAt: in.ExpiresAt,
	})
	if err != nil {
		return dataapi.ReplicaIntentSnapshot{}, err
	}
	return replicaIntentSnapshot(intent), nil
}

func (c *Controller) GetDataAvailabilityLocked(rootManifestID string) (dataapi.AvailabilitySnapshot, error) {
	snapshot, ok := c.DataAvailabilityLocked(rootManifestID)
	if !ok {
		return dataapi.AvailabilitySnapshot{}, errors.New("data availability not found")
	}
	return availabilitySnapshot(snapshot), nil
}

func (c *Controller) ListReplicaRepairsLocked(rootManifestID string) []dataapi.RepairSnapshot {
	repairs := c.data.ListReplicaRepairs(rootManifestID)
	out := make([]dataapi.RepairSnapshot, 0, len(repairs))
	for _, repair := range repairs {
		out = append(out, repairSnapshot(repair))
	}
	return out
}

func replicaIntentSnapshot(in appdata.ReplicaIntent) dataapi.ReplicaIntentSnapshot {
	return dataapi.ReplicaIntentSnapshot{
		ID: in.ID, RootManifestID: in.RootManifestID, Version: in.Version,
		DesiredCopies: in.DesiredCopies, MinimumCopies: in.MinimumCopies,
		LeaseDuration: in.LeaseDuration, RenewalHorizon: in.RenewalHorizon,
		Retention: in.Retention, CreatedAt: in.CreatedAt, UpdatedAt: in.UpdatedAt, ExpiresAt: in.ExpiresAt,
	}
}

func availabilitySnapshot(in appdata.AvailabilitySnapshot) dataapi.AvailabilitySnapshot {
	return dataapi.AvailabilitySnapshot{
		RootManifestID: in.RootManifestID, IntentID: in.IntentID, IntentVersion: in.IntentVersion,
		State: in.State, Reason: in.Reason, DesiredCopies: in.DesiredCopies, MinimumCopies: in.MinimumCopies,
		ValidCopies: in.ValidCopies, CurrentLeases: in.CurrentLeases, StaleCopies: in.StaleCopies,
		ExpiredCopies: in.ExpiredCopies, CorruptCopies: in.CorruptCopies, PendingRepairs: in.PendingRepairs,
		ObservedAt: in.ObservedAt, NextLeaseExpiry: in.NextLeaseExpiry,
	}
}

func repairSnapshot(in appdata.RepairRecord) dataapi.RepairSnapshot {
	return dataapi.RepairSnapshot{
		ID: in.ID, IntentID: in.IntentID, IntentVersion: in.IntentVersion, RootManifestID: in.RootManifestID,
		BlobID: in.BlobID, MissingOrdinal: in.MissingOrdinal, State: in.State, Attempts: in.Attempts,
		PostLeaseAttempts: in.PostLeaseAttempts, StartedAt: in.StartedAt, LossEligibleAt: in.LossEligibleAt,
		DeadlineAt: in.DeadlineAt, NextAttemptAt: in.NextAttemptAt, LastAttemptAt: in.LastAttemptAt,
		Reason: in.Reason, FinishedAt: in.FinishedAt,
	}
}
