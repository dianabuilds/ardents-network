package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/issuance"
	"github.com/dianabuilds/ardents-network/internal/successor/nodeidentity"
	"io"
)

type issuanceResult struct {
	Operation string `json:"operation"`
	Phase     string `json:"phase"`
	Outcome   string `json:"outcome"`
}

func issuanceOutcome(err error) string {
	switch {
	case err == nil:
		return "completed"
	case errors.Is(err, issuance.ErrUncertain), errors.Is(err, nodeidentity.ErrUncertain):
		return "storage-uncertain"
	case errors.Is(err, issuance.ErrUnsupported), errors.Is(err, nodeidentity.ErrUnsupported):
		return "unsupported-platform"
	case errors.Is(err, issuance.ErrBusy), errors.Is(err, nodeidentity.ErrBusy):
		return "busy"
	case errors.Is(err, issuance.ErrConflict):
		return "request-conflict"
	case errors.Is(err, issuance.ErrValidity):
		return "outside-validity"
	case errors.Is(err, issuance.ErrCapacity):
		return "result-capacity"
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "canceled"
	case errors.Is(err, issuance.ErrInvalid), errors.Is(err, nodeidentity.ErrInvalid):
		return "invalid-input"
	default:
		return "storage-unavailable"
	}
}
func runIssuance(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	if len(args) > 0 && (args[0] == "initialize-profile" || args[0] == "inspect-profile") {
		return runIssuerProfile(ctx, "issuance."+args[0], args[1:], out, diagnostic)
	}
	if len(args) > 0 && (args[0] == "issue" || args[0] == "initialize-results") {
		return runIssuanceResults(ctx, args, out, diagnostic)
	}
	r := issuanceResult{"issuance", "input", "invalid-input"}
	write := func() int {
		code := 1
		switch r.Outcome {
		case "completed":
			code = 0
		case "invalid-input":
			code = 2
		case "canceled":
			code = 130
		}
		if json.NewEncoder(out).Encode(r) != nil {
			return 2
		}
		return code
	}
	if len(args) != 3 && len(args) != 4 {
		return write()
	}
	if args[1] != "--config" || args[0] != "initialize" && args[0] != "inspect" {
		return write()
	}
	r.Operation = "issuance." + args[0]
	if !issuance.Supported() {
		r.Outcome = "unsupported-platform"
		return write()
	}
	if ctx == nil {
		return write()
	}
	if ctx.Err() != nil {
		r.Outcome = "canceled"
		return write()
	}
	endpoint := ""
	if len(args) == 4 {
		endpoint = args[3]
	}
	if collectorEndpoint(endpoint) != nil {
		return write()
	}
	raw, err := readBounded(args[2], 16<<10)
	if err != nil {
		return write()
	}
	p, err := decodeIssuancePlan(raw)
	clear(raw)
	if err != nil {
		return write()
	}
	o, _ := newObservation(endpoint)
	finish := o.beginIssuance(r.Operation)
	r.Phase = "open"
	if args[0] == "initialize" {
		r.Phase = "initialize"
		err = issuance.Initialize(ctx, p.Root, p.Binding)
	}
	if err == nil {
		r.Phase = "open"
		var store issuance.Store
		store, err = issuance.Open(ctx, p.Root, p.Binding)
		if err == nil {
			r.Phase = "inventory"
			var inventory issuance.Inventory
			inventory, err = store.Inventory()
			if err == nil {
				var public []byte
				public, err = issuanceInventoryJSON(inventory)
				if err == nil {
					r.Phase = "export"
					err = exportIssuanceInventory(ctx, p.InventoryFile, public)
				}
			}
			closeErr := store.Close()
			if closeErr != nil {
				r.Phase = "close"
				err = errors.Join(err, issuance.ErrUncertain, closeErr)
			}
		}
	}
	r.Outcome = issuanceOutcome(err)
	finish(r.Phase, r.Outcome)
	_ = o.close()
	_ = json.NewEncoder(diagnostic).Encode(r)
	return write()
}
