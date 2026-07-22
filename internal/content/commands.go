package content

import (
	model "ardents/internal/content/catalog"
	"ardents/internal/identity/principal"
	"errors"
	"strings"
	"time"
)

var ErrBlobNotFound = errors.New("blob not found")
var ErrStoreUnavailable = errors.New("content store is unavailable")
var ErrBlobPayloadNotLocal = errors.New("blob payload is not locally available")
var ErrBlobIntegrity = errors.New("blob integrity verification failed")

type CommandGuard func(action string) error
type PinAuthorizer func(Blob) error
type CommandEventSink func(topic string, fields map[string]any)

type CommandConfig struct {
	Guard        CommandGuard
	AuthorizePin PinAuthorizer
	Emit         CommandEventSink
}

type Commands struct {
	store *Service
	cfg   CommandConfig
}

func NewCommands(store *Service, cfg CommandConfig) *Commands {
	return &Commands{store: store, cfg: cfg}
}

func (c *Commands) PublishObject(object Object) (Object, error) {
	if err := c.guard("data publish object"); err != nil {
		return Object{}, err
	}
	published, err := c.store.PublishObject(object)
	if err == nil {
		c.emit("data.object_published", map[string]any{"id": published.ID, "type": published.Type})
	}
	return published, err
}

func (c *Commands) PublishBlob(command PublishBlobCommand) (Blob, error) {
	if err := c.guard("data publish blob"); err != nil {
		return Blob{}, err
	}
	published, err := c.store.StoreBlob(command.Blob, command.Payload)
	if err == nil {
		c.emit("data.blob_published", map[string]any{"id": published.ID, "state": published.State, "encrypted": published.Encrypted})
	}
	return published, err
}

func (c *Commands) PublishBlobForOwner(owner principal.ID, command PublishBlobCommand) (Blob, error) {
	if err := c.guard("data publish blob"); err != nil {
		return Blob{}, err
	}
	published, err := c.store.StoreBlobForOwner(owner, command.Blob, command.Payload)
	if err == nil {
		c.emit("data.blob_published", map[string]any{
			"id": published.ID, "state": published.State, "encrypted": published.Encrypted,
		})
	}
	return published, err
}

func (c *Commands) PublishManifest(manifest Manifest) (Manifest, error) {
	if err := c.guard("data publish manifest"); err != nil {
		return Manifest{}, err
	}
	published, err := c.store.PublishManifest(manifest)
	if err == nil {
		c.emit("data.manifest_published", map[string]any{"id": published.ID, "kind": published.Kind})
	}
	return published, err
}

func (c *Commands) RetainBlob(id string, expiresAt time.Time) (Blob, error) {
	if err := c.guard("data retain blob"); err != nil {
		return Blob{}, err
	}
	item, err := c.store.RetainBlob(id, expiresAt)
	if err != nil && strings.Contains(err.Error(), "policy_") {
		c.denied(id, "data.retain_blob", err)
	}
	if err == nil {
		c.emit("data.blob_retained", map[string]any{"id": item.ID, "state": item.State, "retention": item.Retention})
	}
	return item, err
}

func (c *Commands) PinBlob(id string) (Blob, error) {
	if err := c.guard("data pin blob"); err != nil {
		return Blob{}, err
	}
	blob, ok := c.store.GetBlob(id)
	if !ok {
		return Blob{}, ErrBlobNotFound
	}
	if c.cfg.AuthorizePin != nil {
		if err := c.cfg.AuthorizePin(blob); err != nil {
			c.denied(id, "data.pin_blob", err)
			return Blob{}, err
		}
	}
	item, err := c.store.PinBlob(id)
	if err == nil {
		c.emit("data.blob_pinned", map[string]any{"id": item.ID})
	}
	return item, err
}

func (c *Commands) DropBlob(id string) (Blob, error) {
	if err := c.guard("data drop blob"); err != nil {
		return Blob{}, err
	}
	item, err := c.store.DropBlob(id)
	if err == nil {
		c.emit("data.blob_dropped", map[string]any{"id": item.ID, "state": item.State})
	}
	return item, err
}

