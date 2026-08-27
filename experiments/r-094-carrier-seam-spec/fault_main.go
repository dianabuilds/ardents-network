//go:build ignore

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

type faultCaseResult struct {
	Schema          string      `json:"schema"`
	Role            string      `json:"role"`
	Event           string      `json:"event"`
	Case            string      `json:"case"`
	Profile         profileID   `json:"profile"`
	Expected        resultClass `json:"expected,omitempty"`
	Observed        resultClass `json:"observed,omitempty"`
	Passed          bool        `json:"passed,omitempty"`
	Endpoint        string      `json:"endpoint,omitempty"`
	AdapterCalls    string      `json:"adapter_calls,omitempty"`
	OfferDigest     string      `json:"offer_digest,omitempty"`
	PayloadBytes    int         `json:"payload_bytes,omitempty"`
	PayloadSHA256   string      `json:"payload_sha256,omitempty"`
	GoroutinesStart int         `json:"goroutines_start,omitempty"`
	GoroutinesEnd   int         `json:"goroutines_end,omitempty"`
	FDsStart        int         `json:"fds_start,omitempty"`
	FDsEnd          int         `json:"fds_end,omitempty"`
	CleanupJoined   bool        `json:"cleanup_joined,omitempty"`
	ElapsedMS       int64       `json:"elapsed_ms,omitempty"`
	RelayControl    string      `json:"relay_control,omitempty"`
	RemoteStart     string      `json:"remote_start,omitempty"`
	RemoteEnd       string      `json:"remote_end,omitempty"`
	Note            string      `json:"note,omitempty"`
}

type faultArguments struct {
	profile       profileID
	endpoint      string
	caseName      string
	token         string
	deadline      time.Time
	expected      resultClass
	relayControl  string
	requestRebind bool
	resumeFile    string
	holdOutcome   bool
}

const classSuccessOrTimeout resultClass = "success-or-timeout"

func main() {
	if len(os.Args) < 2 {
		faultExit(errors.New("expected fault-server or fault-client subcommand"))
	}
	var err error
	switch os.Args[1] {
	case "fault-server":
		err = runFaultServer(os.Args[2:])
	case "fault-client":
		err = runFaultClient(os.Args[2:])
	case "fault-relay":
		err = runFaultRelay(os.Args[2:])
	default:
		err = fmt.Errorf("unknown subcommand %q", os.Args[1])
	}
	if err != nil {
		faultExit(err)
	}
}

func runFaultServer(arguments []string) error {
	parsed, err := parseFaultArguments("fault-server", arguments)
	if err != nil {
		return err
	}
	material, err := deterministicIdentities()
	if err != nil {
		return err
	}
	attempt := faultAttempt(material, parsed.profile, parsed.endpoint, parsed.token, parsed.deadline)
	ctx, cancel := context.WithDeadline(context.Background(), parsed.deadline)
	defer cancel()
	started := time.Now()
	startGoroutines, startFDs := runtime.NumGoroutine(), faultFDCount()
	path := faultPathObservation{}
	ready := func(endpoint string) {
		faultEncode(faultCaseResult{Schema: "ardents-r094-fault-case-v1", Role: "server", Event: "ready",
			Case: parsed.caseName, Profile: parsed.profile, Endpoint: endpoint, OfferDigest: faultOfferDigest(attempt)})
	}
	serveErr := serveFaultProfile(ctx, parsed.profile, parsed.endpoint, material.server, material.clientID,
		reciprocalFault(attempt.Binding), ready, &path)
	observed := classOf(classify(serveErr))
	endGoroutines, endFDs, cleanup := faultWaitForCleanup(startGoroutines, startFDs)
	passed := faultExpected(parsed.expected, observed) && cleanup
	faultEncode(faultCaseResult{Schema: "ardents-r094-fault-case-v1", Role: "server", Event: "outcome",
		Case: parsed.caseName, Profile: parsed.profile, Expected: parsed.expected, Observed: observed, Passed: passed,
		OfferDigest: faultOfferDigest(attempt), GoroutinesStart: startGoroutines, GoroutinesEnd: endGoroutines,
		FDsStart: startFDs, FDsEnd: endFDs, CleanupJoined: cleanup, ElapsedMS: time.Since(started).Milliseconds(),
		RemoteStart: path.remoteStart, RemoteEnd: path.remoteEnd, Note: faultNote(serveErr)})
	if parsed.holdOutcome {
		time.Sleep(2 * time.Second)
	}
	if !passed {
		return errors.New("fault server result did not match its oracle")
	}
	return nil
}

