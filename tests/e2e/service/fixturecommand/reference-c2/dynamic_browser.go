//go:build browsercompat

package main

import (
	"bytes"
	"errors"
	"io"
	"net/http"
)

// exerciseDynamicReference is the User-side browser workload for the H4-3B
// tracer. It sends normal HTTP through the one already selected Service and
// verifies that the alpha proxy does not become another destination chooser.
func exerciseDynamicReference(client *http.Client, origin string) error {
	response, err := beginDynamicPublishAndTimeline(client, origin)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "text/plain" {
		return errors.New("dynamic Publisher response headers were not preserved")
	}
	first := make([]byte, len("first-"))
	if _, err := io.ReadFull(response.Body, first); err != nil || string(first) != "first-" {
		return errors.New("dynamic Publisher first response chunk was not preserved")
	}
	rest, err := io.ReadAll(response.Body)
	if err != nil || string(rest) != "second" {
		return errors.New("dynamic Publisher streamed response was not preserved")
	}
	response, err = client.Get("http://unregistered.ard/")
	if err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		return errors.New("dynamic Browser Entry forwarded an unregistered alpha name")
	}
	response, err = client.Get("http://ordinary.invalid/")
	if err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		return errors.New("dynamic Browser Entry forwarded an ordinary Internet name")
	}
	response, err = client.Get(origin + "close")
	if err != nil {
		return err
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		return errors.New("dynamic Publisher did not close its selected Service")
	}
	return nil
}

// exerciseDynamicApplicationCrash observes the exact body prefix committed
// before failure. The Endpoint terminal carries the failure classification;
// the local HTTP server cannot replace committed headers with another status.
// Afterwards no name may become a fallback destination.
func exerciseDynamicApplicationCrash(client *http.Client, origin string) error {
	response, err := beginDynamicPublishAndTimeline(client, origin)
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "text/plain" {
		_ = response.Body.Close()
		return errors.New("dynamic Publisher partial response headers were not preserved")
	}
	partial, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if string(partial) != "first-" {
		return errors.New("dynamic Publisher Application crash did not stop at the declared partial response")
	}
	return requireDynamicNoFallback(client, origin)
}

// exerciseDynamicPublisherEndpointCrash completes the same application flow,
// then holds one request until the separately running Publisher Endpoint is
// hard-stopped by the process harness.
func exerciseDynamicPublisherEndpointCrash(client *http.Client, origin string) error {
	response, err := beginDynamicPublishAndTimeline(client, origin)
	if err != nil {
		return err
	}
	body, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if readErr != nil || response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "text/plain" || string(body) != "first-second" {
		return errors.New("dynamic Publisher response changed before Endpoint crash")
	}
	response, err = client.Get(origin + "crash")
	if err == nil {
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
		if response.StatusCode < http.StatusBadRequest {
			return errors.New("hard-stopped Publisher Endpoint reported HTTP success")
		}
	}
	return requireDynamicNoFallback(client, origin)
}

func beginDynamicPublishAndTimeline(client *http.Client, origin string) (*http.Response, error) {
	if client == nil {
		return nil, errors.New("dynamic fixture browser client is unavailable")
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	post, err := http.NewRequest(http.MethodPost, origin+"publish?draft=1", bytes.NewBufferString("title=ardents"))
	if err != nil {
		return nil, err
	}
	post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	post.Header.Set("Cookie", "before=bridge")
	response, err := client.Do(post)
	if err != nil {
		return nil, err
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusFound || response.Header.Get("Location") != "/timeline" || response.Header.Get("Set-Cookie") != "session=dynamic; Path=/" {
		return nil, errors.New("dynamic Publisher redirect or cookie was not preserved")
	}
	get, err := http.NewRequest(http.MethodGet, origin+"timeline", nil)
	if err != nil {
		return nil, err
	}
	get.Header.Set("Cookie", "session=dynamic")
	return client.Do(get)
}

func requireDynamicNoFallback(client *http.Client, origin string) error {
	for _, candidate := range []string{origin, "http://unregistered.ard/", "http://ordinary.invalid/"} {
		response, err := client.Get(candidate)
		if err != nil {
			continue
		}
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
		if response.StatusCode < http.StatusBadRequest {
			return errors.New("dynamic failure selected a fallback destination")
		}
	}
	return nil
}
