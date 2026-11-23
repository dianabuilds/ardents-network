//go:build linux

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/release/infrastructure/launch"
)

func TestPlatformRetiresDetachedSourceStartBeforeEffects(t *testing.T) {
	var out bytes.Buffer
	if code := runInfrastructure(context.Background(), []string{"start-source", "/absent/distribution", "/absent/trust", "closed", "private", "/absent/binding"}, &out); code != 2 || out.Len() != 0 {
		t.Fatal("retired Source startup entered Installation", code, out.String())
	}
}

type platformLaunch = launch.Declaration

func platformDigest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
