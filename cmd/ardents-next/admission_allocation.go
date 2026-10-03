package main

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/allocation"
)

// Allocation exports an unsigned decision for the external Custody commit and
// signing boundary. It does not persist a shadow allocation history or own keys.
func runAdmissionAllocation(ctx context.Context, args []string, out io.Writer) int {
	var c struct {
		Request   []byte   `json:"request"`
		Journal   []byte   `json:"journal"`
		Network   [32]byte `json:"network"`
		Authority [32]byte `json:"authority"`
	}
	if ctx == nil || admissionConfigBounded(args, &c, (4<<20)+(64<<10)) != nil {
		return 2
	}
	if ctx.Err() != nil {
		return 130
	}
	if allocation.ValidateJournal(c.Journal) != nil {
		return 1
	}
	r, err := allocation.Prepare(c.Request, c.Network, time.Now().UTC())
	if err != nil {
		return 1
	}
	d, err := r.Decide(c.Journal, c.Authority, time.Now().UTC())
	if err != nil {
		return 1
	}
	if ctx.Err() != nil {
		return 130
	}
	if json.NewEncoder(out).Encode(map[string]any{"outcome": "unsigned-decision", "permission": d.Permission(), "journal": d.Journal(), "repeated": d.Repeated()}) != nil {
		return 2
	}
	return 0
}
