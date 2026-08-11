package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestSourceModeRejectsIncompleteInvocation(t *testing.T) {
	t.Parallel()
	for _, arguments := range [][]string{nil, {"source"}, {"source", "--config"}, {"role", "--config", "x"}} {
		if err := run(context.Background(), arguments, new(bytes.Buffer)); err == nil || !strings.Contains(err.Error(), "usage:") {
			t.Fatalf("arguments %v returned %v", arguments, err)
		}
	}
}
