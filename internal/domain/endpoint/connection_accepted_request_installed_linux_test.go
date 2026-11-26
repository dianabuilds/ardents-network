//go:build linux && document_worker_installed

package endpoint_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	endpointfixture "github.com/dianabuilds/ardents-network/tests/e2e/endpoint"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission/receiving"
	"github.com/dianabuilds/ardents-network/internal/connection"
	executionruntime "github.com/dianabuilds/ardents-network/internal/domain/execution/runtime"
	"github.com/dianabuilds/ardents-network/internal/domain/execution/worker"
	"github.com/dianabuilds/ardents-network/internal/domain/hosting"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/receiver"
	"github.com/dianabuilds/ardents-network/internal/route/transport"
	"github.com/dianabuilds/ardents-network/tests/e2e/node/processfixture"
)

const acceptedRequestRecovery = "accepted-request-smoke"
const installedRecoveryDocumentBytes = (64 << 10) - 13

var installedRecoveryRendezvous = [32]byte{41}

func TestInstalledConnectionAcceptedRequestRecoveryTCP(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Fatal("invalid environment: original non-root installed Endpoint required")
	}
	t.Run(string(transport.ClosedCarrierTCP), func(t *testing.T) {
		runInstalledConnectionTextLoss(t, transport.ClosedCarrierTCP, acceptedRequestRecovery)
	})
}

type installedRecoverySpends struct {
	presentations processfixture.AdmissionPresentations
	joinBudget    *hosting.Budget
}

// Stop at the first actual Publisher response write. The Text worker can
// produce it only after accepting the complete request and its EOF. Keep the
// original authenticated stream and terminal outcome behind this observation.
type installedAcceptedRequestStream struct {
	*observedConnectionOutcome
	ctx       context.Context
	mu        sync.Mutex
	request   [512]byte
	read      int
	eof       bool
	oversized bool
	writing   chan struct{}
	resume    chan struct{}
	first     sync.Once
	unblock   sync.Once
}

func (observed *installedAcceptedRequestStream) Read(body []byte) (int, error) {
	n, err := observed.observedConnectionOutcome.Read(body)
	observed.mu.Lock()
	copied := copy(observed.request[observed.read:], body[:n])
	observed.read += copied
	observed.oversized = observed.oversized || n != copied
	observed.eof = observed.eof || err == io.EOF
	observed.mu.Unlock()
	return n, err
}

func (observed *installedAcceptedRequestStream) Write(body []byte) (int, error) {
	if len(body) > 0 {
		observed.first.Do(func() {
			close(observed.writing)
			select {
			case <-observed.resume:
			case <-observed.ctx.Done():
			}
		})
	}
	if err := observed.ctx.Err(); err != nil {
		return 0, err
	}
	return observed.observedConnectionOutcome.Write(body)
}

func (observed *installedAcceptedRequestStream) requireRequest(t *testing.T) {
	t.Helper()
	var request [512]byte
	copy(request[:], "ARDTXT01\x01")
	observed.mu.Lock()
	defer observed.mu.Unlock()
	if observed.read != len(request) || observed.oversized || !observed.eof || !bytes.Equal(observed.request[:], request[:]) {
		t.Fatal("Publisher did not receive exactly one original fixed request and EOF")
	}
}

func (observed *installedAcceptedRequestStream) release() {
	observed.unblock.Do(func() { close(observed.resume) })
}

func (observed *installedAcceptedRequestStream) Close() error {
	observed.release()
	return observed.observedConnectionOutcome.Close()
}

func deliverReaderAfterAcceptedRequestLoss(t *testing.T, ctx context.Context, cancel context.CancelFunc, operation *executionruntime.Operation, contract executionruntime.StreamContract, stream *connection.Stream, publisher *installedAcceptedRequestStream, spends *installedRecoverySpends, loseOriginal func()) (bool, *executionruntime.ReaderResult, time.Time, error) {
	t.Helper()
	type delivery struct {
		transferred bool
		result      *executionruntime.ReaderResult
		err         error
	}
	completed := make(chan delivery, 1)
	go func() {
		transferred, result, err := operation.DeliverReaderStream(contract, stream)
		completed <- delivery{transferred, result, err}
	}()
	joined := false
	defer func() {
		publisher.release()
		if !joined {
			cancel()
			if outcome := <-completed; outcome.result != nil {
				outcome.result.Close()
			}
		}
	}()
	select {
	case <-publisher.writing:
	case outcome := <-completed:
		joined = true
		if outcome.result != nil {
			outcome.result.Close()
		}
		t.Fatal("Reader completed before the first original Publisher response", outcome.err)
	case <-ctx.Done():
		t.Fatal("original Publisher did not accept the request", ctx.Err())
	}
	publisher.requireRequest(t)
	if err := spends.presentations.Check(2); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	loseOriginal()
	publisher.release()
	outcome := <-completed
	joined = true
	return outcome.transferred, outcome.result, started, outcome.err
}

