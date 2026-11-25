package main

import "github.com/dianabuilds/ardents-network/internal/admission/quota"

import (
	"bytes"
	"context"
	"testing"
)

func TestAdmissionUnsupportedOrCanceledBeforeFiles(t *testing.T) {
	var out, log bytes.Buffer
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	code := run(ctx, []string{"admission", "initialize", "--config", "nonexistent"}, &out, &log)
	if quota.OperatorAvailability() == "" {
		if code != 130 || !bytes.Contains(out.Bytes(), []byte("canceled")) {
			t.Fatal(code, &out)
		}
	} else {
		if code != 1 || !bytes.Contains(out.Bytes(), []byte("unsupported-platform")) {
			t.Fatal(code, &out)
		}
	}
}
