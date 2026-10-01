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
	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
	processdiag "github.com/dianabuilds/ardents-network/internal/diagnostics/process"
	"github.com/dianabuilds/ardents-network/internal/route/carrier"
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

// The existing fixture supplies launch qualification only. Authentication,
// confined exchange, local document projection and joined cleanup remain real;
// admission/launch/Introduction observations are intentionally absent here.
func TestTextConnectionObservationsThroughRealServiceAndCleanup(t *testing.T) {
	for _, ending := range []string{"document", "cancel-before-request"} {
		t.Run(ending, func(t *testing.T) {
			dir, err := os.MkdirTemp("", "ardents-reader-service-observe-")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { os.RemoveAll(dir) })
			socket := filepath.Join(dir, "debug.sock")
			err = processdiag.Run(t.Context(), socket, func(ctx context.Context) (outcome error) {
				readerOwner, publisherOwner := unpublishedNetworkFixture(t, carrier.ClosedCarrierTCP)
				reader := serviceWorkerFixture(t, &serviceBinding{owner: readerOwner, job: liveCapsuleJob(t, readerOwner)}, nil)
				publisher := serviceWorkerFixture(t, &serviceBinding{owner: publisherOwner, job: liveCapsuleJob(t, publisherOwner)}, []byte("document"))
				run, err := publisher.startPublication(ctx)
				if err != nil {
					return err
				}
				defer func() {
					closeErr := run.Close()
					// Publisher remains an independent live owner after the Reader result.
					// This test explicitly retires it; only its exact cancellation tree is accepted.
					if closeErr != nil && !readCancellationOnly(closeErr) {
						outcome = errors.Join(outcome, errors.New("independent Publisher shutdown failed"), closeErr)
					}
				}()
				caller, cancel := context.WithTimeout(ctx, 30*time.Second)
				defer cancel()
				until := time.Now().Add(time.Minute).Unix()
				stream, err := openWorkerResultFixture(t, caller, reader, run.link, [3]int64{until, until, until})
				if err != nil {
					return err
				}
				if ending == "document" {
					body, err := textdocument.Read(caller, stream)
					if err != nil {
						return err
					}
					if string(body) != "document" {
						return errors.New("document exchange changed")
					}
					if err := stream.Close(); err != nil {
						return err
					}
				} else {
					cancel()
					select {
					case <-stream.joined:
					case <-time.After(5 * time.Second):
						return errors.New("canceled local request did not join")
					}
					if err := stream.Close(); !readCancellationOnly(err) {
						return errors.New("unexpected cancellation cleanup")
					}
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
				var snapshot struct {
					State   string
					Outcome string
					Records []struct {
						Stage string
						State string
					}
				}
				if err := json.NewDecoder(response.Body).Decode(&snapshot); err != nil {
					return err
				}
				expected := "completed"
				if ending == "cancel-before-request" {
					expected = "canceled"
				}
				if snapshot.State != "joined" || snapshot.Outcome != expected {
					return errors.New("Service snapshot joined outcome missing")
				}
				stages := map[string]string{}
				for _, record := range snapshot.Records {
					if record.State != "started" {
						stages[record.Stage] = record.State
					}
				}
				required := []string{"service-authentication", "local-request", "service-close", "worker-close", "caller-join", "session-release"}
				if ending == "document" {
					required = append(required, "document-exchange", "current-owner", "application-response")
				}
				for _, stage := range required {
					if stages[stage] == "" {
						return errors.New("real Service observation missing")
					}
				}
				for _, stage := range []string{"admission", "worker-launch", "introduction"} {
					if stages[stage] != "" {
						return errors.New("fixture invented production Open observation")
					}
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
