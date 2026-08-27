//go:build h4_4_firefox

package endpoint

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/browserentry"
	"github.com/dianabuilds/ardents-network/internal/endpoint/reference"
	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
)

func TestFirefoxBrowserDemandResolvesAlphaOriginThroughEndpointProxyQualification(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Fatal("alpha Browser Entry qualification requires Windows")
	}
	executable := requiredFirefoxExecutable(t)
	now := time.Now().UTC().Truncate(time.Second)
	network, target := targetLinkBytes(71), targetLinkBytes(72)
	authorityPublic, authorityPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	link, err := alpha.ParseServiceLink("ardents-alpha://reference")
	if err != nil {
		t.Fatal(err)
	}
	rawCorpus, err := alpha.IssueCorpus(alpha.CorpusInput{Cohort: "firefox-demand-qualification", Network: network, Serial: 1,
		NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Minute), Bindings: []alpha.BindingInput{{Link: link, Target: target}}}, authorityPrivate)
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := alpha.OpenCorpus(authorityPublic, rawCorpus)
	if err != nil {
		t.Fatal(err)
	}
	floor, err := alpha.OpenPersistentFloor(alpha.PersistentFloorConfig{Root: t.TempDir(), Authority: authorityPublic,
		Cohort: "firefox-demand-qualification", Network: network})
	if err != nil {
		t.Fatal(err)
	}
	defer floor.Close()
	proofPath := filepath.Join(t.TempDir(), "firefox-reference-resources-proof")
	fetcher := &firefoxNamedFetcher{observed: make(chan string, 3), proofPath: proofPath, remaining: map[string]bool{
		"/": true, "/site.css": true, "/mark.svg": true,
	}}
	statePath := filepath.Join(t.TempDir(), "firefox-browser-entry.json")
	entryState, err := browserentry.OpenPublisher(statePath)
	if err != nil {
		t.Fatal(err)
	}
	defer entryState.Close()
	endpoint := &endpoint{network: network, browserEntry: entryState}
	if err := floor.Observe(corpus); err != nil {
		t.Fatal(err)
	}
	resolution, err := endpoint.OpenAlphaBrowserResolution(t.Context(), AlphaBrowserResolutionRequest{Floor: floor,
		Clock: func() time.Time { return now }, Open: func(_ context.Context, binding alpha.Binding) (AlphaBrowserSite, error) {
			if binding.Link() != link || binding.Target() != target {
				return nil, errors.New("Firefox demand resolver opened a different alpha binding")
			}
			origin, openErr := reference.OpenLive(reference.LiveConfig{Target: target,
				Routes: map[string]string{"": "/", "site.css": "/site.css", "mark.svg": "/mark.svg"}, Fetcher: fetcher})
			if openErr != nil {
				return nil, openErr
			}
			proxyURL, release, routeErr := endpoint.openAlphaBrowserRoute("reference.ard", origin)
			if routeErr != nil {
				_ = origin.Close()
				return nil, routeErr
			}
			site := &firefoxDemandResolutionSite{ready: make(chan ReferenceReady, 1), done: make(chan ReferenceOutcome), close: func() error {
				release()
				return origin.Close()
			}}
			site.ready <- ReferenceReady{URL: "http://reference.ard/", AlphaProxyURL: proxyURL, AuthenticatedTarget: target}
			close(site.ready)
			return site, nil
		}})
	if err != nil {
		t.Fatal(err)
	}
	defer resolution.Close()
	endpoint.alphaBrowserMu.Lock()
	proxyURL := endpoint.alphaBrowserProxy.URL()
	endpoint.alphaBrowserMu.Unlock()
	proxy, err := url.Parse(proxyURL)
	if err != nil || proxy.Scheme != "http" || proxy.Hostname() != "127.0.0.1" || proxy.Port() == "" {
		t.Fatalf("Endpoint alpha proxy URL = %q, parse error = %v", proxyURL, err)
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	nativeHost := filepath.Join(t.TempDir(), "ardents-browser-entry.exe")
	build := exec.CommandContext(t.Context(), "go", "build", "-o", nativeHost, "./cmd/ardents-browser-entry")
	build.Dir = root
	if output, buildErr := build.CombinedOutput(); buildErr != nil {
		t.Fatalf("build maintained Browser Entry native host: %v\n%s", buildErr, output)
	}
	script := filepath.Join(root, "tests", "qualification", "h4-4a-firefox", "run-maintained-browser-entry.ps1")
	addonSource := filepath.Join(root, "packaging", "firefox-alpha-browser-entry")
	command := exec.CommandContext(t.Context(), "powershell.exe",
		"-NoProfile", "-ExecutionPolicy", "Bypass", "-File", script,
		"-FirefoxPath", executable,
		"-AddonSource", addonSource,
		"-NativeHostPath", nativeHost,
		"-NativeStatePath", statePath,
		"-ReferenceProofPath", proofPath,
		"-TimeoutSeconds", "30",
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("run alpha Firefox Browser Entry qualification: %v\n%s", err, output)
	}
	waitForFirefoxResources(t, fetcher)
}

