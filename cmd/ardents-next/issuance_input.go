package main

import (
	"encoding/hex"
	"encoding/json"
	"github.com/dianabuilds/ardents-network/internal/admission/issuance"
	"time"
)

type issuanceBindingInput struct {
	Network string `json:"network"`
	Issuer  string `json:"issuer"`
	Signer  string `json:"signer"`
	Start   string `json:"start"`
	End     string `json:"end"`
}
type issuancePlan struct {
	Root          string
	Binding       issuance.Binding
	InventoryFile string
}

func decodeIssuancePlan(raw []byte) (issuancePlan, error) {
	var p issuancePlan
	fields, err := issuerConfigObject(raw, "root", "binding", "inventory_file")
	if err != nil {
		return p, err
	}
	p.Root, err = issuerConfigPath(fields["root"])
	if err != nil {
		return p, err
	}
	p.InventoryFile, err = issuerConfigPath(fields["inventory_file"])
	if err != nil {
		return p, err
	}
	if !issuance.OutputOutsideRoots(p.InventoryFile, p.Root) {
		return p, issuance.ErrInvalid
	}
	binding, err := issuance.DecodeKeyBinding(fields["binding"])
	if err != nil {
		return p, err
	}
	p.Binding = binding
	return p, nil
}

type issuanceInventoryEntry struct {
	Window string `json:"window"`
	Class  uint8  `json:"class"`
	SPKI   string `json:"spki"`
}

func issuanceInventoryJSON(v issuance.Inventory) ([]byte, error) {
	b := v.Binding
	entries := make([]issuanceInventoryEntry, 0, len(v.Keys))
	for _, key := range v.Keys {
		entries = append(entries, issuanceInventoryEntry{time.Unix(int64(key.Window), 0).UTC().Format(time.RFC3339), key.Class, hex.EncodeToString(key.SPKI)})
	}
	return json.Marshal(struct {
		Schema  string                   `json:"schema"`
		Binding issuanceBindingInput     `json:"binding"`
		Keys    []issuanceInventoryEntry `json:"keys"`
		Digest  string                   `json:"digest"`
	}{"ardents-issuer-public-inventory-v1", issuanceBindingInput{hex.EncodeToString(b.Network[:]), hex.EncodeToString(b.Issuer[:]), hex.EncodeToString(b.Signer[:]), b.Start.Format(time.RFC3339), b.End.Format(time.RFC3339)}, entries, hex.EncodeToString(v.Digest[:])})
}
