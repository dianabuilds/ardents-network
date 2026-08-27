package endpoint

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/endpoint/reference"
	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
)

func TestAlphaBrowserResolutionOpensOnlyTheAcceptedNamedBinding(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	network, target := targetLinkBytes(1), targetLinkBytes(33)
	authorityPublic, authorityPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	link, err := alpha.ParseServiceLink("ardents-alpha://blog.alice")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := alpha.IssueCorpus(alpha.CorpusInput{Cohort: "browser-resolution-test", Network: network, Serial: 1,
		NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Minute), Bindings: []alpha.BindingInput{{Link: link, Target: target}}}, authorityPrivate)
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := alpha.OpenCorpus(authorityPublic, raw)
	if err != nil {
		t.Fatal(err)
	}
	floor, err := alpha.OpenPersistentFloor(alpha.PersistentFloorConfig{Root: t.TempDir(), Authority: authorityPublic,
		Cohort: "browser-resolution-test", Network: network})
	if err != nil {
		t.Fatal(err)
	}
	defer floor.Close()
	if err := floor.Observe(corpus); err != nil {
		t.Fatal(err)
	}
	endpoint := &endpoint{network: network}
	var opens int
	resolution, err := endpoint.OpenAlphaBrowserResolution(t.Context(), AlphaBrowserResolutionRequest{Floor: floor,
		Clock: func() time.Time { return now }, Open: func(_ context.Context, binding alpha.Binding) (AlphaBrowserSite, error) {
			opens++
			if binding.Link() != link || binding.Target() != target {
				return nil, errors.New("resolver supplied a different binding")
			}
			origin, openErr := reference.Open(reference.Config{Target: target,
				Document: reference.Resource{ContentType: "text/plain", Body: []byte("resolved")}})
			if openErr != nil {
				return nil, openErr
			}
			proxyURL, release, routeErr := endpoint.openAlphaBrowserRoute("blog.alice.ard", origin)
			if routeErr != nil {
				_ = origin.Close()
				return nil, routeErr
			}
			site := &alphaBrowserResolutionTestSite{ready: make(chan ReferenceReady, 1), done: make(chan ReferenceOutcome), close: func() error {
				release()
				return origin.Close()
			}}
			site.ready <- ReferenceReady{URL: "http://blog.alice.ard/", AlphaProxyURL: proxyURL, AuthenticatedTarget: target}
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
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxy)}}
	response, err := client.Get("http://blog.alice.ard/")
	if err != nil {
		t.Fatal(err)
	}
	body, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if readErr != nil || response.StatusCode != http.StatusOK || string(body) != "resolved" {
		t.Fatalf("resolved alpha response = %d %q %v", response.StatusCode, body, readErr)
	}
	if opens != 1 {
		t.Fatalf("named alpha opener count = %d, want 1", opens)
	}
	for rawURL, wantStatus := range map[string]int{"http://unregistered.ard/": http.StatusNotFound, "http://ordinary.invalid/": http.StatusBadRequest} {
		response, err = client.Get(rawURL)
		if err != nil {
			t.Fatal(err)
		}
		_ = response.Body.Close()
		if response.StatusCode != wantStatus {
			t.Fatalf("refused browser URL %q status = %d, want %d", rawURL, response.StatusCode, wantStatus)
		}
	}
	if opens != 1 {
		t.Fatalf("unregistered or ordinary browser URL opened another target: %d", opens)
	}
}

func TestAlphaBrowserResolutionClassifiesUnavailableCorpusStates(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	for _, test := range []struct {
		name                string
		withdrawn           bool
		notBefore, notAfter time.Time
		wantStatus          int
	}{
		{name: "withdrawn", withdrawn: true, notBefore: now.Add(-time.Minute), notAfter: now.Add(time.Minute), wantStatus: http.StatusGone},
		{name: "expired", notBefore: now.Add(-2 * time.Minute), notAfter: now.Add(-time.Minute), wantStatus: http.StatusServiceUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			network := targetLinkBytes(1)
			authorityPublic, authorityPrivate, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			link, err := alpha.ParseServiceLink("ardents-alpha://blog.alice")
			if err != nil {
				t.Fatal(err)
			}
			input := alpha.CorpusInput{Cohort: "browser-resolution-failure", Network: network, Serial: 1,
				NotBefore: test.notBefore, NotAfter: test.notAfter, Withdrawn: test.withdrawn}
			if !test.withdrawn {
				input.Bindings = []alpha.BindingInput{{Link: link, Target: targetLinkBytes(33)}}
			}
			raw, err := alpha.IssueCorpus(input, authorityPrivate)
			if err != nil {
				t.Fatal(err)
			}
			corpus, err := alpha.OpenCorpus(authorityPublic, raw)
			if err != nil {
				t.Fatal(err)
			}
			floor, err := alpha.OpenPersistentFloor(alpha.PersistentFloorConfig{Root: t.TempDir(), Authority: authorityPublic,
				Cohort: "browser-resolution-failure", Network: network})
			if err != nil {
				t.Fatal(err)
			}
			defer floor.Close()
			if err := floor.Observe(corpus); err != nil {
				t.Fatal(err)
			}
			endpoint := &endpoint{network: network}
			opens := 0
			resolution, err := endpoint.OpenAlphaBrowserResolution(t.Context(), AlphaBrowserResolutionRequest{Floor: floor,
				Clock: func() time.Time { return now }, Open: func(context.Context, alpha.Binding) (AlphaBrowserSite, error) {
					opens++
					return nil, errors.New("unavailable name opened a Service")
				}})
			if err != nil {
				t.Fatal(err)
			}
			defer resolution.Close()
			endpoint.alphaBrowserMu.Lock()
			proxyURL := endpoint.alphaBrowserProxy.URL()
			endpoint.alphaBrowserMu.Unlock()
			proxy, err := url.Parse(proxyURL)
			if err != nil {
				t.Fatal(err)
			}
			response, err := (&http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxy)}}).Get("http://blog.alice.ard/")
			if err != nil {
				t.Fatal(err)
			}
			_ = response.Body.Close()
			if response.StatusCode != test.wantStatus || opens != 0 {
				t.Fatalf("%s browser result = status %d openings %d, want status %d and no opening", test.name, response.StatusCode, opens, test.wantStatus)
			}
		})
	}
}

type alphaBrowserResolutionTestSite struct {
	ready chan ReferenceReady
	done  chan ReferenceOutcome
	close func() error
	once  sync.Once
	err   error
}

func (site *alphaBrowserResolutionTestSite) Ready() <-chan ReferenceReady {
	return site.ready
}

func (site *alphaBrowserResolutionTestSite) Done() <-chan ReferenceOutcome {
	return site.done
}

func (site *alphaBrowserResolutionTestSite) Close() error {
	site.once.Do(func() {
		if site.close != nil {
			site.err = site.close()
		}
		close(site.done)
	})
	return site.err
}
