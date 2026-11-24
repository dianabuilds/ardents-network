package node_test

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func TestCompiledNodeDispatchRequiresExactOperation(t *testing.T) {
	t.Setenv("ARDENTS_DEBUG_SOCKET", "")
	binary := buildNodeCommand(t)
	for _, args := range [][]string{{"node"}, {"node", "source"}, {"node", "issuer-extra"}, {"node", "inspect-process", "extra"}, {"node", "restart"}} {
		t.Run(strings.Join(args, "/"), func(t *testing.T) {
			command := exec.CommandContext(t.Context(), binary, args...)
			var stdout, stderr bytes.Buffer
			command.Stdout, command.Stderr = &stdout, &stderr
			var exit *exec.ExitError
			if err := command.Run(); !errors.As(err, &exit) || exit.ExitCode() != 2 {
				t.Fatalf("unselected Node operation %v returned %v; stderr %q", args, err, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatal("unselected Node operation projected accepting output", stdout.String())
			}
		})
	}
}