func waitForFirefoxResources(t *testing.T, fetcher *firefoxNamedFetcher) {
	t.Helper()
	want := map[string]bool{"/": true, "/site.css": true, "/mark.svg": true}
	declared := map[string]bool{"/": true, "/site.css": true, "/mark.svg": true}
	deadline := time.NewTimer(15 * time.Second)
	defer deadline.Stop()
	for len(want) != 0 {
		select {
		case path := <-fetcher.observed:
			if !declared[path] {
				t.Fatalf("Firefox requested unexpected named Reference resource %q", path)
			}
			delete(want, path)
		case <-deadline.C:
			t.Fatalf("Firefox did not request named Reference resources: missing=%v observed=%v", want, fetcher.Paths())
		}
	}
}

func requiredFirefoxExecutable(t *testing.T) string {
	t.Helper()
	executable := os.Getenv("ARDENTS_REFERENCE_C2_FIREFOX")
	if executable == "" {
		t.Fatal("ARDENTS_REFERENCE_C2_FIREFOX is required for the selected Firefox qualification")
	}
	return executable
}

type firefoxDemandResolutionSite struct {
	ready chan ReferenceReady
	done  chan ReferenceOutcome
	close func() error
	once  sync.Once
	err   error
}

func (site *firefoxDemandResolutionSite) Ready() <-chan ReferenceReady { return site.ready }

func (site *firefoxDemandResolutionSite) Done() <-chan ReferenceOutcome { return site.done }

func (site *firefoxDemandResolutionSite) Close() error {
	site.once.Do(func() {
		if site.close != nil {
			site.err = site.close()
		}
		close(site.done)
	})
	return site.err
}

type firefoxNamedFetcher struct {
	observed  chan string
	mu        sync.Mutex
	paths     []string
	proofPath string
	remaining map[string]bool
}

func (fetcher *firefoxNamedFetcher) Fetch(_ context.Context, request reference.Request) (reference.Response, error) {
	fetcher.mu.Lock()
	fetcher.paths = append(fetcher.paths, request.Path)
	delete(fetcher.remaining, request.Path)
	completed := len(fetcher.remaining) == 0 && fetcher.proofPath != ""
	proofPath := fetcher.proofPath
	fetcher.mu.Unlock()
	if completed {
		if err := os.WriteFile(proofPath, []byte("firefox-reference-resources\n"), 0o600); err != nil {
			return reference.Response{}, err
		}
	}
	select {
	case fetcher.observed <- request.Path:
	default:
	}
	switch request.Path {
	case "/":
		return reference.Response{ContentType: "text/html", Body: []byte("<!doctype html><link rel=\"stylesheet\" href=\"site.css\"><img src=\"mark.svg\">")}, nil
	case "/site.css":
		return reference.Response{ContentType: "text/css", Body: []byte("body{color:#252525}")}, nil
	case "/mark.svg":
		return reference.Response{ContentType: "image/svg+xml", Body: []byte("<svg xmlns=\"http://www.w3.org/2000/svg\"/>")}, nil
	default:
		return reference.Response{}, errors.New("unexpected named Reference resource")
	}
}

func (fetcher *firefoxNamedFetcher) Paths() []string {
	fetcher.mu.Lock()
	defer fetcher.mu.Unlock()
	return append([]string(nil), fetcher.paths...)
}