func runFaultClient(arguments []string) error {
	parsed, err := parseFaultArguments("fault-client", arguments)
	if err != nil {
		return err
	}
	material, err := deterministicIdentities()
	if err != nil {
		return err
	}
	attempt := faultAttempt(material, parsed.profile, parsed.endpoint, parsed.token, parsed.deadline)
	module := newCarrierModule()
	ctx, cancel := context.WithDeadline(context.Background(), parsed.deadline)
	defer cancel()
	started := time.Now()
	startGoroutines, startFDs := runtime.NumGoroutine(), faultFDCount()
	resumeErr := faultPrepareResume(parsed.resumeFile)
	var carrier *authenticatedCarrier
	var openErr error
	if resumeErr == nil {
		carrier, openErr = module.Open(ctx, attempt)
	}
	var rebindErr, transcriptErr, closeErr, stopErr error
	relayControlResult := ""
	payloadDigest := sha256.Sum256(faultPayload())
	if resumeErr == nil && openErr == nil && parsed.resumeFile != "" {
		faultEncode(faultCaseResult{Schema: "ardents-r094-fault-case-v1", Role: "client", Event: "opened",
			Case: parsed.caseName, Profile: parsed.profile, Endpoint: parsed.endpoint,
			AdapterCalls: faultCalls(module), OfferDigest: faultOfferDigest(attempt)})
		resumeErr = faultWaitForResume(ctx, parsed.resumeFile)
	}
	if resumeErr == nil && openErr == nil {
		if parsed.requestRebind {
			relayControlResult, rebindErr = sendFaultRelayControl(
				parsed.relayControl, "rebind", parsed.token, parsed.deadline,
			)
		}
		if rebindErr == nil {
			payloadDigest, transcriptErr = faultExchangeClient(carrier)
		}
		closeErr = carrier.Close()
	}
	if parsed.relayControl != "" {
		time.Sleep(50 * time.Millisecond)
		_, stopErr = sendFaultRelayControl(parsed.relayControl, "stop", parsed.token, parsed.deadline)
	}
	resultErr := errors.Join(resumeErr, openErr, rebindErr, transcriptErr, closeErr, stopErr)
	observed := classOf(resultErr)
	endGoroutines, endFDs, cleanup := faultWaitForCleanup(startGoroutines, startFDs)
	passed := faultExpected(parsed.expected, observed) && faultSelectedOnce(module, parsed.profile) && cleanup
	if observed == classSuccess {
		passed = passed && transcriptErr == nil && closeErr == nil
	}
	faultEncode(faultCaseResult{Schema: "ardents-r094-fault-case-v1", Role: "client", Event: "outcome",
		Case: parsed.caseName, Profile: parsed.profile, Expected: parsed.expected, Observed: observed, Passed: passed,
		Endpoint: parsed.endpoint, AdapterCalls: faultCalls(module), OfferDigest: faultOfferDigest(attempt),
		PayloadBytes: faultPayloadSize, PayloadSHA256: hex.EncodeToString(payloadDigest[:]),
		GoroutinesStart: startGoroutines, GoroutinesEnd: endGoroutines, FDsStart: startFDs, FDsEnd: endFDs,
		CleanupJoined: cleanup, ElapsedMS: time.Since(started).Milliseconds(),
		RelayControl: relayControlResult, Note: faultNote(resultErr)})
	if !passed {
		return errors.New("fault client result did not match its oracle")
	}
	return nil
}

