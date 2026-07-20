package authority

import (
	"context"
	"errors"
	"strings"
	"time"

	controlprojection "ardents/internal/control/projection"
	appdata "ardents/internal/data"
	dataapi "ardents/internal/data/api"
	dataplacement "ardents/internal/data/placement"
	datareplication "ardents/internal/data/replication"
	datatransfer "ardents/internal/data/transfer"
)

func (c *Controller) PublishObjectLocked(object dataapi.ObjectSnapshot) (dataapi.ObjectSnapshot, error) {
	if err := c.requireAuthoritativeStateMutableLocked("data publish object"); err != nil {
		return dataapi.ObjectSnapshot{}, err
	}
	published, err := c.data.PublishObject(appdata.Object{
		ID:       object.ID,
		Type:     object.Type,
		Owner:    object.Owner,
		Body:     controlprojection.CloneMap(object.Body),
		BlobRefs: refsFromSnapshots(object.BlobRefs),
	})
	if err != nil {
		return dataapi.ObjectSnapshot{}, err
	}
	c.publish("data.object_published", map[string]any{"id": published.ID, "type": published.Type})
	return objectSnapshot(published), nil
}

func (c *Controller) GetObjectLocked(id string) (dataapi.ObjectSnapshot, error) {
	item, ok := c.data.GetObject(id)
	if !ok {
		return dataapi.ObjectSnapshot{}, errors.New("object not found")
	}
	return objectSnapshot(item), nil
}

func (c *Controller) ListObjectsLocked() ([]dataapi.ObjectSnapshot, error) {
	items := c.data.ListObjects()
	out := make([]dataapi.ObjectSnapshot, 0, len(items))
	for _, item := range items {
		out = append(out, objectSnapshot(item))
	}
	return out, nil
}

func (c *Controller) PublishBlobLocked(blob dataapi.BlobSnapshot) (dataapi.BlobSnapshot, error) {
	if err := c.requireAuthoritativeStateMutableLocked("data publish blob"); err != nil {
		return dataapi.BlobSnapshot{}, err
	}
	published, err := c.data.StoreBlob(appdata.Blob{
		ID:        blob.ID,
		CID:       blob.CID,
		MediaType: blob.MediaType,
		Size:      blob.Size,
		Hash:      blob.Hash,
		Cipher:    blob.Cipher,
		KeyID:     blob.KeyID,
		State:     blob.State,
		Retention: blob.Retention,
		Encrypted: blob.Encrypted,
		ExpiresAt: blob.ExpiresAt,
	}, blob.Payload)
	if err != nil {
		return dataapi.BlobSnapshot{}, err
	}
	c.publish("data.blob_published", map[string]any{"id": published.ID, "state": published.State, "encrypted": published.Encrypted})
	return blobSnapshot(published), nil
}

func (c *Controller) GetBlobLocked(id string) (dataapi.BlobSnapshot, error) {
	item, ok := c.data.GetBlob(id)
	if !ok {
		return dataapi.BlobSnapshot{}, errors.New("blob not found")
	}
	return blobSnapshot(item), nil
}

func (c *Controller) ListBlobsLocked() ([]dataapi.BlobSnapshot, error) {
	items := c.data.ListBlobs()
	out := make([]dataapi.BlobSnapshot, 0, len(items))
	for _, item := range items {
		out = append(out, blobSnapshot(item))
	}
	return out, nil
}

func (c *Controller) PublishManifestLocked(manifest dataapi.ManifestSnapshot) (dataapi.ManifestSnapshot, error) {
	if err := c.requireAuthoritativeStateMutableLocked("data publish manifest"); err != nil {
		return dataapi.ManifestSnapshot{}, err
	}
	published, err := c.data.PublishManifest(appdata.Manifest{
		ID:        manifest.ID,
		Kind:      manifest.Kind,
		Owner:     manifest.Owner,
		Refs:      refsFromSnapshots(manifest.Refs),
		Access:    manifest.Access,
		Retention: manifest.Retention,
		Encrypted: manifest.Encrypted,
		Metadata:  controlprojection.CloneMap(manifest.Metadata),
	})
	if err != nil {
		return dataapi.ManifestSnapshot{}, err
	}
	c.publish("data.manifest_published", map[string]any{"id": published.ID, "kind": published.Kind})
	return manifestSnapshot(published), nil
}

