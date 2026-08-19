//go:build linux && live

package network_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/camouflage"
	"github.com/dianabuilds/ardents-network/internal/planfile"
	"github.com/dianabuilds/ardents-network/internal/serviceconn"
)

func runBlockedHostileLedger(t *testing.T) {
	cell := os.Getenv("ARDENTS_HOSTILE_CELL")
	parts := strings.Split(cell, "/")
	if len(parts) != 4 || parts[1] != "G9-ledger-leakage" {
		t.Fatal("hostile ledger cell is invalid")
	}
	prepareBlockedState(t, "bridge-network", "bridge-network")
	prepareBlockedState(t, "local-roles", "local-roles")
	runBlockedCommand(t, "/usr/local/bin/ardents-bridge", "import", "/run/secure/import.json")
	timeline := startBlockedTimeline(t)
	transition, err := os.ReadFile("/run/secure/transition.bin")
	if err != nil {
		t.Fatal(err)
	}
	transition = stampBlockedTransition(t, transition, timeline)
	var entry struct {
		RouteManifestDigest string `json:"route_manifest_digest"`
	}
	if err := planfile.Decode("/run/secure/entry.json", 32<<10, &entry); err != nil {
		t.Fatal(err)
	}
	var manifest [32]byte
	if err := planfile.FixedHex(entry.RouteManifestDigest, manifest[:]); err != nil {
		t.Fatal(err)
	}
	owner, closeOwner := openBlockedBridgeOwner(t, "/run/secure/import.json", time.Now)
	identity, candidate, ordinal, _, _, err := owner.BeginContact(transition, manifest, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	var before, after, receipt string
	if parts[2] == "ledger-reset-restart" {
		if err := closeOwner(); err != nil {
			t.Fatal(err)
		}
		owner, closeOwner = openBlockedBridgeOwner(t, "/run/secure/import.json", time.Now)
		before = blockedCurrentStateHash(t, "/run/state/bridge")
		_, _, _, _, _, restartErr := owner.BeginContact(transition, manifest, time.Now().Add(time.Minute))
		if restartErr == nil || !strings.Contains(restartErr.Error(), "bridge-interrupted") {
			t.Fatalf("restart reset the durable ledger: %v", restartErr)
		}
		after = blockedCurrentStateHash(t, "/run/state/bridge")
		receipt = "reopened-durable-owner:" + restartErr.Error()
	} else if strings.HasPrefix(parts[2], "candidate-leak-") {
		before = blockedCurrentStateHash(t, "/run/state/bridge")
		receipt = exerciseBlockedCandidateCanary(t, owner, parts[2], identity, candidate, transition, manifest)
		after = blockedCurrentStateHash(t, "/run/state/bridge")
	} else {
		before, after, receipt = exerciseBlockedLedgerVariant(t, owner, parts[2], transition, manifest, ordinal)
	}
	if err := closeOwner(); err != nil {
		t.Fatal(err)
	}
	result := blockedHostileLedgerResult{Kind: "hostile-ledger", Cell: cell,
		Terminal: "bridge-local-denial", Before: before, After: after, Receipt: receipt}
	writeBlockedJSON(t, hostileLedgerResultPath(blockedSync()), result)
	raw, _ := json.Marshal(result)
	fmt.Println(string(raw))
}

func exerciseBlockedLedgerVariant(t *testing.T, owner blockedBridgeOwner, variant string,
	transition []byte, manifest [32]byte, ordinal byte,
) (string, string, string) {
	t.Helper()
	var err error
	var before string
	switch variant {
	case "regime-oscillation":
		before = blockedCurrentStateHash(t, "/run/state/bridge")
		_, _, _, _, _, err = owner.BeginContact(transition, manifest, time.Now().Add(time.Minute))
	case "slot1-before-slot0", "retry-before-initial":
		before = blockedCurrentStateHash(t, "/run/state/bridge")
		_, _, _, err = owner.NextContact(context.Background())
	case "duplicate-ordinal":
		if finishErr := owner.FinishContact(ordinal, uint64(time.Second), false, true); finishErr != nil {
			t.Fatal(finishErr)
		}
		before = blockedCurrentStateHash(t, "/run/state/bridge")
		err = owner.FinishContact(ordinal, uint64(2*time.Second), false, true)
	case "ledger-reset-new-operation":
		if finishErr := owner.FinishContact(ordinal, uint64(time.Second), true, true); finishErr != nil {
			t.Fatal(finishErr)
		}
		operation := exerciseNewApplicationOperation(t)
		mutated := bytes.Clone(transition)
		mutated[len("ardents-h3-bridge-transition-v1")] ^= 1
		before = blockedCurrentStateHash(t, "/run/state/bridge")
		_, _, _, _, _, err = owner.BeginContact(mutated, manifest, time.Now().Add(time.Minute))
		if err != nil {
			err = fmt.Errorf("%s:%w", operation, err)
		}
	default:
		t.Fatalf("unsupported hostile ledger variant %q", variant)
	}
	if err == nil || !strings.Contains(err.Error(), "bridge-local-denial") {
		t.Fatalf("hostile ledger variant %s returned %v", variant, err)
	}
	after := blockedCurrentStateHash(t, "/run/state/bridge")
	return before, after, err.Error()
}

func exerciseNewApplicationOperation(t *testing.T) string {
	t.Helper()
	fixture := newHostileServiceFixture(t)
	client, err := serviceconn.New(serviceconn.Setup{NetworkID: fixture.network, BrokerID: [32]byte{6},
		AuthorityPublic: fixture.authorityPublic, IntroductionPublic: fixture.introductionPublic,
		ConnectionPrincipal: fixture.client})
	if err != nil {
		t.Fatal(err)
	}
	session := hostileAdmit(t, client, "connection", fixture.client, fixture.now)
	target := fixture.credential.Target
	target[0]++
	route, routePeer := net.Pipe()
	application, applicationPeer := net.Pipe()
	defer route.Close()
	defer routePeer.Close()
	defer application.Close()
	defer applicationPeer.Close()
	result, operationErr := client.Do(context.Background(), serviceconn.Request{Action: "connect",
		Principal: fixture.client, Session: session, Target: target, Publication: fixture.publication,
		Route: route, Application: application, BytesEachDirection: 1, At: fixture.now})
	if operationErr == nil || result.Class != "service target authentication failure" {
		t.Fatalf("new Application operation was not independently exercised: %+v %v", result, operationErr)
	}
	digest := sha256.Sum256([]byte(result.Class + operationErr.Error()))
	return "application-operation-sha256=" + hex.EncodeToString(digest[:])
}

func exerciseBlockedCandidateCanary(t *testing.T, owner blockedBridgeOwner, variant string, identity [32]byte,
	candidate, transition []byte, manifest [32]byte,
) string {
	t.Helper()
	canary, err := hex.DecodeString(os.Getenv("ARDENTS_FINAL_CANDIDATE_CANARY"))
	if err != nil || len(canary) != 32 {
		t.Fatal("candidate canary is unavailable")
	}
	mutated := mutateBlockedCandidate(t, candidate, variant, canary)
	config, validateErr := camouflage.Validate(mutated, identity)
	if (variant == "candidate-leak-path" || variant == "candidate-leak-certificate") && validateErr != nil {
		t.Fatalf("bound candidate canary was not exercised: %v", validateErr)
	}
	candidateErr := validateErr
	if validateErr == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		carrier, cleanup, _, openErr := camouflage.OpenClient(ctx, config, camouflage.Client{
			Binary: "/candidate/webtunnel-client", StateRoot: "/run/state/candidate-canary",
			Deadline: time.Now().Add(7 * time.Second)})
		cancel()
		if carrier != nil {
			_ = carrier.Close()
		}
		if cleanup != nil {
			openErr = errors.Join(openErr, cleanup())
		}
		if openErr == nil {
			t.Fatal("candidate canary unexpectedly opened a carrier")
		}
		candidateErr = openErr
	}
	if strings.Contains(fmt.Sprint(candidateErr), hex.EncodeToString(canary)) ||
		strings.Contains(fmt.Sprint(candidateErr), string(canary)) {
		t.Fatal("candidate parser leaked the injected canary")
	}
	_, _, _, _, _, denial := owner.BeginContact(transition, manifest, time.Now().Add(time.Minute))
	if denial == nil || !strings.Contains(denial.Error(), "bridge-local-denial") {
		t.Fatalf("candidate canary operation returned %v", denial)
	}
	digest := sha256.Sum256(mutated)
	errorDigest := sha256.Sum256([]byte(fmt.Sprint(candidateErr)))
	return "candidate-input-sha256:" + hex.EncodeToString(digest[:]) +
		":candidate-diagnostic-sha256:" + hex.EncodeToString(errorDigest[:]) + ":" + denial.Error()
}

