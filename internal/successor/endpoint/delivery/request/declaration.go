package request

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	posixpath "path"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/endpoint/runtimeplan"
)

// Declaration is one root-declared installation input. It contains declarations
// and an independently supplied first-install pin, never Permission responses,
// Authority keys or serialized Release authorizations.
type Declaration struct {
	Schema           string               `json:"schema"`
	BundleRoot       string               `json:"bundle_root"`
	ManifestSHA256   string               `json:"manifest_sha256,omitempty"`
	InstallationRoot string               `json:"installation_root"`
	ReleaseFloorRoot string               `json:"release_floor_root"`
	ReferenceTime    string               `json:"reference_time"`
	Headless         runtimeplan.Headless `json:"headless"`
	Source           runtimeplan.Source   `json:"source"`
}

func DecodeDeclaration(raw []byte) (Declaration, error) {
	var request Declaration
	if err := decodeCanonical(raw, 64<<10, &request); err != nil {
		return Declaration{}, err
	}
	if request.Schema != "ardents-endpoint-installation-request-v1" {
		return Declaration{}, errors.New("installation request schema is invalid")
	}
	if request.ManifestSHA256 != "" && !canonicalDigest(request.ManifestSHA256) {
		return Declaration{}, errors.New("installation pin is invalid")
	}
	at, err := time.Parse(time.RFC3339Nano, request.ReferenceTime)
	if err != nil || at.IsZero() || at.UTC().Format(time.RFC3339Nano) != request.ReferenceTime {
		return Declaration{}, errors.New("installation reference time must be canonical UTC")
	}
	headlessRaw, err := json.Marshal(request.Headless)
	if err != nil {
		return Declaration{}, err
	}
	headless, err := runtimeplan.DecodeHeadless(headlessRaw)
	if err != nil {
		return Declaration{}, fmt.Errorf("installation headless declaration: %w", err)
	}
	sourceRaw, err := json.Marshal(request.Source)
	if err != nil {
		return Declaration{}, err
	}
	source, err := runtimeplan.DecodeSource(sourceRaw)
	if err != nil {
		return Declaration{}, fmt.Errorf("installation Source declaration: %w", err)
	}
	if headless.NetworkID != source.NetworkID || headless.NetworkThreshold != source.Threshold ||
		headless.LocalRoleStateRoot != source.LocalRoleStateRoot || headless.TimeConfidenceFile != source.ClockObservationFile ||
		source.RefreshIntervalMS == 0 || !sameSignerMap(headless.NetworkAuthorities, source.Authorities) {
		return Declaration{}, errors.New("installation Source does not match its headless declaration")
	}
	// The selected generation renders this sole field to its bound Source file.
	// A caller cannot retain an external mutable Source plan as another owner.
	if request.Headless.NetworkSourcePlan != "" {
		return Declaration{}, errors.New("installation Source must be inlined, not an external plan")
	}
	immutable := []string{request.BundleRoot, request.InstallationRoot, request.ReleaseFloorRoot}
	for index, path := range immutable {
		if !canonicalPath(path) || posixpath.Dir(path) == path {
			return Declaration{}, errors.New("installation root path is invalid")
		}
		for _, other := range immutable[:index] {
			if pathsOverlap(path, other) {
				return Declaration{}, errors.New("installation roots overlap")
			}
		}
	}
	roots := MutableRoots(request.Headless)
	for i, root := range roots {
		for _, prior := range roots[:i] {
			if pathsOverlap(root, prior) {
				return Declaration{}, errors.New("declared mutable roots overlap")
			}
		}
	}
	paths := append(MutableRoots(request.Headless), request.Headless.ApplicationSocket, request.Headless.TimeConfidenceFile,
		request.Headless.ReaderPermission.RequestPath, request.Headless.ReaderPermission.ResponsePath,
		request.Source.ClientCertificate, request.Source.ClientKey)
	if request.Headless.Role == "" {
		paths = append(paths, request.Headless.AdministrationSocket, request.Headless.PublisherPermission.RequestPath, request.Headless.PublisherPermission.ResponsePath)
	}
	for _, member := range request.Source.Sources {
		paths = append(paths, member.RootCA)
	}
	for _, path := range paths {
		if !canonicalPath(path) {
			return Declaration{}, errors.New("installation declaration path is invalid")
		}
		for _, root := range immutable {
			if pathsOverlap(path, root) {
				return Declaration{}, errors.New("installation declarations overlap an immutable or Release root")
			}
		}
	}
	return request, nil
}

func decodeCanonical(raw []byte, maximum int, target any) error {
	if len(raw) == 0 || len(raw) > maximum {
		return errors.New("installation document exceeds its bound")
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return errors.New("installation document is invalid")
	}
	canonical, err := json.Marshal(target)
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return errors.New("installation document is not canonical")
	}
	return nil
}

func canonicalDigest(value string) bool {
	digest, err := hex.DecodeString(value)
	return err == nil && len(digest) == 32 && hex.EncodeToString(digest) == value
}

func canonicalPath(path string) bool {
	return posixpath.IsAbs(path) && posixpath.Clean(path) == path && !strings.ContainsAny(path, "\x00\r\n")
}

func pathsOverlap(left, right string) bool {
	return left == right || strings.HasPrefix(left, right+"/") || strings.HasPrefix(right, left+"/")
}

func MutableRoots(plan runtimeplan.Headless) []string {
	roots := []string{plan.NetworkStateRoot, plan.EntryStateRoot, plan.LocalRoleStateRoot, plan.TextTokenRoot}
	if plan.Role == "" {
		roots = append(roots, plan.PublicationRoot, plan.ServiceInstanceRoot)
	}
	return roots
}

func sameSignerMap(left, right map[[32]byte]ed25519.PublicKey) bool {
	if len(left) != len(right) {
		return false
	}
	for identity, key := range left {
		if !bytes.Equal(key, right[identity]) {
			return false
		}
	}
	return true
}
