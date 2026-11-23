package issuance

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"
)

// DeliverInventory reads the original public inventory and exports it before
// joining the opened store. A cleanup failure takes precedence in the receipt.
func DeliverInventory(ctx context.Context, root string, binding Binding, path string, initialize bool) (phase string, result error) {
	if !OutputOutsideRoots(path, root) {
		return "input", ErrInvalid
	}
	if err := CheckExportDestination(ctx, path); err != nil {
		return "input", err
	}
	if initialize {
		if err := Initialize(ctx, root, binding); err != nil {
			return "initialize", err
		}
	}
	store, err := Open(ctx, root, binding)
	if err != nil {
		return "open", err
	}
	defer func() {
		if err := store.Close(); err != nil {
			phase = "close"
			result = errors.Join(result, ErrUncertain, err)
		}
	}()
	inventory, err := store.Inventory()
	if err != nil {
		return "inventory", err
	}
	public, err := marshalInventory(inventory)
	if err != nil {
		return "inventory", err
	}
	return "export", Export(ctx, path, public)
}

type inventoryBinding struct {
	Network string `json:"network"`
	Issuer  string `json:"issuer"`
	Signer  string `json:"signer"`
	Start   string `json:"start"`
	End     string `json:"end"`
}
type inventoryEntry struct {
	Window string `json:"window"`
	Class  uint8  `json:"class"`
	SPKI   string `json:"spki"`
}

func marshalInventory(v Inventory) ([]byte, error) {
	b := v.Binding
	entries := make([]inventoryEntry, 0, len(v.Keys))
	for _, key := range v.Keys {
		entries = append(entries, inventoryEntry{time.Unix(int64(key.Window), 0).UTC().Format(time.RFC3339), key.Class, hex.EncodeToString(key.SPKI)})
	}
	return json.Marshal(struct {
		Schema  string           `json:"schema"`
		Binding inventoryBinding `json:"binding"`
		Keys    []inventoryEntry `json:"keys"`
		Digest  string           `json:"digest"`
	}{"ardents-issuer-public-inventory-v1", inventoryBinding{hex.EncodeToString(b.Network[:]), hex.EncodeToString(b.Issuer[:]), hex.EncodeToString(b.Signer[:]), b.Start.Format(time.RFC3339), b.End.Format(time.RFC3339)}, entries, hex.EncodeToString(v.Digest[:])})
}
