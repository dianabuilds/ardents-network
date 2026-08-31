package enrollment

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"runtime"
	"strings"
	"time"
)

const (
	closedAlphaInputSchema = "ardents-alpha-enrollment-input-v1"
	maximumInputSize       = 16 << 10
)

// ClosedAlphaInput is the bounded, non-secret local representation of an
// independently delivered closed-alpha enrollment pin. It never grants an
// update, installation, or Authority by itself.
type ClosedAlphaInput struct {
	Schema         string `json:"schema"`
	BundleRoot     string `json:"bundle_root"`
	Cohort         string `json:"cohort"`
	Release        string `json:"release"`
	Platform       string `json:"platform"`
	ManifestSHA256 string `json:"manifest_sha256"`
	Environment    string `json:"environment"`
	Network        string `json:"network"`
	TargetPath     string `json:"target_path"`
}

// ReadClosedAlphaInput reads one bounded, regular non-secret enrollment input.
// It deliberately does not fetch an invitation or validate the referenced
// bundle; Request supplies the resulting fact to Verify.
func ReadClosedAlphaInput(path string) (ClosedAlphaInput, error) {
	if path == "" {
		return ClosedAlphaInput{}, errors.New("alpha enrollment input is required")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() == 0 || info.Size() > maximumInputSize {
		return ClosedAlphaInput{}, errors.New("alpha enrollment input is not a bounded regular file")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return ClosedAlphaInput{}, err
	}
	var input ClosedAlphaInput
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || !validClosedAlphaInput(input) {
		return ClosedAlphaInput{}, errors.New("alpha enrollment input is invalid")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return ClosedAlphaInput{}, errors.New("alpha enrollment input is invalid")
	}
	return input, nil
}

// Request returns the exact verification request for artifact at the supplied
// decision time. The current architecture is local host fact, not invitation
// metadata.
func (input ClosedAlphaInput) Request(artifact string, at time.Time) Request {
	return Request{BundleRoot: input.BundleRoot, ExecutablePath: artifact,
		Pin:         Pin{Cohort: input.Cohort, Release: input.Release, Platform: input.Platform, ManifestSHA256: input.ManifestSHA256},
		Environment: input.Environment, Network: input.Network, TargetPath: input.TargetPath, Architecture: runtime.GOARCH, ReferenceTime: at.UTC()}
}

func validClosedAlphaInput(input ClosedAlphaInput) bool {
	for _, value := range []string{input.BundleRoot, input.Cohort, input.Release, input.Platform, input.ManifestSHA256, input.Environment, input.Network, input.TargetPath} {
		if value == "" || strings.ContainsAny(value, "\t\r\n") {
			return false
		}
	}
	return input.Schema == closedAlphaInputSchema
}