func (proof *installedRecoverySpends) observeAdmission(admit receiver.Admit, id [32]byte, budget *hosting.Budget) receiver.Admit {
	if id == installedRecoveryRendezvous {
		proof.joinBudget = budget
	}
	return func(ctx context.Context, channel receiver.Channel, token []byte) (receiving.Grant, error) {
		digest := sha256.Sum256(token)
		grant, err := admit(ctx, channel, token)
		selected := id == installedRecoveryRendezvous && channel.Hello.Purpose == ardp.PurposeDataJoin
		proof.presentations.Observe(selected, channel.Binding, digest, err)
		return grant, err
	}
}

func (proof *installedRecoverySpends) requireJoinBudgetReleased(t *testing.T, ctx context.Context) {
	t.Helper()
	if proof.joinBudget == nil {
		t.Fatal("original Rendezvous has no genuine Hosting budget")
	}
	observation, err := proof.joinBudget.Observe(ctx)
	if err != nil || observation.ReservedBytes != 0 {
		t.Fatalf("joined Rendezvous retained Hosting capacity: bytes=%d, error=%v", observation.ReservedBytes, err)
	}
}

// Retain the exact kernel object before any fault. A newly opened path could
// describe a different invocation after systemd has collected this one.
type installedRecoveryWorker struct {
	instance worker.Instance
	events   *os.File
}

func pinInstalledRecoveryWorker(t *testing.T, ctx context.Context, invocation *executionruntime.Invocation, wantRole string) installedRecoveryWorker {
	t.Helper()
	name, role, pid, uid := endpointfixture.InstalledConnectionWorkerIdentity(t, invocation)
	instance, err := worker.ObserveInstance(ctx, name, role)
	if err != nil || role != wantRole || uint64(instance.PID) != pid || uint64(instance.UID) != uid {
		t.Fatal("original worker manager identity differs", err)
	}
	root, err := os.OpenRoot("/sys/fs/cgroup" + instance.Cgroup)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	events, err := root.Open("cgroup.events")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := events.Close(); err != nil {
			t.Error(err)
		}
	})
	var filesystem syscall.Statfs_t
	if err := syscall.Fstatfs(int(events.Fd()), &filesystem); err != nil || filesystem.Type != 0x63677270 {
		t.Fatal("original cgroup events descriptor is unavailable", err)
	}
	pinned := installedRecoveryWorker{instance: instance, events: events}
	if state, err := pinned.population(); err != nil || state != "populated 1" {
		t.Fatal("original worker cgroup has no running process", err)
	}
	return pinned
}

func (pinned installedRecoveryWorker) population() (string, error) {
	if _, err := pinned.events.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	body, err := io.ReadAll(io.LimitReader(pinned.events, 4096))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(body), "\n") {
		if line == "populated 0" || line == "populated 1" {
			return line, nil
		}
	}
	return "", errors.New("original cgroup population is unavailable")
}

func (pinned installedRecoveryWorker) requireJoined(t *testing.T, parent context.Context) {
	t.Helper()
	state, err := pinned.population()
	if !errors.Is(err, syscall.ENODEV) && (err != nil || state != "populated 0") {
		t.Fatal("original worker cgroup still populated after join", err)
	}
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		output, err := exec.CommandContext(ctx, "systemctl", "list-units", "--no-legend", "--no-pager", "--plain", "--all", "--", pinned.instance.Name).Output()
		if err != nil {
			t.Fatal("original worker unit collection could not be observed", err)
		}
		if strings.TrimSpace(string(output)) == "" {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("original worker unit remained loaded after join")
		case <-ticker.C:
		}
	}
}
