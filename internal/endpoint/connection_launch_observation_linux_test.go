//go:build linux

package endpoint

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	applicationconnection "github.com/dianabuilds/ardents-network/internal/application/connection"
	processdiag "github.com/dianabuilds/ardents-network/internal/diagnostics/process"
	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

// This is Open's real cancellation path through admission and its installed
// launch gate. It supplies no worker artifact or launch-success substitute.
func TestTextConnectionObservationsJoinRealCancelledLaunch(t *testing.T) {
	dir, err := os.MkdirTemp("", "ardents-reader-observe-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	socket := filepath.Join(dir, "debug.sock")
	err = processdiag.Run(t.Context(), socket, func(ctx context.Context) error {
		endpoint, principal := dutyContextEndpoint(t)
		endpoint.network = fixtureID(221)
		capability, err := endpoint.Admit(principal, broker.Connection)
		if err != nil {
			return err
		}
		contextOwner, err := endpoint.beginDutyContext(ctx, capability, principal, broker.Connection)
		if err != nil {
			return err
		}
		owner, err := contextOwner.openConnection()
		if err != nil {
			return err
		}
		defer owner.Close()
		release, err := endpoint.acquireLaunch(ctx)
		if err != nil {
			return err
		}
		defer release()
		destination, err := targetlink.Encode(targetlink.Link{Network: endpoint.network, Target: fixtureID(223)})
		if err != nil {
			return err
		}
		caller, cancel := context.WithCancel(ctx)
		defer cancel()
		completed := make(chan error, 1)
		go func() {
			stream, err := owner.Open(caller, applicationconnection.Request{Destination: applicationconnection.TargetLink, Value: destination})
			if stream != nil {
				err = errors.Join(err, stream.Close())
			}
			completed <- err
		}()
		var job *jobIdentity
		deadline := time.NewTimer(5 * time.Second)
		defer deadline.Stop()
		tick := time.NewTicker(time.Millisecond)
		defer tick.Stop()
		for job == nil {
			contextOwner.mu.Lock()
			job = contextOwner.job
			contextOwner.mu.Unlock()
			if job != nil {
				break
			}
			select {
			case <-completed:
				return errors.New("Open did not reserve real launch")
			case <-deadline.C:
				return errors.New("real launch reservation unavailable")
			case <-tick.C:
			}
		}
		cancel()
		if err := <-completed; err == nil {
			return errors.New("canceled launch succeeded")
		}
		owner.mu.Lock()
		pending := owner.pending
		owner.mu.Unlock()
		contextOwner.mu.Lock()
		finished := job.finished
		retained := contextOwner.job
		contextOwner.mu.Unlock()
		if pending != nil || retained != nil || !finished || endpoint.admission.Active() != 1 {
			return errors.New("Open did not join ownership")
		}
		client := &http.Client{Timeout: time.Second, Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", socket)
		}}}
		defer client.CloseIdleConnections()
		response, err := client.Get("http://diagnostic/connection")
		if err != nil {
			return err
		}
		defer response.Body.Close()
		body, err := io.ReadAll(io.LimitReader(response.Body, 32<<10))
		if err != nil {
			return err
		}
		var snapshot struct {
			State   string
			Outcome string
			Records []struct {
				Stage       string
				State       string
				ContextStop string `json:"context_stop"`
				Budget      *int64 `json:"remaining_budget_ns"`
			}
		}
		if err := json.Unmarshal(body, &snapshot); err != nil {
			return err
		}
		if snapshot.State != "joined" || snapshot.Outcome != "failed" {
			return errors.New("real cancellation observation incomplete")
		}
		hasActivationBudget := false
		for _, record := range snapshot.Records {
			if record.Stage == "worker-activation" && record.State == "started" && record.Budget != nil && *record.Budget > 0 && *record.Budget <= int64(15*time.Second) {
				hasActivationBudget = true
			}
		}
		if !hasActivationBudget {
			return errors.New("actual activation deadline missing")
		}
		completedStages := map[string]string{}
		stops := map[string]string{}
		for _, record := range snapshot.Records {
			if record.State != "started" {
				completedStages[record.Stage] = record.State
				stops[record.Stage] = record.ContextStop
			}
		}
		for _, stage := range []string{"admission", "activation", "worker-launch", "worker-activation", "caller-join", "session-release"} {
			if completedStages[stage] == "" {
				return errors.New("real launch stage missing")
			}
		}
		if completedStages["worker-launch"] != "failed" || stops["worker-launch"] != "canceled" || strings.Contains(string(body), destination) {
			return errors.New("launch class or privacy invalid")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
