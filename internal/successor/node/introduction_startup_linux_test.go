//go:build linux

package node

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
)

// This refusal test supplies no successful State, History, process or Grant.
func TestIntroductionCanceledOriginalStartupHasNoPrivateEffects(t *testing.T) {
	root := t.TempDir()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	config := IntroductionConfig{Authority: role.Authority{Current: func() (network.RuntimeView, error) {
		t.Error("already-canceled startup observed foreign State")
		return network.RuntimeView{}, errors.New("unavailable State")
	}},
		ParticipationRoot: filepath.Join(root, "participation"), SpendRoot: filepath.Join(root, "spend"), HostingRoot: filepath.Join(root, "hosting"), IntroductionRoot: filepath.Join(root, "introduction")}
	if handle, err := ListenIntroduction(ctx, config); handle != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("already-canceled original startup accepted", handle, err)
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
		t.Fatal("already-canceled startup created private owners", entries, err)
	}
}

func TestIntroductionZeroHandleHasNoOriginalCompletion(t *testing.T) {
	var empty Introduction
	empty.Stop()
	if empty.Done() != nil || empty.Close() == nil || empty.JoinedOutcome() != "failed" {
		t.Fatal("zero handle supplied original lifetime")
	}
	if first, cleanup := empty.JoinedFailureOwners(); first != "absent" || cleanup != "" {
		t.Fatal("zero handle minted diagnostic provenance", first, cleanup)
	}
}
