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

	applicationadministration "github.com/dianabuilds/ardents-network/internal/application/administration"
	"github.com/dianabuilds/ardents-network/internal/application/broker"
	applicationconnection "github.com/dianabuilds/ardents-network/internal/application/connection"
	"github.com/dianabuilds/ardents-network/internal/endpoint/worker"
	"github.com/dianabuilds/ardents-network/internal/node"
	routecarrier "github.com/dianabuilds/ardents-network/internal/route/carrier"
	"github.com/dianabuilds/ardents-network/internal/service/instance"
)

func unpublishedNetworkWithInstance(t *testing.T, carrier routecarrier.CarrierProfile, acquire func([32]byte, time.Time, time.Time) (*instance.Root, *instance.Binding), configure ...func(int, *node.Config)) (*dutyContext, *dutyContext) {
	t.Helper()
	endpoint, publisher, source := publisherNetworkWithInstance(t, carrier, acquire, configure...)
	reader := permissionContextFixture(t, endpoint, fixtureID(211), broker.Connection)
	source.issuePermission(t, reader, [3]uint32{64, 64, 0})
	return reader, publisher
}

// Uses the installed launch verifier and actual confined worker processes.
// State/authority provisioning is the network fixture, not a command ceremony.
func TestInstalledTextWorkersReadTargetThroughJoinedNetwork(t *testing.T) {
	if err := worker.VerifyEndpointService(t.Context()); err != nil {
		t.Fatalf("invalid installed environment: %v", err)
	}
	for _, carrier := range []routecarrier.CarrierProfile{routecarrier.ClosedCarrierTCP, routecarrier.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			for _, size := range []struct {
				name  string
				bytes int
			}{{"empty", 0}, {"reference", 64 << 10}, {"maximum", 4 << 20}, {"refresh", 64 << 10}} {
				t.Run(size.name, func(t *testing.T) {
					exchangeInstalledAdministration(t, carrier, bytes.Repeat([]byte("x"), size.bytes), size.name == "refresh")
				})
			}
		})
	}
}

func exchangeInstalledAdministration(t *testing.T, carrier routecarrier.CarrierProfile, body []byte, refresh bool) {
	t.Helper()
	readerOwner, publisherOwner := unpublishedNetworkWithInstance(t, carrier, func(network [32]byte, now, until time.Time) (*instance.Root, *instance.Binding) {
		return acquireInstalledServiceInstance(t, network, now, until)
	})
	owner, err := publisherOwner.openAdministration()
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
	server, err := applicationadministration.Listen(socket, owner)
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
	published := runInstalledCommand(t, startup, nil, "publish", socket, document)
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
		observeInstalledRefresh(t, ctx, publisherOwner, run)
	}
	// Obtain the destination through the real separately authorized local owner.
	// No test-side projection of the run's private fields supplies the Link.
	reader, err := readerOwner.openConnection()
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
	destination := runInstalledCommand(t, ctx, nil, "link", socket)
	defer clear(destination)
	if len(destination) < 2 || destination[len(destination)-1] != '\n' || bytes.Count(destination, []byte{'\n'}) != 1 {
		t.Fatal("link command did not emit exactly one destination line")
	}
	actual := runInstalledCommand(t, ctx, destination, "read", readSocket)
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
	outcome, err := applicationadministration.Request(ctx, socket, applicationadministration.Withdraw)
	if err != nil || outcome != applicationadministration.Withdrawn {
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
func runInstalledCommand(t *testing.T, ctx context.Context, input []byte, arguments ...string) []byte {
	t.Helper()
	if _, err := worker.LoadArtifact(worker.Text); err != nil {
		t.Fatalf("installed command artifact: %v", err)
	}
	bounded, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	// The canonical artifact path is spelled independently of the worker
	// package; LoadArtifact above already pins and verifies the installed root.
	command := exec.CommandContext(bounded, "/usr/lib/ardents/text-worker-root/ardents-text", arguments...)
	command.WaitDelay = 5 * time.Second
	command.Stdin = bytes.NewReader(input)
	limit := map[string]int{"publish": 0, "link": 514, "read": 4 << 20}[arguments[0]]
	output := &installedOutput{limit: limit, cancel: cancel}
	diagnostic := &installedOutput{limit: 4096, cancel: cancel}
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
type installedOutput struct {
	buffer bytes.Buffer
	limit  int
	cancel context.CancelFunc
}

func (output *installedOutput) Write(value []byte) (int, error) {
	if len(value) > output.limit-output.buffer.Len() {
		output.cancel()
		return 0, io.ErrShortBuffer
	}
	return output.buffer.Write(value)
}
