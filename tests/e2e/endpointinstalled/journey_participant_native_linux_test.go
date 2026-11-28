//go:build linux && endpoint_pair_native && document_worker_installed

package endpointinstalled_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/administration"
	"github.com/dianabuilds/ardents-network/internal/application/document"
	"github.com/dianabuilds/ardents-network/internal/application/endpointstream"
	endpointclient "github.com/dianabuilds/ardents-network/internal/application/endpointstream/client"
	endpointfixture "github.com/dianabuilds/ardents-network/tests/e2e/endpoint"
)

// TestNativeEndpointPlatformLedJourneyParticipant runs inside one guest manager.
// It receives only public signed Network material and actual permission
// requests/responses over the local fixture channel; no private key, live owner
// or accepting flag crosses managers.
func TestNativeEndpointPlatformLedJourneyParticipant(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Fatal("journey participant requires a root driver inside its own manager")
	}
	role := os.Getenv(endpointfixture.JourneyRoleEnv)
	if role != "publisher" && role != "reader" {
		t.Fatal("invalid environment: selected journey role required")
	}
	base := os.Getenv(endpointfixture.JourneyRootEnv)
	if !filepath.IsAbs(base) {
		t.Fatal("invalid environment: absolute journey root required")
	}
	nonce := os.Getenv(endpointfixture.JourneyNonceEnv)
	if decoded, err := hex.DecodeString(nonce); err != nil || len(decoded) != 32 {
		t.Fatal("invalid environment: original fixture nonce required")
	}
	conn, err := (&net.Dialer{}).DialContext(t.Context(), "tcp", os.Getenv(endpointfixture.JourneyAddressEnv))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	deadline := time.Now().Add(6 * time.Minute)
	if testDeadline, ok := t.Deadline(); ok {
		deadline = testDeadline
	}
	if err := conn.SetDeadline(deadline); err != nil {
		t.Fatal("bound journey participant channel", err)
	}
	endpointfixture.SendPairMessage(t, conn, endpointfixture.PairMessage{Operation: "hello", Role: role, Nonce: nonce})
	input := endpointfixture.ReceivePairMessage(t, conn)
	if input.Operation != "network" || input.Network == nil {
		t.Fatal("journey fixture lacks signed Network input")
	}
	etcDir := filepath.Join("/etc/ardents", "journey-"+nonce)
	approve := func(_, permissionRole string, request []byte) []byte {
		endpointfixture.SendPairMessage(t, conn, endpointfixture.PairMessage{Operation: "permission", Role: permissionRole, Body: request})
		response := endpointfixture.ReceivePairMessage(t, conn)
		if response.Operation != "permission" || response.Role != permissionRole {
			t.Fatal("foreign fixture approval response")
		}
		return response.Body
	}
	exercise := func(participant endpointfixture.JourneyParticipant) {
		original := participant.Original
		var publisherProcess *exec.Cmd
		var publisherOutput, publisherDiagnostic bytes.Buffer
		var publisherDone chan error
		var publisherStream *endpointfixture.JourneyStreamOutput
		defer func() {
			if publisherProcess != nil {
				_ = publisherProcess.Process.Kill()
				<-publisherDone
			}
		}()
		endpointfixture.SendPairMessage(t, conn, endpointfixture.PairMessage{Operation: "ready", Original: original, Principals: participant.Principals})
		for {
			command := endpointfixture.ReceivePairMessage(t, conn)
			if command.Operation != "restart" && command.Operation != "stream-result" && endpointfixture.StartupIdentity(t, t.Context()) != original {
				t.Fatal("journey command lost original installed participant")
			}
			response := endpointfixture.PairMessage{Operation: command.Operation, Original: original}
			switch command.Operation {
			case "publish":
				if role != "publisher" {
					t.Fatal("reader received publication fixture command")
				}
				path := filepath.Join(t.TempDir(), "snapshot-input")
				if err := os.WriteFile(path, command.Body, 0600); err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithTimeout(t.Context(), administration.SnapshotTimeout)
				err := administration.PublishLocal(ctx, filepath.Join(participant.Base, "administration", "socket"), path)
				cancel()
				if err != nil {
					endpointfixture.DumpJourneyJournal(t)
					t.Fatal("journey installed Snapshot", err)
				}
				ctx, cancel = context.WithTimeout(t.Context(), 15*time.Second)
				response.Link, err = administration.RequestPublishedLink(ctx, filepath.Join(participant.Base, "administration", "socket"))
				cancel()
				if err != nil {
					t.Fatal("journey installed Link", err)
				}
			case "file-publish":
				if role != "publisher" || command.Stream == nil || publisherProcess != nil {
					t.Fatal("invalid binary file publication command")
				}
				input, err := os.CreateTemp(t.TempDir(), "binary-input-")
				if err != nil {
					t.Fatal(err)
				}
				_, writeErr := io.Copy(input, command.Stream.Reader())
				closeErr := input.Close()
				if writeErr != nil || closeErr != nil {
					t.Fatalf("prepare binary file: copy %v, close %v", writeErr, closeErr)
				}
				socket := filepath.Join(participant.Base, "administration", "socket")
				process := exec.CommandContext(t.Context(), os.Getenv("ARDENTS_PREBUILT_NEXT"), "publish", socket, input.Name())
				if output, err := process.CombinedOutput(); err != nil {
					endpointfixture.DumpJourneyJournal(t)
					t.Fatalf("installed binary file publication: %v: %s", err, output)
				}
				ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
				response.Link, err = administration.RequestPublishedLink(ctx, socket)
				cancel()
				if err != nil {
					t.Fatal("journey installed binary file Link", err)
				}
			case "read":
				if role != "reader" {
					t.Fatal("publisher received reader fixture command")
				}
				ctx, cancel := context.WithTimeout(t.Context(), endpointstream.ReaderTimeout)
				stream, err := endpointclient.Dial(ctx, filepath.Join(participant.Base, "application", "reader"), endpointstream.Request{Destination: endpointstream.TargetLink, Value: command.Link})
				if err == nil {
					var output bytes.Buffer
					err = document.ReceiveFile(ctx, stream, &output)
					response.Body = output.Bytes()
				}
				cancel()
				if err != nil {
					t.Fatal("journey installed reader exchange", err)
				}
			case "stream-publish":
				if role != "publisher" || publisherProcess != nil {
					t.Fatal("invalid byte-stream publication command")
				}
				socket := filepath.Join(participant.Base, "administration", "socket")
				process := exec.CommandContext(t.Context(), os.Getenv("ARDENTS_PREBUILT_NEXT"), "stream", "publish", socket)
				process.Stdin = bytes.NewReader(command.Body)
				process.Stdout, process.Stderr = &publisherOutput, &publisherDiagnostic
				if command.Stream != nil {
					publisherStream = endpointfixture.NewJourneyStreamOutput()
					process.Stdin = command.Stream.PacedReader(t.Context())
					process.Stdout = publisherStream
				}
				if err := process.Start(); err != nil {
					t.Fatal(err)
				}
				publisherProcess = process
				publisherDone = make(chan error, 1)
				go func(done chan<- error) { done <- process.Wait() }(publisherDone)
				journeyWait(t, 55*time.Second, func() bool {
					select {
					case err := <-publisherDone:
						publisherProcess = nil
						t.Fatalf("stream publisher stopped before readiness: %v: %s", err, &publisherDiagnostic)
					default:
					}
					ctx, cancel := context.WithTimeout(t.Context(), time.Second)
					defer cancel()
					link, err := administration.RequestPublishedLink(ctx, socket)
					response.Link = link
					return err == nil && link != ""
				})
			case "stream-connect":
				if role != "reader" {
					t.Fatal("invalid byte-stream connection command")
				}
				path := filepath.Join(t.TempDir(), "target-link")
				if err := os.WriteFile(path, []byte(command.Link), 0600); err != nil {
					t.Fatal(err)
				}
				socket := filepath.Join(participant.Base, "application", "reader")
				process := exec.CommandContext(t.Context(), os.Getenv("ARDENTS_PREBUILT_NEXT"), "stream", "connect", socket, "--link-file", path)
				process.Stdin = bytes.NewReader(command.Body)
				var diagnostic bytes.Buffer
				process.Stderr = &diagnostic

				var output []byte
				var err error
				if command.Stream == nil {
					output, err = process.Output()
				} else {
					stream := endpointfixture.NewJourneyStreamOutput()
					process.Stdin, process.Stdout = command.Stream.PacedReader(t.Context()), stream
					err = process.Run()
					proof := stream.Proof()
					response.Proof = &proof
				}
				if err != nil {
					endpointfixture.DumpJourneyJournal(t)
					response.Failure = fmt.Sprintf("stream connect: %v: %s", err, &diagnostic)
					t.Log(response.Failure)
				}
				response.Body = output
			case "stream-result":
				if role != "publisher" || publisherProcess == nil {
					t.Fatal("invalid byte-stream result command")
				}
				err := <-publisherDone
				publisherProcess = nil
				if err != nil {
					endpointfixture.DumpJourneyJournal(t)
					response.Failure = fmt.Sprintf("stream publish: %v: %s", err, &publisherDiagnostic)
					t.Log(response.Failure)
				} else {
					response.Original = endpointfixture.StartupIdentity(t, t.Context())
				}

				if publisherStream == nil {
					response.Body = publisherOutput.Bytes()
				} else {
					proof := publisherStream.Proof()
					response.Proof = &proof
				}
			case "withdraw":
				if role != "publisher" {
					t.Fatal("reader received withdrawal fixture command")
				}
				ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
				_, err := administration.Request(ctx, filepath.Join(participant.Base, "administration", "socket"), administration.Withdraw)
				cancel()
				if err != nil {
					t.Fatal("journey installed withdrawal", err)
				}
				ctx, cancel = context.WithTimeout(t.Context(), 15*time.Second)
				link, err := administration.RequestPublishedLink(ctx, filepath.Join(participant.Base, "administration", "socket"))
				cancel()
				if err == nil || link != "" {
					t.Fatal("withdrawn installed publisher still exposes a Link")
				}
			case "restart":
				original = journeyRestartContinuity(t, participant, original, approve)
				response.Original = original
				endpointfixture.SendPairMessage(t, conn, response)
				return
			case "finish":
				endpointfixture.SendPairMessage(t, conn, response)
				return
			default:
				t.Fatal("unknown journey fixture command")
			}
			endpointfixture.SendPairMessage(t, conn, response)
		}
	}
	binary := os.Getenv("ARDENTS_PREBUILT_NEXT")
	if binary == "" {
		binary = buildArdentsNext(t)
	}
	endpointfixture.RunJourneyParticipant(t, binary, base, etcDir, *input.Network, approve, exercise)
}