func (c *Controller) GetManifestLocked(id string) (dataapi.ManifestSnapshot, error) {
	item, ok := c.data.GetManifest(id)
	if !ok {
		return dataapi.ManifestSnapshot{}, errors.New("manifest not found")
	}
	return manifestSnapshot(item), nil
}

func (c *Controller) ListManifestsLocked() ([]dataapi.ManifestSnapshot, error) {
	items := c.data.ListManifests()
	out := make([]dataapi.ManifestSnapshot, 0, len(items))
	for _, item := range items {
		out = append(out, manifestSnapshot(item))
	}
	return out, nil
}

func (c *Controller) RetainBlobLocked(id string, expiresAt time.Time) (dataapi.BlobSnapshot, error) {
	if err := c.requireAuthoritativeStateMutableLocked("data retain blob"); err != nil {
		return dataapi.BlobSnapshot{}, err
	}
	item, err := c.data.RetainBlob(id, expiresAt)
	if err != nil {
		if strings.Contains(err.Error(), "policy_") {
			c.policyDeniedLocked(id, "data.retain_blob", err)
		}
		return dataapi.BlobSnapshot{}, err
	}
	c.publish("data.blob_retained", map[string]any{"id": item.ID, "state": item.State, "retention": item.Retention})
	return blobSnapshot(item), nil
}

func (c *Controller) PinBlobLocked(id string) (dataapi.BlobSnapshot, error) {
	if err := c.requireAuthoritativeStateMutableLocked("data pin blob"); err != nil {
		return dataapi.BlobSnapshot{}, err
	}
	blob, ok := c.data.GetBlob(id)
	if !ok {
		return dataapi.BlobSnapshot{}, errors.New("blob not found")
	}
	if err := c.policy.AllowBlobPin(blobSnapshot(blob)); err != nil {
		c.policyDeniedLocked(id, "data.pin_blob", err)
		return dataapi.BlobSnapshot{}, err
	}
	item, err := c.data.PinBlob(id)
	if err != nil {
		return dataapi.BlobSnapshot{}, err
	}
	c.publish("data.blob_pinned", map[string]any{"id": item.ID})
	return blobSnapshot(item), nil
}

func (c *Controller) DropBlobLocked(id string) (dataapi.BlobSnapshot, error) {
	if err := c.requireAuthoritativeStateMutableLocked("data drop blob"); err != nil {
		return dataapi.BlobSnapshot{}, err
	}
	item, err := c.data.DropBlob(id)
	if err != nil {
		return dataapi.BlobSnapshot{}, err
	}
	c.publish("data.blob_dropped", map[string]any{"id": item.ID, "state": item.State})
	return blobSnapshot(item), nil
}

func (c *Controller) DataInventoryLocked() dataapi.DataInventorySnapshot {
	return c.data.DataInventory()
}

