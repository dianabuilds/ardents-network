//go:build linux

package endpoint

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
	processdiag "github.com/dianabuilds/ardents-network/internal/diagnostics/process"
	"github.com/dianabuilds/ardents-network/internal/route/carrier"
)

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
