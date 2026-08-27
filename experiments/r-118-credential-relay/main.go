//go:build ignore

// This disposable program is invoked by file path. It keeps the candidate
// Credential Relay outside the maintained module/package map.
package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	requestSize  = 512
	responseSize = 512
	requestKind  = byte(1)
	responseKind = byte(2)
	accepted     = byte(1)
	rejected     = byte(2)
)

type selection struct {
	Network, Digest, Introduction [32]byte
	Epoch                         uint64
	NotAfter                      time.Time
}

type issuanceRequest struct {
	Network, Digest, Introduction, Attachment, ClientKey [32]byte
	Epoch                                                uint64
	Role                                                 byte
	NotAfter                                             time.Time
}

type issuanceResponse struct {
	Class byte
	Grant []byte
}

type issuerResult struct {
	PlaintextForbidden, AdmissionForwarded bool
	RemoteAddress                          string
	Accepted                               bool
}

type initiatorResult struct {
	EndpointAddress string
	Forwarded       int
	ReplayRefused   bool
}

type endpointResult struct {
	Accepted, Refused, GrantExact, ReplayRefused bool
}

func main() {
	role := flag.String("role", "coordinator", "experiment role")
	cell := flag.String("case", "exact", "matrix cell")
	issuer := flag.String("issuer", "", "issuer origin")
	initiator := flag.String("initiator", "", "Initiator origin")
	keyConfig := flag.String("key-config", "", "base64 OHTTP key configuration")
	admission := flag.String("admission", "", "base64 synthetic local admission proof")
	expiry := flag.String("expiry", "", "RFC3339 selected expiry")
	requests := flag.Int("requests", 1, "expected Endpoint requests")
	flag.Parse()
	if *role == "coordinator" {
		must(runCoordinator(*cell))
		return
	}
	deadline, err := time.Parse(time.RFC3339, *expiry)
	must(err)
	switch *role {
	case "issuer":
		must(runIssuer(*cell, selected(deadline)))
	case "initiator":
		must(runInitiator(*issuer, decode(*admission), *requests))
	case "endpoint":
		must(runEndpoint(*cell, *initiator, decode(*keyConfig), decode(*admission), selected(deadline)))
	default:
		must(errors.New("unknown experiment role"))
	}
}

func runCoordinator(cell string) error {
	if !validCell(cell) {
		return errors.New("unknown experiment case")
	}
	deadline := time.Now().UTC().Add(time.Minute).Truncate(time.Second)
	proof := make([]byte, 32)
	if _, err := rand.Read(proof); err != nil {
		return err
	}
	issuerChild, issuerReady, err := start("-role=issuer", "-case="+cell, "-expiry="+deadline.Format(time.RFC3339))
	if err != nil {
		return err
	}
	count := 1
	if cell == "replay-admission" {
		count = 2
	}
	initiatorChild, initiatorReady, err := start("-role=initiator", "-issuer=http://"+issuerReady[0], "-admission="+encode(proof), fmt.Sprintf("-requests=%d", count), "-expiry="+deadline.Format(time.RFC3339))
	if err != nil {
		_ = issuerChild.kill()
		return err
	}
	endpointChild, _, err := start("-role=endpoint", "-case="+cell, "-initiator=http://"+initiatorReady[0], "-key-config="+issuerReady[1], "-admission="+encode(proof), "-expiry="+deadline.Format(time.RFC3339))
	if err != nil {
		_ = issuerChild.kill()
		_ = initiatorChild.kill()
		return err
	}
	endpointRaw, endpointErr := endpointChild.wait()
	initiatorRaw, initiatorErr := initiatorChild.wait()
	issuerRaw, issuerErr := issuerChild.wait()
	if endpointErr != nil || initiatorErr != nil || issuerErr != nil {
		return errors.Join(endpointErr, initiatorErr, issuerErr)
	}
	var endpoint endpointResult
	var initiator initiatorResult
	var issuer issuerResult
	if err := json.Unmarshal(endpointRaw, &endpoint); err != nil {
		return err
	}
	if err := json.Unmarshal(initiatorRaw, &initiator); err != nil {
		return err
	}
	if err := json.Unmarshal(issuerRaw, &issuer); err != nil {
		return err
	}
	if err := assertCell(cell, endpoint, initiator, issuer); err != nil {
		return err
	}
	print(map[string]any{"case": cell, "result": "passed", "issuer_plaintext_forbidden": issuer.PlaintextForbidden,
		"issuer_admission_forwarded": issuer.AdmissionForwarded, "initiator_forwarded": initiator.Forwarded})
	return nil
}

type child struct {
	command *exec.Cmd
	output  *bufio.Reader
	errors  bytes.Buffer
}

func start(arguments ...string) (*child, []string, error) {
	command := exec.Command(os.Args[0], arguments...)
	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	value := &child{command: command, output: bufio.NewReader(stdout)}
	command.Stderr = &value.errors
	if err := command.Start(); err != nil {
		return nil, nil, err
	}
	line, err := value.output.ReadString('\n')
	if err != nil || !strings.HasPrefix(line, "READY ") {
		_ = value.kill()
		return nil, nil, errors.New("child role did not become ready")
	}
	return value, strings.Fields(strings.TrimSpace(strings.TrimPrefix(line, "READY "))), nil
}

func (value *child) wait() ([]byte, error) {
	tail, readErr := io.ReadAll(value.output)
	waitErr := value.command.Wait()
	if readErr != nil || waitErr != nil {
		return nil, fmt.Errorf("child role failed: %v %v %s", readErr, waitErr, value.errors.String())
	}
	for _, line := range strings.Split(string(tail), "\n") {
		if strings.HasPrefix(line, "RESULT ") {
			return []byte(strings.TrimPrefix(line, "RESULT ")), nil
		}
	}
	return nil, errors.New("child role returned no result")
}

func (value *child) kill() error {
	if value.command.Process != nil {
		_ = value.command.Process.Kill()
	}
	_, err := value.wait()
	return err
}

func encode(raw []byte) string { return base64.RawStdEncoding.EncodeToString(raw) }
func decode(raw string) []byte { value, _ := base64.RawStdEncoding.DecodeString(raw); return value }
func validCell(value string) bool {
	return value == "exact" || value == "target" || value == "replay-admission" || value == "node-substitution" || value == "expiry-substitution" || value == "wrong-key"
}
func print(value any) { raw, _ := json.Marshal(value); fmt.Printf("RESULT %s\n", raw) }
func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
