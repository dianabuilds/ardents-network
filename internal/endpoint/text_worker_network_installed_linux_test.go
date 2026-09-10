//go:build linux && text_worker_installed

package endpoint

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/interfacev1/administration"
	applicationconnection "github.com/dianabuilds/ardents-network/internal/application/interfacev2/connection"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/service/instance"
)

// Uses the installed launch verifier and actual confined worker processes.
// State/authority provisioning is the network fixture, not a command ceremony.
func TestInstalledTextWorkersReadTargetThroughJoinedNetwork(t *testing.T) {
	if err := verifyTextEndpointService(t.Context()); err != nil {
		t.Fatalf("invalid installed environment: %v", err)
	}
	for _, carrier := range []route.CarrierProfile{route.ClosedCarrierTCP, route.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			for _, size := range []struct {
				name  string
				bytes int
			}{{"empty", 0}, {"reference", 64 << 10}, {"maximum", 4 << 20}, {"refresh", 64 << 10}} {
				t.Run(size.name, func(t *testing.T) {
					exchangeInstalledTextAdministration(t, carrier, bytes.Repeat([]byte("x"), size.bytes), size.name == "refresh")
				})
			}
		})
	}
}

func exchangeInstalledTextAdministration(t *testing.T, carrier route.CarrierProfile, body []byte, refresh bool) {
	t.Helper()
	readerOwner, publisherOwner := textUnpublishedNetworkWithInstance(t, carrier, func(network [32]byte, now, until time.Time) (*instance.Root, *instance.Binding) {
		return acquireInstalledServiceInstance(t, network, now, until)
	})
	owner, err := publisherOwner.openTextAdministration()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := owner.Close(); err != nil {
			t.Error(err)
		}
	})
	directory, err := os.MkdirTemp("", "text-administration-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(directory); err != nil {
			t.Error(err)
		}
	})
	socket := filepath.Join(directory, "admin.sock")
	server, err := administration.Listen(socket, owner)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Error(err)
		}
	})
	limit := time.Minute
	if refresh {
		limit = 8 * time.Minute
	}
	ctx, cancel := context.WithTimeout(t.Context(), limit)
	defer cancel()
	startup, finishStartup := context.WithCancel(ctx)
	document := filepath.Join(directory, "document.txt")
	if err := os.WriteFile(document, body, 0600); err != nil {
		t.Fatal(err)
	}
	published := runInstalledTextCommand(t, startup, nil, "publish", socket, document)
	finishStartup()
	if len(published) != 0 {
		t.Fatal("publish command emitted unexpected output")
	}
	owner.mu.Lock()
	run := owner.run
	owner.mu.Unlock()
	if run == nil {
		t.Fatal("published receipt without retained publication")
	}
	if refresh {
		observeInstalledTextRefresh(t, ctx, publisherOwner, run)
	}
	// Obtain the destination through the real separately authorized local owner.
	// No test-side projection of the run's private fields supplies the Link.
	reader, err := readerOwner.openTextConnection()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := reader.Close(); err != nil {
			t.Error(err)
		}
	})
	readSocket := filepath.Join(directory, "read.sock")
	readServer, err := applicationconnection.Listen(readSocket, reader)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := readServer.Close(); err != nil {
			t.Error(err)
		}
	})
	destination := runInstalledTextCommand(t, ctx, nil, "link", socket)
	defer clear(destination)
	if len(destination) < 2 || destination[len(destination)-1] != '\n' || bytes.Count(destination, []byte{'\n'}) != 1 {
		t.Fatal("link command did not emit exactly one destination line")
	}
	actual := runInstalledTextCommand(t, ctx, destination, "read", readSocket)
	defer clear(actual)
	if !bytes.Equal(actual, body) {
		t.Fatalf("installed command document length %d, wanted %d", len(actual), len(body))
	}
	readerOwner.mu.Lock()
	retired := readerOwner.job == nil && readerOwner.lastJob != nil && readerOwner.lastJob.finished && readerOwner.lastJob.cleanupErr == nil
	readerOwner.mu.Unlock()
	if !retired {
		t.Fatal("document returned before reader retirement")
	}
	outcome, err := administration.Request(ctx, socket, administration.Withdraw)
	if err != nil || outcome != administration.Withdrawn {
		t.Fatalf("withdraw Administration: %s, %v", outcome, err)
	}
	select {
	case <-publisherOwner.done:
	default:
		t.Fatal("withdraw returned before Publisher retirement")
	}
}

// Execute the manifest-pinned ordinary artifact, outside its worker root, in
// trusted UI mode. CommandContext and WaitDelay bound and join child/pipes.
func runInstalledTextCommand(t *testing.T, ctx context.Context, input []byte, arguments ...string) []byte {
	t.Helper()
	if _, err := loadTextWorkerArtifact(); err != nil {
		t.Fatalf("installed command artifact: %v", err)
	}
	bounded, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	command := exec.CommandContext(bounded, textWorkerRoot+"/ardents-text", arguments...)
	command.WaitDelay = 5 * time.Second
	command.Stdin = bytes.NewReader(input)
	limit := map[string]int{"publish": 0, "link": 514, "read": 4 << 20}[arguments[0]]
	output := &installedTextOutput{limit: limit, cancel: cancel}
	diagnostic := &installedTextOutput{limit: 4096, cancel: cancel}
	command.Stdout, command.Stderr = output, diagnostic
	err := command.Run()
	defer clear(diagnostic.buffer.Bytes())
	if err != nil || bounded.Err() != nil || diagnostic.buffer.Len() != 0 {
		clear(output.buffer.Bytes())
		t.Fatalf("installed %s command failed: %v / %v (diagnostic bytes %d)", arguments[0], err, bounded.Err(), diagnostic.buffer.Len())
	}
	return output.buffer.Bytes()
}

// Each stream has one os/exec copy goroutine. Overflow cancels the child before
// returning an error, so neither a noisy child nor its pipe can outlive Run.
type installedTextOutput struct {
	buffer bytes.Buffer
	limit  int
	cancel context.CancelFunc
}

func (output *installedTextOutput) Write(value []byte) (int, error) {
	if len(value) > output.limit-output.buffer.Len() {
		output.cancel()
		return 0, io.ErrShortBuffer
	}
	return output.buffer.Write(value)
}
