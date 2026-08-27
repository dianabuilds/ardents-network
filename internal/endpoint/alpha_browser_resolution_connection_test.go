package endpoint_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/browserentry"
	endpointapi "github.com/dianabuilds/ardents-network/internal/endpoint"
	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
)

func TestAlphaBrowserResolutionOpensAServiceFromTheTypedName(t *testing.T) {
	fixture := newFixture(t)
	publisher := newPublisher(t, fixture)
	publication := publish(t, publisher, fixture, fixture.first, fixture.firstPrivate)
	statePath := filepath.Join(t.TempDir(), "browser-entry.json")
	client, err := endpointapi.New(endpointapi.Setup{NetworkID: fixture.networkID, BrokerID: [32]byte{8},
		AuthorityPublic: fixture.authorityPublic, IntroductionPublic: fixture.introductionPublic,
		ConnectionPrincipal: fixture.clientPrincipal, BrowserEntryStatePath: statePath})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	link, err := alpha.ParseServiceLink("ardents-alpha://blog.alice")
	if err != nil {
		t.Fatal(err)
	}
	corpusPublic, corpusPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := alpha.IssueCorpus(alpha.CorpusInput{Cohort: "typed-browser-name", Network: fixture.networkID, Serial: 1,
		NotBefore: fixture.now.Add(-time.Minute), NotAfter: fixture.now.Add(time.Minute),
		Bindings: []alpha.BindingInput{{Link: link, Target: fixture.first.Target}}}, corpusPrivate)
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := alpha.OpenCorpus(corpusPublic, raw)
	if err != nil {
		t.Fatal(err)
	}
	floor, err := alpha.OpenPersistentFloor(alpha.PersistentFloorConfig{Root: t.TempDir(), Authority: corpusPublic,
		Cohort: "typed-browser-name", Network: fixture.networkID})
	if err != nil {
		t.Fatal(err)
	}
	defer floor.Close()
	if err := floor.Observe(corpus); err != nil {
		t.Fatal(err)
	}
	clientRoute, publisherRoute := net.Pipe()
	publisherEndpoint, publisherApplication := net.Pipe()
	defer publisherApplication.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	publisherDone := make(chan serviceOutcome, 1)
	publisherSession := admit(t, publisher, "connection", fixture.publisherPrincipal, fixture.now)
	go func() {
		result, runErr := publisher.Accept(ctx, endpointapi.InboundConnectionRequest{
			Principal: fixture.publisherPrincipal, Capability: publisherSession, Route: publisherRoute,
			Application: publisherEndpoint, BytesEachDirection: 64 << 10, At: fixture.now,
		})
		publisherDone <- serviceOutcome{result, runErr}
	}()
	publisherRequest := make(chan *http.Request, 1)
	go serveOneTransparentReference(publisherApplication, publisherRequest)
	clientSession := admit(t, client, "connection", fixture.clientPrincipal, fixture.now)
	opened := make(chan *endpointapi.ReferenceConnection, 1)
	resolution, err := client.OpenAlphaBrowserResolution(ctx, endpointapi.AlphaBrowserResolutionRequest{Floor: floor,
		Clock: func() time.Time { return fixture.now }, Open: func(openCtx context.Context, binding alpha.Binding) (endpointapi.AlphaBrowserSite, error) {
			if binding.Link() != link || binding.Target() != fixture.first.Target {
				return nil, errors.New("typed browser resolver selected the wrong alpha binding")
			}
			running, openErr := client.StartAlphaTransparentConnection(openCtx, endpointapi.AlphaTransparentConnectionRequest{Binding: binding,
				Connection: endpointapi.OutboundConnectionRequest{Principal: fixture.clientPrincipal, Capability: clientSession,
					Target: fixture.first.Target, Publication: publication, Route: clientRoute, BytesEachDirection: 64 << 10, At: fixture.now}})
			if openErr == nil {
				opened <- running
			}
			return running, openErr
		}})
	if err != nil {
		t.Fatal(err)
	}
	defer resolution.Close()
	port, err := nativeBrowserEntryPortResult(statePath)
	if err != nil {
		t.Fatal(err)
	}
	authentication, err := nativeBrowserEntryProxyAuthentication(statePath)
	if err != nil || authentication.Port != port {
		t.Fatalf("Browser Entry alpha proxy authentication = %+v, %v", authentication, err)
	}
	proxy, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", port))
	if err != nil {
		t.Fatal(err)
	}
	proxy.User = url.UserPassword(browserentry.ProxyUsername, authentication.Password)
	browser := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxy), DisableCompression: true}}
	request, err := http.NewRequest(http.MethodPost, "http://blog.alice.ard/publish?draft=1", bytes.NewBufferString("post=ready"))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Cookie", "session=typed-name")
	response, err := browser.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	body, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if readErr != nil || response.StatusCode != http.StatusCreated || response.Header.Get("Set-Cookie") != "saved=true; Path=/" || string(body) != "published" {
		t.Fatalf("typed alpha browser response = %d %#v %q %v", response.StatusCode, response.Header, body, readErr)
	}
	running := <-opened
	if running == nil {
		t.Fatal("typed alpha browser did not open a Service Connection")
	}
	remoteRequest := <-publisherRequest
	if remoteRequest == nil || remoteRequest.Host != "blog.alice.ard" || remoteRequest.URL.String() != "/publish?draft=1" ||
		remoteRequest.Header.Get("Cookie") != "session=typed-name" {
		t.Fatalf("Publisher received a changed typed-name request: %#v", remoteRequest)
	}
	if err := resolution.Close(); err != nil {
		t.Fatal(err)
	}
	if _, stateErr := os.Stat(statePath); !errors.Is(stateErr, os.ErrNotExist) {
		t.Fatalf("closed typed-name resolver retained Browser Entry state: %v", stateErr)
	}
	select {
	case <-running.Done():
	case <-time.After(time.Second):
		t.Fatal("typed alpha browser Service Connection did not withdraw")
	}
	select {
	case <-publisherDone:
	case <-time.After(time.Second):
		t.Fatal("typed alpha browser Publisher connection did not terminate")
	}
}
