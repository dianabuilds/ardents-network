package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/admission/receiving"
)

// This local consumer spends tokens and retains finite allowance receipts. It
// opens no channel and reserves no physical work: a receipt is not a Hosting grant.
func runAdmissionReceiver(ctx context.Context, args []string, input io.ReadCloser, out, diagnostic io.Writer) (code int) {
	var config struct {
		Root     string                `json:"root"`
		Profile  string                `json:"profile"`
		Network  *networkAuthorityPlan `json:"network,omitempty"`
		Receiver receiving.Receiver    `json:"receiver"`
		NotAfter time.Time             `json:"not_after"`
	}
	if ctx == nil || admissionConfig(args, &config) != nil || !absoluteAdmissionPath(config.Root) || !validAdmissionAuthority(config.Profile, config.Network, config.Root) {
		return 2
	}
	if ctx.Err() != nil {
		return 130
	}
	authority, err := openAdmissionAuthority(config.Profile, config.Network)
	if err != nil {
		return 1
	}
	defer func() {
		if authority.close() != nil {
			code = 1
		}
	}()
	o, err := receiving.Open(config.Root, config.Receiver, func() (receiving.Observation, error) {
		return authority.receiver(config.Receiver, config.NotAfter)
	})
	if err != nil {
		return 1
	}
	var grant receiving.Grant
	active := false
	defer func() {
		if errors.Join(grant.Release(), o.Close()) != nil {
			code = 1
		}
		_ = json.NewEncoder(diagnostic).Encode(map[string]any{"operation": "admission.receiver", "phase": "closed", "exit_code": code})
	}()
	return admissionConsole(ctx, input, out, func(ctx context.Context, raw []byte) (any, bool, error) {
		var c struct {
			Operation string          `json:"operation"`
			Token     []byte          `json:"token"`
			Class     admission.Class `json:"class"`
			Deadline  time.Time       `json:"deadline"`
			Remaining uint64          `json:"remaining"`
		}
		if err := decodeAdmissionObject(raw, &c); err != nil {
			return nil, false, err
		}
		var next receiving.Grant
		var err error
		// There is no physical workload in this command; callers cannot request
		// capacity or infer a resource grant from token acceptance.
		reserve := func() (func() error, error) { return nil, nil }
		switch c.Operation {
		case "accept":
			if active {
				return nil, false, errors.New("allowance already retained")
			}
			next, err = o.Accept(ctx, c.Class, c.Token, c.Deadline, reserve)
		case "refill":
			if !active {
				return nil, false, errors.New("allowance absent")
			}
			next, err = o.Refill(ctx, grant, c.Remaining, c.Token, reserve)
		case "release":
			err = grant.Release()
			active = false
		case "close":
			return map[string]string{"outcome": "closed"}, true, nil
		default:
			return nil, false, errors.New("unknown receiver operation")
		}
		if err != nil {
			return nil, false, err
		}
		if c.Operation == "accept" || c.Operation == "refill" {
			if err = grant.Release(); err != nil {
				_ = next.Release()
				return nil, false, err
			}
			grant = next
			active = true
		}
		return map[string]any{"outcome": "completed", "active": active, "deadline": grant.Allowance().Deadline(), "bytes": grant.Allowance().Bytes()}, false, nil
	})
}
