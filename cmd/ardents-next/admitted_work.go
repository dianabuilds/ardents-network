package main

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/successor/admittedwork"
	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
)

type admittedWorkPlan struct {
	Root     string             `json:"root"`
	Profile  string             `json:"profile"`
	Budget   string             `json:"budget"`
	Receiver receiving.Receiver `json:"receiver"`
	NotAfter time.Time          `json:"not_after"`
	Deadline time.Time          `json:"deadline"`
	Token    []byte             `json:"token"`
	Bytes    uint64             `json:"bytes"`
}

// The local workload is a finite loopback transfer, not a Route or Carrier.
// This operation owns both lifetimes; neither domain imports the other.
func runAdmittedWork(ctx context.Context, args []string, out io.Writer) int {
	if !hosting.Supported() {
		return 1
	}
	var p admittedWorkPlan
	if ctx == nil || admissionConfig(args, &p) != nil || !absoluteAdmissionPath(p.Root) || !absoluteAdmissionPath(p.Profile) || !absoluteAdmissionPath(p.Budget) || p.Bytes == 0 || p.Bytes > 64<<10 {
		return 2
	}
	now := time.Now()
	if !now.Before(p.Deadline) || p.Deadline.After(now.Add(5*time.Second)) {
		return 2
	}
	err := admittedwork.Run(ctx, admittedwork.Plan{Root: p.Root, Budget: p.Budget, Receiver: p.Receiver, NotAfter: p.NotAfter, Deadline: p.Deadline, Token: p.Token, Bytes: p.Bytes, Observe: admissionObserver(p.Profile)})
	result := "completed"
	code := 0
	if err != nil {
		result, code = "refused", 1
	}
	if json.NewEncoder(out).Encode(map[string]any{"outcome": result}) != nil {
		return 2
	}
	return code
}