// journeyRestartContinuity proves a platform-led restart reaches Service
// readiness again under a new InvocationID with the same immutable selection and
// retained durable floors (the platform-led replacement for the transitional
// pair's restart-refusal). The restarted invocation erased its volatile holder and
// retired the prior permission handover, so the external fixture authority
// re-approves its fresh request before readiness, exactly as the original
// bootstrap. It returns the new original invocation.
func journeyRestartContinuity(t *testing.T, participant endpointfixture.JourneyParticipant, previous [3]string, approve func(base, role string, request []byte) []byte) [3]string {
	t.Helper()
	selectionPath := filepath.Join(participant.Base, "installation", "selection.json")
	before, err := os.ReadFile(selectionPath)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("systemctl", "stop", "ardents-endpoint.service").CombinedOutput(); err != nil {
		t.Fatalf("restart stop: %v: %s", err, out)
	}
	journeyWait(t, 30*time.Second, func() bool { return endpointfixture.StartupIdentity(t, t.Context())[2] != "active" })
	if out, err := exec.Command("systemctl", "start", "ardents-endpoint.service").CombinedOutput(); err != nil {
		endpointfixture.DumpJourneyJournal(t)
		t.Fatalf("restart start: %v: %s", err, out)
	}
	current := endpointfixture.StartupIdentity(t, t.Context())
	if current[0] == "0" || len(current[1]) != 32 || current[2] != "active" {
		t.Fatal("restarted participant is not active", current)
	}
	if current[1] == previous[1] {
		t.Fatal("platform-led restart reused the original invocation", current)
	}
	after, err := os.ReadFile(selectionPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("platform-led restart changed the immutable selection", err)
	}
	endpointfixture.GrantOriginalPermissions(t, participant.Base, current[1], participantUID(t), participantGID(t), 60*time.Second, approve)
	endpointfixture.WaitOriginalReader(t, filepath.Join(participant.Base, "application", "reader"), current, participantUID(t), participantGID(t), 60*time.Second)
	t.Logf("platform-led restart reached Service readiness; new InvocationID=%s, selection retained", current[1])
	return current
}

func participantUID(t *testing.T) uint32 {
	t.Helper()
	account, err := user.Lookup("ardents-endpoint")
	if err != nil {
		t.Fatal(err)
	}
	uid, err := strconv.ParseUint(account.Uid, 10, 32)
	if err != nil {
		t.Fatal(err)
	}
	return uint32(uid)
}

func participantGID(t *testing.T) uint32 {
	t.Helper()
	account, err := user.Lookup("ardents-endpoint")
	if err != nil {
		t.Fatal(err)
	}
	gid, err := strconv.ParseUint(account.Gid, 10, 32)
	if err != nil {
		t.Fatal(err)
	}
	return uint32(gid)
}

func journeyWait(t *testing.T, bound time.Duration, ready func() bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), bound)
	defer cancel()
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		if ready() {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("journey observation exceeded bound:", ctx.Err())
		case <-tick.C:
		}
	}
}