func mutateBlockedCandidate(t *testing.T, candidate []byte, variant string, canary []byte) []byte {
	t.Helper()
	mutated := bytes.Clone(candidate)
	const magic = "ardents-h3-wt1"
	if len(mutated) < len(magic)+2 || string(mutated[:len(magic)]) != magic {
		t.Fatal("candidate envelope is invalid")
	}
	profileLength := int(mutated[len(magic)+1])
	address := len(magic) + 2 + profileLength
	pathLength := address + 4 + 2
	if pathLength+2 > len(mutated) {
		t.Fatal("candidate envelope path is unavailable")
	}
	switch variant {
	case "candidate-leak-invite":
		return append(mutated, canary...)
	case "candidate-leak-address":
		copy(mutated[address:address+4], canary[:4])
		return append(mutated, canary[4:]...)
	case "candidate-leak-path":
		oldLength := int(binary.BigEndian.Uint16(mutated[pathLength : pathLength+2]))
		end := pathLength + 2 + oldLength
		path := []byte("/" + hex.EncodeToString(canary))
		result := append(bytes.Clone(mutated[:pathLength]), byte(len(path)>>8), byte(len(path)))
		result = append(result, path...)
		return append(result, mutated[end:]...)
	case "candidate-leak-certificate":
		copy(mutated[len(mutated)-32:], canary)
		return mutated
	default:
		t.Fatalf("unsupported candidate canary variant %q", variant)
		return nil
	}
}

func blockedCurrentStateHash(t *testing.T, root string) string {
	t.Helper()
	pointer, err := os.ReadFile(root + "/current")
	if err != nil {
		t.Fatal(err)
	}
	name := strings.TrimSpace(string(pointer))
	raw, err := os.ReadFile(root + "/state-" + name)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	if hex.EncodeToString(digest[:]) != name {
		t.Fatal(errors.New("hostile ledger state commitment mismatch"))
	}
	return name
}
