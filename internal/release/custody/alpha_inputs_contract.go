package custody

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"
)

const (
	maximumAlphaRequestBytes  = 12 << 20
	maximumAlphaArtifactBytes = 64 << 20

	fixedAlphaProfile        = "ardents-h4-alpha-1-v1"
	fixedAlphaSourceRevision = "70bf425eec937edcc22e8f0534db992aa2002a16"
	fixedAlphaEndpointSHA256 = "33473599f7902508d1ca9cb9d09eb6777aff05d9c7c652e96f841b196bfd1fe1"
	fixedAlphaControlSHA256  = "d69b4c5d5f6fae76cbeacfb6acee8abaec9b6cbb56afd339982ea6d55ef9449c"
	fixedAlphaEnvelopeSHA256 = "0d14d5fcf9bc285e23d507a6382e9ff7100b2018acf182f6e65a885e52ec1738"
)

var (
	// ErrOutputExists reports that the fixed alpha-input output is not absent.
	ErrOutputExists = errors.New("release custody alpha-input output already exists")
	// ErrPreflight reports that constructed release or control evidence was not
	// accepted by the maintained readers before publication.
	ErrPreflight = errors.New("release custody alpha-input preflight rejected")
)

var alphaInputFileNames = [...]string{
	"1.root.json",
	"1.snapshot.json",
	"1.targets.json",
	"RELEASE",
	"catalog.ac1",
	"catalog.pub",
	"compatibility.ac1",
	"compatibility.pub",
	"corpus.pub",
	"network.ac1",
	"network.pub",
	"release.ac1",
	"release.pub",
	"timestamp.json",
}

type alphaInputPolicy struct {
	Profile, SourceRevision       string
	EndpointSHA256, ControlSHA256 string
	EnvelopeSHA256                string
}

var fixedAlphaInputPolicy = alphaInputPolicy{
	Profile: fixedAlphaProfile, SourceRevision: fixedAlphaSourceRevision,
	EndpointSHA256: fixedAlphaEndpointSHA256, ControlSHA256: fixedAlphaControlSHA256,
	EnvelopeSHA256: fixedAlphaEnvelopeSHA256,
}

// BuildAlphaInputsConfig supplies one bounded public request, exact artifact
// bytes, and one previously absent external output directory. The operation
// never accepts a signer, target path, metadata role, source URL, or upload
// destination.
type BuildAlphaInputsConfig struct {
	Root     string
	Request  []byte
	Endpoint []byte
	Control  []byte
	Output   string
}

// AlphaInputFile records one direct output file without exposing its bytes.
type AlphaInputFile struct {
	Name   string
	Size   int64
	Digest [32]byte
}

// AlphaInputsReceipt is the non-secret result of one accepted fixed-alpha
// construction. It is evidence, not first-contact authority.
type AlphaInputsReceipt struct {
	EnvelopeDigest [32]byte
	RequestDigest  [32]byte
	EndpointDigest [32]byte
	ControlDigest  [32]byte
	OutputDigest   [32]byte
	Cohort         string
	Release        string
	SourceRevision string
	NotBeforeUnix  int64
	NotAfterUnix   int64
	TUFVersion     uint64
	CatalogVersion uint64
	Preflight      string
	Files          []AlphaInputFile
}

// BuildAlphaInputs constructs and preflights the one ADR-0052 static input
// set, then publishes the complete direct directory atomically. Decrypted key
// material is retained only for this call and is never returned.
func BuildAlphaInputs(ctx context.Context, config BuildAlphaInputsConfig, secrets SecretInput) (AlphaInputsReceipt, error) {
	return buildAlphaInputs(ctx, config, secrets, fixedAlphaInputPolicy, time.Now().UTC())
}

func buildAlphaInputs(ctx context.Context, config BuildAlphaInputsConfig, secrets SecretInput, policy alphaInputPolicy,
	invokedAt time.Time) (AlphaInputsReceipt, error) {
	if ctx == nil || secrets == nil {
		return AlphaInputsReceipt{}, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return AlphaInputsReceipt{}, err
	}
	request, err := parseAlphaInputsRequest(config.Request, config.Endpoint, config.Control, policy, invokedAt)
	if err != nil {
		return AlphaInputsReceipt{}, err
	}
	root, err := checkedRoot(config.Root)
	if err != nil {
		return AlphaInputsReceipt{}, err
	}
	output, err := checkedAlphaOutput(config.Output)
	if err != nil {
		return AlphaInputsReceipt{}, err
	}
	raw, err := readRecord(seedPath(root))
	if err != nil {
		return AlphaInputsReceipt{}, err
	}
	defer zero(raw)
	envelopeDigest := digest(raw)
	if hex.EncodeToString(envelopeDigest[:]) != policy.EnvelopeSHA256 {
		return AlphaInputsReceipt{}, ErrInvalid
	}
	password, err := secrets.ReadSecret(ctx, PromptAssemble)
	if err != nil {
		return AlphaInputsReceipt{}, err
	}
	defer zero(password)
	if !validPassword(password) {
		return AlphaInputsReceipt{}, ErrSecret
	}
	record, err := openRecord(raw, password)
	if err != nil {
		return AlphaInputsReceipt{}, err
	}
	defer zeroRecord(record)
	files, err := buildAlphaInputFiles(ctx, request, record, config.Endpoint)
	if err != nil {
		return AlphaInputsReceipt{}, err
	}
	if err := ctx.Err(); err != nil {
		return AlphaInputsReceipt{}, err
	}
	fileReceipt, outputDigest, err := publishAlphaInputFiles(ctx, output, request, files, config.Endpoint, config.Control)
	if err != nil {
		return AlphaInputsReceipt{}, err
	}
	return AlphaInputsReceipt{
		EnvelopeDigest: envelopeDigest, RequestDigest: sha256.Sum256(config.Request),
		EndpointDigest: sha256.Sum256(config.Endpoint), ControlDigest: sha256.Sum256(config.Control), OutputDigest: outputDigest,
		Cohort: request.Cohort, Release: request.Release, SourceRevision: request.SourceRevision,
		NotBeforeUnix: request.NotBefore.Unix(), NotAfterUnix: request.NotAfter.Unix(),
		TUFVersion: 1, CatalogVersion: 1, Preflight: "accepted", Files: fileReceipt,
	}, nil
}