func (c *Controller) StartBlobExchangeLocked(ctx context.Context) error {
	err := datatransfer.StartBlobExchange(ctx, c.dataTransferConfig())
	if c.handleDataPrivacyFailureLocked(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if c.replication == nil {
		return errors.New("data replica control is not configured")
	}
	return c.replication.Start(ctx)
}

func (c *Controller) PlaceBlobReplicaLocked(ctx context.Context, blobID, target string, intentVersion uint64) (dataplacement.Commitment, error) {
	if err := c.requireAuthoritativeStateMutableLocked("data place replica"); err != nil {
		return dataplacement.Commitment{}, err
	}
	if c.replication == nil {
		return dataplacement.Commitment{}, errors.New("data replica control is not configured")
	}
	return c.replication.PlaceBlob(ctx, blobID, target, intentVersion)
}

func (c *Controller) PlaceAvailableBlobReplicasLocked(ctx context.Context, blobID string, count int, intentVersion uint64) (datareplication.PlacementOutcome, error) {
	if err := c.requireAuthoritativeStateMutableLocked("data place available replicas"); err != nil {
		return datareplication.PlacementOutcome{}, err
	}
	if c.replication == nil {
		return datareplication.PlacementOutcome{}, errors.New("data replica control is not configured")
	}
	return c.replication.PlaceAvailable(ctx, blobID, count, intentVersion)
}

func (c *Controller) ProbeBlobReplicaLocked(ctx context.Context, commitment dataplacement.Commitment) (dataplacement.Commitment, error) {
	if err := c.requireAuthoritativeStateMutableLocked("data probe replica"); err != nil {
		return dataplacement.Commitment{}, err
	}
	if c.replication == nil {
		return dataplacement.Commitment{}, errors.New("data replica control is not configured")
	}
	return c.replication.ProbeReplica(ctx, commitment)
}

func (c *Controller) SetReplicaIntentLocked(intent appdata.ReplicaIntent) (appdata.ReplicaIntent, error) {
	if err := c.requireAuthoritativeStateMutableLocked("data set replica intent"); err != nil {
		return appdata.ReplicaIntent{}, err
	}
	return c.data.SetReplicaIntent(intent)
}

func (c *Controller) DataAvailabilityLocked(rootManifestID string) (appdata.AvailabilitySnapshot, bool) {
	return c.data.GetAvailability(rootManifestID)
}

func (c *Controller) ReconcileDataAvailabilityLocked(ctx context.Context) error {
	if err := c.requireAuthoritativeStateMutableLocked("data reconcile availability"); err != nil {
		return err
	}
	if c.replication == nil {
		return errors.New("data replica control is not configured")
	}
	return c.replication.ReconcileOnce(ctx)
}

func (c *Controller) FetchBlobLocked(ctx context.Context, id string) (dataapi.BlobSnapshot, error) {
	if err := c.requireAuthoritativeStateMutableLocked("data fetch blob"); err != nil {
		return dataapi.BlobSnapshot{}, err
	}
	blob, err := datatransfer.FetchBlob(ctx, c.dataTransferConfig(), id)
	if err != nil {
		c.handleDataPrivacyFailureLocked(err)
		return dataapi.BlobSnapshot{}, err
	}
	return blobSnapshot(blob), nil
}

func (c *Controller) FetchChunkedLocked(ctx context.Context, rootID string) (dataapi.ChunkFetchSnapshot, error) {
	if err := c.requireAuthoritativeStateMutableLocked("data fetch chunked manifest"); err != nil {
		return dataapi.ChunkFetchSnapshot{}, err
	}
	result, err := datatransfer.FetchChunked(ctx, c.dataTransferConfig(), rootID, datatransfer.ChunkFetchOptions{})
	if err != nil {
		c.handleDataPrivacyFailureLocked(err)
		return dataapi.ChunkFetchSnapshot{}, err
	}
	return dataapi.ChunkFetchSnapshot{
		Root: manifestSnapshot(result.Root), ChunkCount: result.ChunkCount,
		FetchedCount: result.FetchedCount, ResumedCount: result.ResumedCount, TotalBytes: result.TotalBytes,
	}, nil
}

func (c *Controller) dataTransferConfig() datatransfer.ExchangeConfig {
	return datatransfer.ExchangeConfig{
		ConfigName:  c.cfgName,
		Diagnostics: c.diag,
		Discovery:   c.disco,
		Identity:    c.ident,
		Trust:       c.trust,
		Transport:   c.trans,
		Policy:      c.policy,
		Data:        c.data,
		PrivateKey:  c.privateKey,
		Private:     c.privateData,
		Publish:     c.publish,
		PolicyDenied: func(resource, action string, err error) {
			c.policyDeniedLocked(resource, action, err)
		},
	}
}
