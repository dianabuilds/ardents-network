package node_test

import (
	"bytes"
	"encoding/json"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
	"os/exec"
	"testing"
)

type routeBootstrapPeer struct {
	command    *exec.Cmd
	diagnostic bytes.Buffer
	waited     bool
}

func startRouteBootstrapPeer(t *testing.T, binary, config string) *routeBootstrapPeer {
	t.Helper()
	peer := &routeBootstrapPeer{command: exec.CommandContext(t.Context(), binary, "route", "receive", "--config", config)}
	out, err := peer.command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	peer.command.Stderr = &peer.diagnostic
	processfixture.PrepareInterrupt(peer.command)
	if err := peer.command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if !peer.waited {
			_ = peer.command.Process.Kill()
			_ = peer.command.Wait()
		}
		if t.Failed() && peer.diagnostic.Len() != 0 {
			t.Log("receiver diagnostics", peer.diagnostic.String())
		}
	})
	var event map[string]string
	if err := json.NewDecoder(out).Decode(&event); err != nil || event["operation"] != "route.receive" || event["phase"] != "listening" {
		t.Fatal("receiver did not listen", err, event, peer.diagnostic.String())
	}
	return peer
}
