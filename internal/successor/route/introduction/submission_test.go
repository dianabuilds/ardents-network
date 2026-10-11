package introduction

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/successor/route/capsule"
	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
)

// All local owners here can only refuse. No checker supplies successful
// authority, a Grant, registration, capsule authentication or transport ACK.
func TestReceivingSubmissionRejectsBeforeAuthorityAndPhysicalEffects(t *testing.T) {
	for _, fault := range []string{"purpose", "allowance", "deadline", "record", "caller", "connection", "registry", "canceled", "expired", "authority"} {
		t.Run(fault, func(t *testing.T) {
			registry := &Registry{}
			conn := &unadmittedRegistrationConn{}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			currentCalls, recorded := 0, 0
			lost := errors.New("original authority unavailable")
			end := time.Now().Add(time.Minute)
			channel := RegistrationChannel{
				Hello:    ardp.Hello{Purpose: ardp.PurposeSubmission, Deadline: end},
				Bytes:    role.AdmissionWireBytes + 2*ardp.HeaderSize + 4096 + 16384,
				Deadline: end, Record: func(error) { recorded++ },
				Authority: role.Authority{Current: func() (network.RuntimeView, error) { currentCalls++; return network.RuntimeView{}, lost }},
			}
			switch fault {
			case "purpose":
				channel.Hello.Purpose = ardp.PurposeForwarding
			case "allowance":
				channel.Bytes--
			case "deadline":
				channel.Deadline = time.Time{}
			case "record":
				channel.Record = nil
			case "caller":
				ctx = nil
			case "registry":
				registry = nil
			case "canceled":
				cancel()
			case "expired":
				channel.Hello.Deadline = time.Now().Add(-time.Second)
			}
			var err error
			if fault == "connection" {
				err = registry.ServeSubmission(ctx, nil, channel)
			} else {
				err = registry.ServeSubmission(ctx, conn, channel)
			}
			expectedCalls := 0
			if fault == "authority" {
				expectedCalls = 1
				if !errors.Is(err, lost) {
					t.Fatal("lost original authority replaced", err)
				}
			}
			if fault == "canceled" && !errors.Is(err, context.Canceled) {
				t.Fatal("original cancellation replaced", err)
			}
			if fault == "expired" && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatal("original expiry replaced", err)
			}
			if err == nil || currentCalls != expectedCalls || recorded != 0 || conn.reads != 0 || conn.writes != 0 || conn.closes != 0 || conn.interrupts != 0 {
				t.Fatal("unadmitted submission crossed its original boundary", err, currentCalls, recorded, conn)
			}
		})
	}
}

func TestSubmissionMissingSourceCannotInventPeerRefusal(t *testing.T) {
	err := Submit(t.Context(), nil, network.RetainedDuty{}, capsule.Envelope{}, nil)
	var refusal SubmissionRefusal
	if err == nil || errors.As(err, &refusal) {
		t.Fatal("missing Source supplied a remote outcome", err)
	}
}

func TestSubmissionRefusalRetainsDecodedOutcomeThroughWrapping(t *testing.T) {
	// Independent fixed RESULT bytes are codec evidence, not a real peer ACK.
	nonce := [32]byte{9}
	raw := make([]byte, 16384)
	raw[0], raw[32] = 9, 1
	status, err := DecodeResult(raw, nonce)
	if err != nil {
		t.Fatal(err)
	}
	wrapped := fmt.Errorf("original submission: %w", SubmissionRefusal{Status: status})
	var refusal SubmissionRefusal
	if !errors.As(wrapped, &refusal) || refusal.Status != 1 {
		t.Fatal("typed refusal lost", wrapped)
	}
	if errors.Is(wrapped, context.DeadlineExceeded) || errors.Is(wrapped, context.Canceled) {
		t.Fatal("peer refusal became local cancellation", wrapped)
	}
}
