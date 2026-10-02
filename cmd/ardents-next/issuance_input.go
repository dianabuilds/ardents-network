package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/issuance"
	"io"
	"path/filepath"
	"strings"
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
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if uniqueJSON(d) != nil {
		return p, issuance.ErrInvalid
	}
	if _, err := d.Token(); err != io.EOF {
		return p, issuance.ErrInvalid
	}
	if !requiredAdmissionObject(raw, "root", "binding", "inventory_file") {
		return p, issuance.ErrInvalid
	}
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(raw, &fields)
	if json.Unmarshal(fields["root"], &p.Root) != nil || json.Unmarshal(fields["inventory_file"], &p.InventoryFile) != nil ||
		!filepath.IsAbs(p.Root) || filepath.Clean(p.Root) != p.Root || !filepath.IsAbs(p.InventoryFile) || filepath.Clean(p.InventoryFile) != p.InventoryFile {
		return p, issuance.ErrInvalid
	}
	if !issuanceOutputOutsideRoots(p.InventoryFile, p.Root) {
		return p, issuance.ErrInvalid
	}
	binding, err := decodeIssuanceBinding(fields["binding"])
	if err != nil {
		return p, err
	}
	p.Binding = binding
	return p, nil
}

// Reject even not-yet-created state roots before initialization has effects.
func issuanceOutputOutsideRoots(output string, roots ...string) bool {
	for _, root := range roots {
		relative, err := filepath.Rel(root, output)
		if err != nil || relative == "." || relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return false
		}
	}
	return true
}

func decodeIssuanceBinding(raw []byte) (issuance.Binding, error) {
	var p issuancePlan
	if !requiredAdmissionObject(raw, "network", "issuer", "signer", "start", "end") {
		return p.Binding, issuance.ErrInvalid
	}
	var b issuanceBindingInput
	if json.Unmarshal(raw, &b) != nil {
		return p.Binding, issuance.ErrInvalid
	}
	for _, item := range []struct {
		value  string
		target *[32]byte
	}{{b.Network, &p.Binding.Network}, {b.Issuer, &p.Binding.Issuer}, {b.Signer, &p.Binding.Signer}} {
		value, err := admissionHex(item.value, 32)
		if err != nil {
			return p.Binding, issuance.ErrInvalid
		}
		copy(item.target[:], value)
	}
	var err error
	p.Binding.Start, err = admissionTime(b.Start)
	if err != nil {
		return p.Binding, issuance.ErrInvalid
	}
	p.Binding.End, err = admissionTime(b.End)
	if err != nil {
		return p.Binding, issuance.ErrInvalid
	}
	return p.Binding, nil
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