func parseFaultArguments(name string, arguments []string) (faultArguments, error) {
	set := flag.NewFlagSet(name, flag.ContinueOnError)
	profile := set.String("profile", "", "exact experiment profile")
	endpoint := set.String("endpoint", "", "listen or dial address")
	caseName := set.String("case", "", "stable case identifier")
	token := set.String("token", "", "shared attempt token")
	deadlineUnix := set.Int64("deadline-unix", 0, "shared absolute deadline in Unix seconds")
	expected := set.String("expect", "", "expected result class")
	relayControl := set.String("relay-control", "", "lab-only UDP relay control endpoint")
	requestRebind := set.Bool("request-rebind", false, "rebind relay upstream after Carrier Open")
	resumeFile := set.String("resume-file", "", "lab-only pause file checked after Carrier Open")
	holdOutcome := set.Bool("hold-after-outcome", false, "lab-only server hold for final namespace observation")
	if err := set.Parse(arguments); err != nil {
		return faultArguments{}, err
	}
	parsed := faultArguments{profile: profileID(*profile), endpoint: *endpoint, caseName: *caseName, token: *token,
		deadline: time.Unix(*deadlineUnix, 0).UTC(), expected: resultClass(*expected),
		relayControl: *relayControl, requestRebind: *requestRebind, resumeFile: *resumeFile,
		holdOutcome: *holdOutcome}
	if (parsed.profile != tcpProfile && parsed.profile != quicProfile) || parsed.endpoint == "" ||
		parsed.caseName == "" || parsed.token == "" || *deadlineUnix <= time.Now().Unix() ||
		(parsed.expected != classSuccess && parsed.expected != classTimeout && parsed.expected != classSuccessOrTimeout) ||
		(parsed.requestRebind && parsed.relayControl == "") ||
		(parsed.resumeFile != "" && (name != "fault-client" || parsed.resumeFile != faultResumePath)) ||
		(parsed.holdOutcome && name != "fault-server") {
		return faultArguments{}, errors.New("fault arguments are incomplete or unsupported")
	}
	return parsed, nil
}

func faultExpected(expected, observed resultClass) bool {
	if expected == classSuccessOrTimeout {
		return observed == classSuccess || observed == classTimeout
	}
	return observed == expected
}

func faultAttempt(material identities, profile profileID, endpoint, token string, deadline time.Time) carrierAttempt {
	return carrierAttempt{
		AuthorityDigest: sha256.Sum256([]byte("r094-fault-authority")), Profile: profile, Endpoint: endpoint,
		ExpectedPeer: material.serverID, Certificate: material.client, Deadline: deadline,
		Binding: route.LegBinding{
			NetworkID: sha256.Sum256([]byte("r094-fault-network")), Epoch: 1,
			Digest: sha256.Sum256([]byte("r094-fault-epoch")), AttachmentID: sha256.Sum256([]byte(token)),
			SenderRole: 1, PeerRole: 3, SenderNodeID: material.clientID, PeerNodeID: material.serverID,
			NotAfter: deadline,
		},
	}
}

func reciprocalFault(value route.LegBinding) route.LegBinding {
	return route.LegBinding{
		NetworkID: value.NetworkID, Epoch: value.Epoch, Digest: value.Digest, AttachmentID: value.AttachmentID,
		SenderRole: value.PeerRole, PeerRole: value.SenderRole, SenderNodeID: value.PeerNodeID,
		PeerNodeID: value.SenderNodeID, NotAfter: value.NotAfter,
	}
}

func faultWaitForCleanup(startGoroutines, startFDs int) (int, int, bool) {
	deadline := time.Now().Add(time.Second)
	for {
		goroutines, descriptors := runtime.NumGoroutine(), faultFDCount()
		if goroutines <= startGoroutines+2 && (startFDs < 0 || descriptors <= startFDs) {
			return goroutines, descriptors, true
		}
		if !time.Now().Before(deadline) {
			return goroutines, descriptors, false
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func faultFDCount() int {
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return -1
	}
	return len(entries)
}

func faultSelectedOnce(module *carrierModule, profile profileID) bool {
	return module.calls[profile] == 1 && module.calls[tcpProfile]+module.calls[quicProfile] == 1
}

func faultCalls(module *carrierModule) string {
	return fmt.Sprintf("tcp=%d,quic=%d", module.calls[tcpProfile], module.calls[quicProfile])
}

func faultOfferDigest(attempt carrierAttempt) string {
	binding, _ := route.EncodeLegBinding(attempt.Binding)
	value := append([]byte(attempt.Profile), 0)
	value = append(value, attempt.AuthorityDigest[:]...)
	value = append(value, attempt.ExpectedPeer[:]...)
	value = append(value, binding...)
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

func faultNote(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func faultEncode(value faultCaseResult) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(value)
}

func faultExit(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
