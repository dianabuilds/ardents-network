package endpoint_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/browserentry"
	endpointapi "github.com/dianabuilds/ardents-network/internal/endpoint"
	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

func TestReferenceConnectionServesDeclaredStaticRouteAfterTargetAuthentication(t *testing.T) {
	fixture := newFixture(t)
	publisher := newPublisher(t, fixture)
	publication := publish(t, publisher, fixture, fixture.first, fixture.firstPrivate)
	client, err := endpointapi.New(endpointapi.Setup{NetworkID: fixture.networkID, BrokerID: [32]byte{8},
		AuthorityPublic: fixture.authorityPublic, IntroductionPublic: fixture.introductionPublic,
		ConnectionPrincipal: fixture.clientPrincipal})
	if err != nil {
		t.Fatal(err)
	}
	link, err := targetlink.Encode(targetlink.Link{Network: fixture.networkID, Target: fixture.first.Target})
	if err != nil {
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
	requestSeen := make(chan *http.Request, 1)
	go serveOneStaticReference(publisherApplication, requestSeen)
	clientSession := admit(t, client, "connection", fixture.clientPrincipal, fixture.now)
	browserOpened := make(chan endpointapi.ReferenceReady, 1)
	running, err := client.StartReferenceConnection(ctx, endpointapi.ReferenceConnectionRequest{
		TargetLink: link,
		Routes:     map[string]string{"": "/"},
		Connection: endpointapi.OutboundConnectionRequest{Principal: fixture.clientPrincipal, Capability: clientSession,
			Target: fixture.first.Target, Publication: publication, Route: clientRoute, BytesEachDirection: 64 << 10, At: fixture.now},
		Browser: referenceBrowserFunc(func(_ context.Context, opened endpointapi.ReferenceReady) error {
			browserOpened <- opened
			return nil
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	ready, ok := <-running.Ready()
	if !ok || ready.URL == "" || ready.AuthenticatedTarget != fixture.first.Target {
		t.Fatalf("Reference origin was not published after exact Target authentication: %+v", ready)
	}
	if opened := <-browserOpened; opened != ready {
		t.Fatalf("browser received a different Reference origin: got=%+v want=%+v", opened, ready)
	}
	httpClient := &http.Client{Transport: &http.Transport{Proxy: nil}}
	response, err := httpClient.Get(ready.URL)
	if err != nil {
		t.Fatal(err)
	}
	body, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if readErr != nil || response.StatusCode != http.StatusOK || string(body) != "<h1>Reference</h1>" {
		t.Fatalf("browser response = %d %q %v", response.StatusCode, body, readErr)
	}
	remoteRequest := <-requestSeen
	if remoteRequest == nil || remoteRequest.Method != http.MethodGet || remoteRequest.URL.Path != "/" ||
		remoteRequest.Host != "reference" || remoteRequest.Header.Get("Cookie") != "" {
		t.Fatalf("remote static request was not the closed profile: %#v", remoteRequest)
	}
	if err := running.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-running.Done():
	case <-time.After(time.Second):
		t.Fatal("Reference Connection did not withdraw after local close")
	}
	select {
	case <-publisherDone:
	case <-time.After(time.Second):
		t.Fatal("publisher Service Connection did not terminate after client close")
	}
}

func TestReferenceConnectionNeverLaunchesBrowserForInvalidTargetLink(t *testing.T) {
	fixture := newFixture(t)
	client, err := endpointapi.New(endpointapi.Setup{NetworkID: fixture.networkID, BrokerID: [32]byte{8},
		AuthorityPublic: fixture.authorityPublic, IntroductionPublic: fixture.introductionPublic,
		ConnectionPrincipal: fixture.clientPrincipal})
	if err != nil {
		t.Fatal(err)
	}
	opened := false
	_, err = client.StartReferenceConnection(t.Context(), endpointapi.ReferenceConnectionRequest{
		TargetLink: "not-an-ardents-target-link",
		Browser: referenceBrowserFunc(func(context.Context, endpointapi.ReferenceReady) error {
			opened = true
			return nil
		}),
	})
	if err == nil || opened {
		t.Fatalf("invalid Target Link browser launch: err=%v opened=%t", err, opened)
	}
}

func TestAlphaReferenceConnectionPresentsOnlyTheVerifiedBindingAtNamedHTTPOrigin(t *testing.T) {
	fixture := newFixture(t)
	publisher := newPublisher(t, fixture)
	publication := publish(t, publisher, fixture, fixture.first, fixture.firstPrivate)
	entryStatePath := filepath.Join(t.TempDir(), "firefox-browser-entry.json")
	client, err := endpointapi.New(endpointapi.Setup{NetworkID: fixture.networkID, BrokerID: [32]byte{8},
		AuthorityPublic: fixture.authorityPublic, IntroductionPublic: fixture.introductionPublic,
		ConnectionPrincipal: fixture.clientPrincipal, BrowserEntryStatePath: entryStatePath})
	if err != nil {
		t.Fatal(err)
	}
	binding := issuedAlphaBinding(t, fixture.networkID, fixture.first.Target)
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
	requestSeen := make(chan *http.Request, 1)
	go serveOneStaticReference(publisherApplication, requestSeen)
	clientSession := admit(t, client, "connection", fixture.clientPrincipal, fixture.now)
	browserOpened := make(chan endpointapi.ReferenceReady, 1)
	running, err := client.StartAlphaReferenceConnection(ctx, endpointapi.AlphaReferenceConnectionRequest{
		Binding: binding, Routes: map[string]string{"": "/"},
		Connection: endpointapi.OutboundConnectionRequest{Principal: fixture.clientPrincipal, Capability: clientSession,
			Target: fixture.first.Target, Publication: publication, Route: clientRoute, BytesEachDirection: 64 << 10, At: fixture.now},
		Browser: referenceBrowserFunc(func(_ context.Context, opened endpointapi.ReferenceReady) error {
			browserOpened <- opened
			return nil
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	ready, ok := <-running.Ready()
	if !ok || ready.URL != "http://blog.alice.ard/" || ready.AlphaProxyURL == "" || ready.AuthenticatedTarget != fixture.first.Target {
		t.Fatalf("named Reference origin was not published after exact Target authentication: %+v", ready)
	}
	if opened := <-browserOpened; opened != ready {
		t.Fatalf("browser received a different named Reference origin: got=%+v want=%+v", opened, ready)
	}
	proxyURL, parseErr := url.Parse(ready.AlphaProxyURL)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	if port := nativeBrowserEntryPort(t, entryStatePath); port != uint16(portNumber(t, proxyURL)) {
		t.Fatalf("Firefox native-host port = %d, want active alpha proxy %s", port, proxyURL.Host)
	}
	proxyAuthentication, authenticationErr := nativeBrowserEntryProxyAuthentication(entryStatePath)
	if authenticationErr != nil || proxyAuthentication.Port != uint16(portNumber(t, proxyURL)) {
		t.Fatalf("Firefox native-host authentication = %+v, error = %v", proxyAuthentication, authenticationErr)
	}
	proxyURL.User = url.UserPassword(browserentry.ProxyUsername, proxyAuthentication.Password)
	clientHTTP := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
	response, err := clientHTTP.Get(ready.URL)
	if err != nil {
		t.Fatal(err)
	}
	body, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if readErr != nil || response.StatusCode != http.StatusOK || string(body) != "<h1>Reference</h1>" {
		t.Fatalf("named browser response = %d %q %v", response.StatusCode, body, readErr)
	}
	remoteRequest := <-requestSeen
	if remoteRequest == nil || remoteRequest.Method != http.MethodGet || remoteRequest.URL.Path != "/" || remoteRequest.Host != "reference" {
		t.Fatalf("named remote static request was not the closed profile: %#v", remoteRequest)
	}
	if err := running.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-running.Done():
	case <-time.After(time.Second):
		t.Fatal("named Reference Connection did not withdraw after local close")
	}
	if _, err := nativeBrowserEntryPortResult(entryStatePath); err == nil {
		t.Fatal("Firefox native host retained the withdrawn alpha proxy")
	}
	select {
	case <-publisherDone:
	case <-time.After(time.Second):
		t.Fatal("publisher Service Connection did not terminate after named client close")
	}
}

func nativeBrowserEntryPort(t *testing.T, statePath string) uint16 {
	t.Helper()
	port, err := nativeBrowserEntryPortResult(statePath)
	if err != nil {
		t.Fatal(err)
	}
	return port
}

func nativeBrowserEntryPortResult(statePath string) (uint16, error) {
	response, err := nativeBrowserEntryResult(statePath, browserentry.OperationLoopbackProxyPort)
	if err != nil {
		return 0, err
	}
	var result struct {
		Port uint16 `json:"port"`
	}
	if err := json.Unmarshal(response, &result); err != nil {
		return 0, err
	}
	return result.Port, nil
}

type nativeBrowserEntryAuthentication struct {
	Port     uint16 `json:"port"`
	Password string `json:"password"`
}

func nativeBrowserEntryProxyAuthentication(statePath string) (nativeBrowserEntryAuthentication, error) {
	response, err := nativeBrowserEntryResult(statePath, browserentry.OperationLoopbackProxyAuthentication)
	if err != nil {
		return nativeBrowserEntryAuthentication{}, err
	}
	var result nativeBrowserEntryAuthentication
	if err := json.Unmarshal(response, &result); err != nil {
		return nativeBrowserEntryAuthentication{}, err
	}
	return result, nil
}

func nativeBrowserEntryResult(statePath, operation string) ([]byte, error) {
	requestBody, err := json.Marshal(struct {
		Operation string `json:"operation"`
	}{Operation: operation})
	if err != nil {
		return nil, err
	}
	var request bytes.Buffer
	var length [4]byte
	binary.LittleEndian.PutUint32(length[:], uint32(len(requestBody)))
	_, _ = request.Write(length[:])
	_, _ = request.Write(requestBody)
	var response bytes.Buffer
	if err := browserentry.ServeNativeHost(&request, &response, statePath); err != nil {
		return nil, err
	}
	if _, err := io.ReadFull(&response, length[:]); err != nil {
		return nil, err
	}
	body := make([]byte, binary.LittleEndian.Uint32(length[:]))
	if _, err := io.ReadFull(&response, body); err != nil {
		return nil, err
	}
	return body, nil
}

func portNumber(t *testing.T, value *url.URL) int {
	t.Helper()
	_, portText, err := net.SplitHostPort(value.Host)
	if err != nil {
		t.Fatal(err)
	}
	var port int
	if _, err := fmt.Sscan(portText, &port); err != nil {
		t.Fatal(err)
	}
	return port
}

func TestAlphaTransparentConnectionCarriesOneDynamicPublisherRequest(t *testing.T) {
	fixture := newFixture(t)
	publisher := newPublisher(t, fixture)
	publication := publish(t, publisher, fixture, fixture.first, fixture.firstPrivate)
	client, err := endpointapi.New(endpointapi.Setup{NetworkID: fixture.networkID, BrokerID: [32]byte{8},
		AuthorityPublic: fixture.authorityPublic, IntroductionPublic: fixture.introductionPublic,
		ConnectionPrincipal: fixture.clientPrincipal})
	if err != nil {
		t.Fatal(err)
	}
	binding := issuedAlphaBinding(t, fixture.networkID, fixture.first.Target)
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
	browserOpened := make(chan endpointapi.ReferenceReady, 1)
	running, err := client.StartAlphaTransparentConnection(ctx, endpointapi.AlphaTransparentConnectionRequest{Binding: binding,
		Connection: endpointapi.OutboundConnectionRequest{Principal: fixture.clientPrincipal, Capability: clientSession,
			Target: fixture.first.Target, Publication: publication, Route: clientRoute, BytesEachDirection: 64 << 10, At: fixture.now},
		Browser: referenceBrowserFunc(func(_ context.Context, opened endpointapi.ReferenceReady) error {
			browserOpened <- opened
			return nil
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	ready, ok := <-running.Ready()
	if !ok || ready.URL != "http://blog.alice.ard/" || ready.AlphaProxyURL == "" || ready.AuthenticatedTarget != fixture.first.Target {
		t.Fatalf("transparent alpha origin was not published after target authentication: %+v", ready)
	}
	if opened := <-browserOpened; opened != ready {
		t.Fatalf("browser received a different transparent alpha origin: got=%+v want=%+v", opened, ready)
	}
	proxyURL, err := url.Parse(ready.AlphaProxyURL)
	if err != nil {
		t.Fatal(err)
	}
	httpClient := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL), DisableCompression: true}}
	request, err := http.NewRequest(http.MethodPost, ready.URL+"publish?draft=1", bytes.NewBufferString("post=ready"))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Cookie", "session=local")
	response, err := httpClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	body, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if readErr != nil || response.StatusCode != http.StatusCreated || response.Header.Get("Set-Cookie") != "saved=true; Path=/" || string(body) != "published" {
		t.Fatalf("transparent browser response = %d %#v %q %v", response.StatusCode, response.Header, body, readErr)
	}
	remoteRequest := <-publisherRequest
	if remoteRequest == nil || remoteRequest.Method != http.MethodPost || remoteRequest.URL.String() != "/publish?draft=1" ||
		remoteRequest.Host != "blog.alice.ard" || remoteRequest.Header.Get("Cookie") != "session=local" {
		t.Fatalf("Publisher received a changed transparent request: %#v", remoteRequest)
	}
	if err := running.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-running.Done():
	case <-time.After(time.Second):
		t.Fatal("transparent alpha Connection did not withdraw after local close")
	}
	select {
	case <-publisherDone:
	case <-time.After(time.Second):
		t.Fatal("Publisher transparent Service Connection did not terminate after client close")
	}
}

type referenceBrowserFunc func(context.Context, endpointapi.ReferenceReady) error

func (open referenceBrowserFunc) OpenReference(ctx context.Context, ready endpointapi.ReferenceReady) error {
	return open(ctx, ready)
}

func serveOneStaticReference(connection net.Conn, seen chan<- *http.Request) {
	request, err := http.ReadRequest(bufio.NewReader(connection))
	if err != nil {
		seen <- nil
		return
	}
	seen <- request
	_, _ = io.WriteString(connection, "HTTP/1.1 200 OK\r\nContent-Type: text/html\r\nContent-Length: 18\r\nConnection: close\r\n\r\n<h1>Reference</h1>")
	_ = connection.Close()
}

func serveOneTransparentReference(connection net.Conn, seen chan<- *http.Request) {
	request, err := http.ReadRequest(bufio.NewReader(connection))
	if err != nil {
		seen <- nil
		return
	}
	body, readErr := io.ReadAll(request.Body)
	_ = request.Body.Close()
	if readErr != nil || string(body) != "post=ready" || request.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
		seen <- nil
		return
	}
	seen <- request
	_, _ = io.WriteString(connection, "HTTP/1.1 201 Created\r\nContent-Type: text/plain\r\nSet-Cookie: saved=true; Path=/\r\nContent-Length: 9\r\n\r\npublished")
	_ = connection.Close()
}

func issuedAlphaBinding(t *testing.T, network, target [32]byte) alpha.Binding {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	link, err := alpha.ParseServiceLink("ardents-alpha://blog.alice")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.August, 25, 12, 0, 0, 0, time.UTC)
	raw, err := alpha.IssueCorpus(alpha.CorpusInput{Cohort: "closed-alpha-1", Network: network, Serial: 1,
		Bindings: []alpha.BindingInput{{Link: link, Target: target}}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour)}, private)
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := alpha.OpenCorpus(public, raw)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := corpus.Resolve(link, now)
	if err != nil {
		t.Fatal(err)
	}
	return binding
}
