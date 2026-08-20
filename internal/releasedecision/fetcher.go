package releasedecision

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/theupdateframework/go-tuf/v2/metadata"
)

// fetcherAdapter is the bounded byte Adapter the package feeds to go-tuf.
// It is unexported because the production caller uses offline-import
// exclusively; tests construct a fetcherAdapter through newMapFetcher
// (same package) to exercise the bounded envelope and the two fake
// distributor independence test. The Adapter owns no network client and
// makes no trust decision: it is a fail-closed reject-only resource
// preflight that hands the candidate the exact bytes from Inputs.Files.
type fetcherAdapter struct {
	mu       sync.Mutex
	base     *url.URL
	files    map[string][]byte
	envelope envelopeState
}

type envelopeState struct {
	fetches int
	bytes   int64
}

// newMapFetcher wraps the supplied files as a bounded fetcherAdapter
// rooted at the supplied base URL. The base URL must be a canonical
// https URL with no query, fragment, or user info; the candidate uses
// the base to enforce URL confinement.
func newMapFetcher(base string, files map[string][]byte) (*fetcherAdapter, error) {
	parsed, err := url.Parse(base)
	if err != nil {
		return nil, fmt.Errorf("releasedecision: parse base URL: %w", err)
	}
	if parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil ||
		parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("releasedecision: base URL is outside the offline envelope")
	}
	parsed.Path = strings.TrimSuffix(path.Clean(parsed.Path), "/") + "/"
	return &fetcherAdapter{base: parsed, files: copyFiles(files), envelope: envelopeState{}}, nil
}

// copyFiles returns a shallow copy of the supplied map so the fetcher
// can keep the original Inputs.Files untouched.
func copyFiles(files map[string][]byte) map[string][]byte {
	dup := make(map[string][]byte, len(files))
	for key, value := range files {
		dup[key] = value
	}
	return dup
}

// DownloadFile is the go-tuf fetcher interface method. It refuses URLs
// outside the base path, refuses any non-https request, refuses query or
// fragment, and refuses to exceed the bounded per-file and aggregate
// byte envelopes.
func (f *fetcherAdapter) DownloadFile(raw string, maxLength int64, _ time.Duration) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.envelope.fetches++
	if f.envelope.fetches > maximumFetches {
		return nil, fmt.Errorf("releasedecision: fetch count exceeds %d", maximumFetches)
	}
	if maxLength < 0 || maxLength > maximumMetadataFileBytes {
		return nil, fmt.Errorf("releasedecision: requested length %d exceeds the bound", maxLength)
	}
	request, err := url.Parse(raw)
	if err != nil || request.Scheme != f.base.Scheme || request.Host != f.base.Host ||
		request.User != nil || request.RawQuery != "" || request.Fragment != "" {
		return nil, errors.New("releasedecision: URL is outside the offline envelope")
	}
	decoded, err := url.PathUnescape(request.EscapedPath())
	if err != nil || !strings.HasPrefix(decoded, f.base.Path) {
		return nil, errors.New("releasedecision: URL path is outside the offline envelope")
	}
	cleaned := path.Clean(decoded)
	if cleaned != decoded || strings.Contains(decoded, `\`) {
		return nil, errors.New("releasedecision: URL path is not confined")
	}
	for _, segment := range strings.Split(decoded, "/") {
		if segment == "." || segment == ".." {
			return nil, errors.New("releasedecision: URL path escapes the offline envelope")
		}
	}
	data, ok := f.files[raw]
	if !ok {
		// The TUF client uses a 404 to signal "no newer root" or
		// "missing consistent-snapshot variant". Return the typed
		// error the client expects so Refresh can complete.
		return nil, &metadata.ErrDownloadHTTP{StatusCode: 404, URL: raw}
	}
	if int64(len(data)) > maxLength || int64(len(data)) > maximumMetadataFileBytes {
		return nil, &releaseError{class: outcomeReleaseInvalid, message: "metadata file exceeds the per-file bound"}
	}
	if f.envelope.bytes+int64(len(data)) > maximumMetadataBytes {
		return nil, &releaseError{class: outcomeReleaseInvalid, message: "metadata aggregate exceeds the bound"}
	}
	f.envelope.bytes += int64(len(data))
	return append([]byte(nil), data...), nil
}

// envelopeUsed reports the bounded fetch and byte envelope that the
// fetcher actually consumed. Evaluate reads it after Refresh to confirm
// the candidate stayed inside the published profile.
func (f *fetcherAdapter) envelopeUsed() envelopeState {
	f.mu.Lock()
	defer f.mu.Unlock()
	return envelopeState{fetches: f.envelope.fetches, bytes: f.envelope.bytes}
}

// releaseError is a typed error that carries the bounded outcome the
// package would otherwise return. The candidate's helpers (like
// get-tuf's metadata.ErrDownloadHTTP) are string-only; the package
// converts the messages back into outcomes at the top of Evaluate.
type releaseError struct {
	class   Outcome
	message string
}

func (e *releaseError) Error() string {
	return e.message
}

// classifyReleaseError maps a fetcher error to a bounded outcome. The
// classification is conservative: any unrecognized error becomes
// release-invalid.
func classifyReleaseError(err error) Outcome {
	if err == nil {
		return outcomeReleaseAccepted
	}
	var typed *releaseError
	if errors.As(err, &typed) {
		return typed.class
	}
	if strings.Contains(err.Error(), "exceeds the bound") {
		return outcomeReleaseInvalid
	}
	if strings.Contains(err.Error(), "URL") || strings.Contains(err.Error(), "offline envelope") {
		return outcomeReleaseInvalid
	}
	if strings.Contains(err.Error(), "exceeds") {
		return outcomeReleaseInvalid
	}
	return outcomeReleaseInvalid
}