func (c *Commands) guard(action string) error {
	if c == nil || c.store == nil {
		return ErrStoreUnavailable
	}
	if c.cfg.Guard != nil {
		return c.cfg.Guard(action)
	}
	return nil
}

func (c *Commands) emit(topic string, fields map[string]any) {
	if c.cfg.Emit != nil {
		c.cfg.Emit(topic, fields)
	}
}

func (c *Commands) denied(resource, action string, err error) {
	reason := ""
	if err != nil {
		reason = err.Error()
	}
	c.emit("policy.denied", map[string]any{"id": resource, "action": action, "reason": reason, "resource": resource})
}

type PartSnapshot struct {
	State  string
	Reason string
}

// InventorySnapshot is the immutable public inventory truth owned by Data.
type InventorySnapshot struct {
	Objects            int
	Manifests          int
	Blobs              int
	LocalBlobs         int
	RemoteBlobs        int
	RetainedTemporary  int
	RelayRetained      int
	Pinned             int
	Expired            int
	Deleted            int
	Encrypted          int
	AvailableForResend int
	LocalBytes         int64
	RelayBytes         int64
}

type StoreSnapshot struct {
	Authority int
	Cached    int
	Derived   int
	Pinned    int
}

func ProjectStore(inventory InventorySnapshot, authoritative bool) StoreSnapshot {
	authority := 0
	if authoritative {
		authority = 1
	}
	return StoreSnapshot{Authority: authority, Cached: inventory.RemoteBlobs, Derived: inventory.Objects + inventory.Manifests, Pinned: inventory.Pinned}
}

func (s *Service) InventorySnapshot() InventorySnapshot {
	in := s.Inventory()
	return InventorySnapshot{
		Objects:            in.Objects,
		Manifests:          in.Manifests,
		Blobs:              in.Blobs,
		LocalBlobs:         in.LocalBlobs,
		RemoteBlobs:        in.RemoteBlobs,
		RetainedTemporary:  in.RetainedTemporary,
		RelayRetained:      in.RelayRetained,
		Pinned:             in.Pinned,
		Expired:            in.Expired,
		Deleted:            in.Deleted,
		Encrypted:          in.Encrypted,
		AvailableForResend: in.AvailableForResend,
		LocalBytes:         in.LocalBytes,
		RelayBytes:         in.RelayBytes,
	}
}

type inventoryPayloadInfo func(id string) (present bool, size int64)

func ProjectInventory(objectsCount, manifestsCount int, blobs map[string]model.Blob, localPayload inventoryPayloadInfo) model.Inventory {
	out := model.Inventory{
		Objects:   objectsCount,
		Manifests: manifestsCount,
		Blobs:     len(blobs),
	}
	for id, blob := range blobs {
		applyBlobInventory(&out, id, blob, localPayload)
	}
	return out
}

func applyBlobInventory(out *model.Inventory, id string, blob model.Blob, localPayload inventoryPayloadInfo) {
	present, size := localPayload(id)
	if blob.Encrypted {
		out.Encrypted++
	}
	applyBlobStateInventory(out, blob, present)
	if !present {
		return
	}
	applyBlobByteInventory(out, blob, size)
}

func applyBlobStateInventory(out *model.Inventory, blob model.Blob, present bool) {
	switch blob.State {
	case "available-local":
		if present {
			out.LocalBlobs++
			out.AvailableForResend++
		}
	case "available-remote":
		out.RemoteBlobs++
	case "retained-temporary":
		if present {
			out.RetainedTemporary++
			out.LocalBlobs++
			out.AvailableForResend++
			if blob.Retention == "relay-temporary" {
				out.RelayRetained++
			}
		}
	case "pinned":
		if present {
			out.Pinned++
			out.LocalBlobs++
			out.AvailableForResend++
		}
	case "expired":
		out.Expired++
	case "deleted":
		out.Deleted++
	}
}

func applyBlobByteInventory(out *model.Inventory, blob model.Blob, size int64) {
	out.LocalBytes += size
	if blob.Retention == "relay-temporary" {
		out.RelayBytes += size
	}
}
