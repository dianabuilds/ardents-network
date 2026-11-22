package main

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"strconv"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission/issuerprofile"
	"github.com/dianabuilds/ardents-network/internal/network/closedprofile"
)

func runClosedProfileInspection(ctx context.Context, args []string, out io.Writer) int {
	if ctx == nil || len(args) != 7 {
		return 2
	}
	if ctx.Err() != nil {
		return 130
	}
	authority, e1 := admissionHex(args[1], 32)
	network, e2 := inspectionID(args[2])
	generation, e3 := inspectionID(args[3])
	digest, e4 := inspectionID(args[4])
	number, e5 := strconv.ParseUint(args[5], 10, 64)
	at, e6 := time.Parse(time.RFC3339, args[6])
	if e1 != nil || e2 != nil || e3 != nil || e4 != nil || e5 != nil || e6 != nil || strconv.FormatUint(number, 10) != args[5] {
		return 2
	}
	raw, err := readBounded(args[0], closedprofile.MaxSize)
	if err != nil {
		return 1
	}
	view, err := closedprofile.Verify(raw, closedprofile.Context{
		NetworkID: network, StateGeneration: generation, EpochDigest: digest,
		Epoch: number, Authority: authority, Now: at.UTC(),
	})
	if err != nil {
		return 1
	}
	if ctx.Err() != nil {
		return 130
	}
	if err := json.NewEncoder(out).Encode(struct {
		Schema    string `json:"schema"`
		Digest    string `json:"digest"`
		Authority string `json:"state_authority_key"`
		Epoch     uint64 `json:"epoch"`
		NotBefore string `json:"not_before"`
		NotAfter  string `json:"not_after"`
	}{"ardents-closed-profile-inspection-v1", hex.EncodeToString(view.Digest[:]), hex.EncodeToString(view.AuthorityKey[:]),
		view.Epoch, view.NotBefore.Format(time.RFC3339), view.NotAfter.Format(time.RFC3339)}); err != nil {
		return 1
	}
	return 0
}

func runIssuerPublicInspection(ctx context.Context, args []string, out io.Writer) int {
	if ctx == nil || len(args) != 4 {
		return 2
	}
	if ctx.Err() != nil {
		return 130
	}
	key, e1 := inspectionID(args[1])
	network, e2 := inspectionID(args[2])
	issuer, e3 := inspectionID(args[3])
	if e1 != nil || e2 != nil || e3 != nil {
		return 2
	}
	raw, err := readBounded(args[0], issuerprofile.MaximumSize)
	if err != nil {
		return 1
	}
	view, err := issuerprofile.Inspect(raw, issuerprofile.PublicPins{Network: network, Issuer: issuer, Signer: key})
	if err != nil {
		return 1
	}
	binding, inventory, ok := view.Snapshot()
	if !ok {
		return 1
	}
	type token struct {
		WindowStart string `json:"window_start"`
		Class       uint8  `json:"class"`
		SPKI        string `json:"spki"`
	}
	tokens := make([]token, 0, len(inventory))
	for _, item := range inventory {
		tokens = append(tokens, token{time.Unix(int64(item.Window), 0).UTC().Format(time.RFC3339), item.Class, base64.RawStdEncoding.EncodeToString(item.SPKI)})
	}
	if ctx.Err() != nil {
		return 130
	}
	if err := json.NewEncoder(out).Encode(struct {
		Schema    string  `json:"schema"`
		Network   string  `json:"network_id"`
		Issuer    string  `json:"issuer_node_id"`
		NotBefore string  `json:"not_before"`
		NotAfter  string  `json:"not_after"`
		TokenKeys []token `json:"token_keys"`
	}{"ardents-closed-issuer-inspection-v1", args[2], args[3], binding.Start.Format(time.RFC3339), binding.End.Format(time.RFC3339), tokens}); err != nil {
		return 1
	}
	return 0
}

func inspectionID(value string) ([32]byte, error) {
	var id [32]byte
	raw, err := admissionHex(value, len(id))
	if err != nil {
		return id, err
	}
	copy(id[:], raw)
	return id, nil
}
