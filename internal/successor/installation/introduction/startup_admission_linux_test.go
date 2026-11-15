package introduction

import (
	"context"
	"errors"
	"testing"

	releaseartifact "github.com/dianabuilds/ardents-network/internal/successor/release/infrastructure"
)

func TestIntroductionStartAndStartupRejectMissingOriginalOwners(t *testing.T) {
	for _, ctx := range []context.Context{nil, context.Background()} {
		if err := StartInitial(ctx, nil, releaseartifact.IntroductionAuthorization{}, "/var/lib/intro/binding.json", "/var/lib/distribution", "/var/lib/release"); err != ErrInput {
			t.Fatal("missing original Release accepted", err)
		}
		h, err := OpenStartup(ctx, "/var/lib/intro/binding.json")
		if h != nil || err == nil {
			t.Fatal("uninstalled caller accepted", err)
		}
	}
	for _, h := range []*Startup{nil, {}} {
		if err := h.Check(); !errors.Is(err, ErrInput) {
			t.Fatal("empty Startup Check accepted", err)
		}
		if _, err := h.Inputs(); !errors.Is(err, ErrInput) {
			t.Fatal("empty Startup private input accepted", err)
		}
		h.Stop()
		if err := h.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
