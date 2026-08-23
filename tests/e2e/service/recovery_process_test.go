package service_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	serviceconn "github.com/dianabuilds/ardents-network/internal/endpoint"
)

func TestServiceProcessesKeepConnectionWhenReplacementFails(t *testing.T) {
	fixture := newRecoveryProcessFixture(t)
	serviceBinary := buildProductCommand(t, "ardents")
	publishBinary := buildE2EFixtureCommand(t, "publish-app")
	streamBinary := buildE2EFixtureCommand(t, "stream-app")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	publisher := startServiceProcess(t, ctx, serviceBinary, fixture.root, fixture.publisherPlan)
	runCommand(t, ctx, fixture.root, publishBinary, "publish", fixture.administration)
	client := startServiceProcess(t, ctx, serviceBinary, fixture.root, fixture.clientPlan)

	type routeOutcome struct {
		observation replacementRouteObservation
		err         error
	}
	routeResult := make(chan routeOutcome, 1)
	go func() {
		observation, err := runReplacementRoute(ctx, fixture.clientRoute, fixture.publisherRoute)
		routeResult <- routeOutcome{observation: observation, err: err}
	}()
	publisherApp := startCommand(ctx, fixture.root, streamBinary, "run", "publisher", fixture.publisherApplication,
		fixture.publisherSeed, fixture.clientSeed, "0", "8388608")
	clientApp := startCommand(ctx, fixture.root, streamBinary, "run", "client", fixture.clientApplication,
		fixture.clientSeed, fixture.publisherSeed, "8388608", "0")

	clientApplication := decodeApplicationResult(t, <-clientApp)
	publisherApplication := decodeApplicationResult(t, <-publisherApp)
	recovery := <-routeResult
	if recovery.err != nil {
		t.Fatalf("replacement Route boundary failed: %v", recovery.err)
	}
	if overlap := recovery.observation.secondInterruption.Sub(recovery.observation.firstInterruption); overlap > time.Second || overlap < 0 {
		t.Fatalf("replacement overlap was outside one second: %s", overlap)
	}
	if elapsed := recovery.observation.finished.Sub(recovery.observation.firstInterruption); elapsed > 8*time.Second || elapsed < 0 {
		t.Fatalf("same connection did not finish its recovery episode within eight seconds: %s", elapsed)
	}
	const logicalCarrierBaseline = int64(8 << 20)
	additionalCarrier := max(int64(0), recovery.observation.carrierBytes-logicalCarrierBaseline)
	if additionalCarrier > 8<<20 {
		t.Fatalf("one recovery episode added more than 8 MiB of carrier traffic: total=%d additional=%d",
			recovery.observation.carrierBytes, additionalCarrier)
	}
	var clientResult, publisherResult serviceconn.RuntimeResult
	client.finish(t, &clientResult)
	publisher.finish(t, &publisherResult)

	for role, result := range map[string]serviceconn.RuntimeResult{"client": clientResult, "publisher": publisherResult} {
		if result.Class != "clean service connection close" || result.AuthenticatedTarget != fixture.target ||
			result.RouteGeneration != 2 || result.RecoveryCount != 1 || result.RouteAttachmentsAccepted != 3 ||
			result.ApplicationIPCAccepts != 1 || result.AcceptedBytes != result.AcknowledgedBytes ||
			result.QueueHighWater > 256<<10 {
			t.Fatalf("%s did not retain one recovered Service Connection: %+v", role, result)
		}
	}
	if clientApplication.Terminal != "success" || publisherApplication.Terminal != "success" ||
		clientApplication.SentDigest != publisherApplication.ReceivedDigest ||
		publisherApplication.ReceivedBytes != 8<<20 || clientApplication.ResultClass != "clean service connection close" ||
		publisherApplication.ResultClass != "clean service connection close" {
		t.Fatalf("Application bytes or terminal result changed: client=%+v publisher=%+v",
			clientApplication, publisherApplication)
	}
	for _, path := range []string{fixture.clientApplication,
		fixture.clientApplication + ".result", fixture.publisherApplication,
		fixture.publisherApplication + ".result", fixture.clientRoute,
		fixture.publisherRoute, fixture.administration} {
		if pathExists(path) {
			t.Fatalf("process-owned socket remained after completion: %s", path)
		}
	}
}

type applicationObservation struct {
	Schema              string   `json:"schema"`
	Terminal            string   `json:"terminal"`
	SentDigest          [32]byte `json:"sent_digest"`
	ReceivedDigest      [32]byte `json:"received_digest"`
	ReceivedBytes       uint32   `json:"received_bytes"`
	ResultClass         string   `json:"result_class"`
	AuthenticatedTarget [32]byte `json:"authenticated_target"`
}

func decodeApplicationResult(t *testing.T, result commandResult) applicationObservation {
	t.Helper()
	if result.err != nil {
		t.Fatalf("Application process failed: %v\n%s", result.err, result.output)
	}
	var value applicationObservation
	for _, line := range bytes.Split(result.output, []byte{'\n'}) {
		var candidate applicationObservation
		if json.Unmarshal(bytes.TrimSpace(line), &candidate) == nil && candidate.Schema != "" {
			value = candidate
		}
	}
	if value.Schema == "" {
		t.Fatalf("Application result is absent:\n%s", result.output)
	}
	return value
}
