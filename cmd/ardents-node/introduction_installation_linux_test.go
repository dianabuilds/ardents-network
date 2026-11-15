package main

import (
	"context"
	"io"
	"testing"
)

func TestIntroductionStartupRejectsMissingOriginalInputs(t *testing.T) {
	for _, args := range [][]string{{"start-initial"}, {"start-installed"}, {"start-initial", "one", "two"}, {"restart", "one"}, {"start-installed", "unaccepted"}} {
		if err := runIntroductionInstallation(context.Background(), args, io.Discard); err == nil {
			t.Fatal("invalid original startup accepted", args)
		}
	}
}
