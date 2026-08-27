package alpha

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/naming"
)

const (
	corpusDomain      = "ardents-alpha-corpus-v1\x00"
	corpusVersion     = 1
	maxCorpusBindings = 8
	maxCohortLength   = 64
	activeCorpus      = 0
	withdrawnCorpus   = 1
)

// Failure is one participant-visible terminal alpha-resolution state.
type Failure string

const (
	// FailureUnavailable means the requested alpha name is absent from a
	// verified active corpus.
	FailureUnavailable Failure = "unavailable"
	// FailureNotYetValid means a signed corpus has not reached its validity
	// start time according to the caller's clock.
	FailureNotYetValid Failure = "not-yet-valid"
	// FailureExpired means a signed corpus has reached its validity end time.
	FailureExpired Failure = "expired"
	// FailureWithdrawn means the entire signed alpha corpus is withdrawn.
	FailureWithdrawn Failure = "withdrawn"
	// FailureStale means a response predates an already observed alpha corpus.
	FailureStale Failure = "stale"
	// FailureConflict means two different signed corpora claim one serial.
	FailureConflict Failure = "conflict"
)

// ResolutionError reports a classified terminal result. Malformed or
// unauthenticated corpus bytes remain ordinary errors and are not converted to
// a destination result.
type ResolutionError struct {
	Failure Failure
}

func (err *ResolutionError) Error() string { return "alpha resolution " + string(err.Failure) }

// HasFailure reports whether err is the requested classified alpha-resolution
// result.
func HasFailure(err error, expected Failure) bool {
	var resolutionErr *ResolutionError
	return errors.As(err, &resolutionErr) && resolutionErr.Failure == expected
}

// BindingInput is one exact alpha-name-to-Target entry of a complete signed
// corpus. A Target is opaque to this package.
type BindingInput struct {
	Link   ServiceLink
	Target [32]byte
}

// CorpusInput describes one complete finite alpha corpus. A withdrawn corpus
// carries no bindings, making withdrawal explicit and authenticated rather
// than an omitted result from a resolver.
type CorpusInput struct {
	Cohort    string
	Network   [32]byte
	Serial    uint64
	NotBefore time.Time
	NotAfter  time.Time
	Withdrawn bool
	Bindings  []BindingInput
}

// Corpus is one verified signed alpha corpus. Resolve checks its signed time
// bounds on every use; it is neither a current canonical Namespace view nor a
// public registration authority.
type Corpus struct {
	cohort    string
	network   [32]byte
	serial    uint64
	notBefore time.Time
	notAfter  time.Time
	withdrawn bool
	bindings  map[ServiceLink]Binding
	raw       []byte
}

// Binding is an exact alpha-only destination selected from a verified Corpus.
type Binding struct {
	network [32]byte
	link    ServiceLink
	target  [32]byte
	serial  uint64
}

// Link returns the alpha-only Service Link that this binding names.
func (binding Binding) Link() ServiceLink { return binding.link }

// Network returns the exact Ardents Network in which this alpha binding may
// be used.
func (binding Binding) Network() [32]byte { return binding.network }

// Target returns the opaque exact Service Target carried by this binding.
func (binding Binding) Target() [32]byte { return binding.target }

// Serial returns the signed corpus serial that produced this binding.
func (binding Binding) Serial() uint64 { return binding.serial }

// Cohort returns the explicit alpha cohort named by the signed corpus.
func (corpus *Corpus) Cohort() string {
	if corpus == nil {
		return ""
	}
	return corpus.cohort
}

// Serial returns the signed corpus serial. It is useful only with an
// authority-owned floor; it is not a canonical Namespace generation.
func (corpus *Corpus) Serial() uint64 {
	if corpus == nil {
		return 0
	}
	return corpus.serial
}

// Network returns the exact Ardents Network carried by the signed corpus.
func (corpus *Corpus) Network() [32]byte {
	if corpus == nil {
		return [32]byte{}
	}
	return corpus.network
}

// NotAfter returns the corpus's signed validity end. It is a corpus-control
// fact, not a canonical Namespace lease.
func (corpus *Corpus) NotAfter() time.Time {
	if corpus == nil {
		return time.Time{}
	}
	return corpus.notAfter
}

// Bytes returns the original authority-signed corpus bytes. A caller must
// verify them again against its own pinned authority before treating them as a
// resolution response.
func (corpus *Corpus) Bytes() []byte {
	if corpus == nil {
		return nil
	}
	return append([]byte(nil), corpus.raw...)
}

// Digest returns a stable digest of the signed corpus bytes for a local
// serial-floor comparison. It is not a public name identifier.
func (corpus *Corpus) Digest() [32]byte { return sha256.Sum256(corpus.Bytes()) }

// IssueCorpus serializes and signs one complete bounded alpha corpus. It is a
// fixture/control primitive; it does not publish, distribute, or register a
// name.
func IssueCorpus(input CorpusInput, authority ed25519.PrivateKey) ([]byte, error) {
	if len(authority) != ed25519.PrivateKeySize {
		return nil, errors.New("alpha corpus authority private key has an invalid length")
	}
	if err := validateInput(input); err != nil {
		return nil, err
	}

	notBefore, err := canonicalMilliseconds(input.NotBefore)
	if err != nil {
		return nil, fmt.Errorf("alpha corpus not-before: %w", err)
	}
	notAfter, err := canonicalMilliseconds(input.NotAfter)
	if err != nil {
		return nil, fmt.Errorf("alpha corpus not-after: %w", err)
	}
	body := make([]byte, 0, 512)
	body = append(body, corpusDomain...)
	body = append(body, corpusVersion, byte(len(input.Cohort)))
	body = append(body, input.Cohort...)
	var fixed [8]byte
	binary.BigEndian.PutUint64(fixed[:], input.Serial)
	body = append(body, fixed[:]...)
	body = append(body, input.Network[:]...)
	binary.BigEndian.PutUint64(fixed[:], uint64(notBefore))
	body = append(body, fixed[:]...)
	binary.BigEndian.PutUint64(fixed[:], uint64(notAfter))
	body = append(body, fixed[:]...)
	if input.Withdrawn {
		body = append(body, withdrawnCorpus)
	} else {
		body = append(body, activeCorpus)
	}
	body = append(body, byte(len(input.Bindings)))
	for _, binding := range input.Bindings {
		nameWire, wireErr := naming.EncodeWire(binding.Link.Name())
		if wireErr != nil {
			return nil, fmt.Errorf("alpha corpus binding: %w", wireErr)
		}
		var length [2]byte
		binary.BigEndian.PutUint16(length[:], uint16(len(nameWire)))
		body = append(body, length[:]...)
		body = append(body, nameWire...)
		body = append(body, binding.Target[:]...)
	}
	signature := ed25519.Sign(authority, body)
	return append(body, signature...), nil
}

// OpenCorpus verifies one signed alpha corpus. A caller must still resolve it
// at an explicit time, so a once-valid corpus cannot remain usable after its
// signed expiry.
func OpenCorpus(authority ed25519.PublicKey, raw []byte) (*Corpus, error) {
	if len(authority) != ed25519.PublicKeySize {
		return nil, errors.New("alpha corpus authority public key has an invalid length")
	}
	if len(raw) < len(corpusDomain)+1+1+1+8+32+8+8+1+1+ed25519.SignatureSize {
		return nil, errors.New("alpha corpus is truncated")
	}
	unsigned := raw[:len(raw)-ed25519.SignatureSize]
	if !ed25519.Verify(authority, unsigned, raw[len(unsigned):]) {
		return nil, errors.New("alpha corpus signature is invalid")
	}
	decoded, err := decodeCorpus(unsigned)
	if err != nil {
		return nil, err
	}
	return &Corpus{cohort: decoded.cohort, network: decoded.network, serial: decoded.serial, notBefore: decoded.notBefore,
		notAfter: decoded.notAfter, withdrawn: decoded.withdrawn, bindings: decoded.bindings,
		raw: append([]byte(nil), raw...)}, nil
}

// Resolve returns the sole exact binding for an alpha-only link. An unknown
// name is unavailable; it is never translated to another destination form.
func (corpus *Corpus) Resolve(link ServiceLink, now time.Time) (Binding, error) {
	if corpus == nil {
		return Binding{}, errors.New("alpha corpus is nil")
	}
	if err := corpus.ValidAt(now); err != nil {
		return Binding{}, err
	}
	if corpus.withdrawn {
		return Binding{}, &ResolutionError{Failure: FailureWithdrawn}
	}
	binding, ok := corpus.bindings[link]
	if !ok {
		return Binding{}, &ResolutionError{Failure: FailureUnavailable}
	}
	return binding, nil
}

// ValidAt verifies only the signed corpus validity interval. A verified
// withdrawn corpus remains valid control evidence, so withdrawal is returned
// by Resolve rather than this method.
func (corpus *Corpus) ValidAt(now time.Time) error {
	if corpus == nil || now.IsZero() {
		return errors.New("alpha corpus resolution time is required")
	}
	if now.Before(corpus.notBefore) {
		return &ResolutionError{Failure: FailureNotYetValid}
	}
	if !now.Before(corpus.notAfter) {
		return &ResolutionError{Failure: FailureExpired}
	}
	return nil
}

type decodedCorpus struct {
	cohort    string
	network   [32]byte
	serial    uint64
	notBefore time.Time
	notAfter  time.Time
	withdrawn bool
	bindings  map[ServiceLink]Binding
}

func decodeCorpus(raw []byte) (decodedCorpus, error) {
	if !strings.HasPrefix(string(raw), corpusDomain) {
		return decodedCorpus{}, errors.New("alpha corpus domain is invalid")
	}
	position := len(corpusDomain)
	if raw[position] != corpusVersion {
		return decodedCorpus{}, errors.New("alpha corpus version is unsupported")
	}
	position++
	cohortLength := int(raw[position])
	position++
	if cohortLength == 0 || cohortLength > maxCohortLength || position+cohortLength > len(raw) {
		return decodedCorpus{}, errors.New("alpha corpus cohort is malformed")
	}
	cohort := string(raw[position : position+cohortLength])
	position += cohortLength
	if !validCohort(cohort) || position+8+32+8+8+1+1 > len(raw) {
		return decodedCorpus{}, errors.New("alpha corpus is malformed")
	}
	serial := binary.BigEndian.Uint64(raw[position : position+8])
	position += 8
	var network [32]byte
	copy(network[:], raw[position:position+32])
	position += 32
	notBefore := time.UnixMilli(int64(binary.BigEndian.Uint64(raw[position : position+8]))).UTC()
	position += 8
	notAfter := time.UnixMilli(int64(binary.BigEndian.Uint64(raw[position : position+8]))).UTC()
	position += 8
	state := raw[position]
	position++
	count := int(raw[position])
	position++
	if network == [32]byte{} || serial == 0 || !notAfter.After(notBefore) || count > maxCorpusBindings {
		return decodedCorpus{}, errors.New("alpha corpus has invalid metadata")
	}
	if state != activeCorpus && state != withdrawnCorpus {
		return decodedCorpus{}, errors.New("alpha corpus state is invalid")
	}
	if (state == withdrawnCorpus && count != 0) || (state == activeCorpus && count == 0) {
		return decodedCorpus{}, errors.New("alpha corpus state and bindings conflict")
	}
	bindings := make(map[ServiceLink]Binding, count)
	for i := 0; i < count; i++ {
		if position+2 > len(raw) {
			return decodedCorpus{}, errors.New("alpha corpus binding is truncated")
		}
		nameLength := int(binary.BigEndian.Uint16(raw[position : position+2]))
		position += 2
		if nameLength == 0 || position+nameLength+32 > len(raw) {
			return decodedCorpus{}, errors.New("alpha corpus binding is malformed")
		}
		name, err := naming.DecodeWire(raw[position : position+nameLength])
		if err != nil {
			return decodedCorpus{}, fmt.Errorf("alpha corpus binding name: %w", err)
		}
		position += nameLength
		var target [32]byte
		copy(target[:], raw[position:position+32])
		position += 32
		if target == [32]byte{} {
			return decodedCorpus{}, errors.New("alpha corpus binding target is empty")
		}
		link := ServiceLink{name: name}
		if _, exists := bindings[link]; exists {
			return decodedCorpus{}, errors.New("alpha corpus has conflicting name bindings")
		}
		bindings[link] = Binding{network: network, link: link, target: target, serial: serial}
	}
	if position != len(raw) {
		return decodedCorpus{}, errors.New("alpha corpus contains trailing data")
	}
	return decodedCorpus{cohort: cohort, network: network, serial: serial, notBefore: notBefore, notAfter: notAfter,
		withdrawn: state == withdrawnCorpus, bindings: bindings}, nil
}

func validateInput(input CorpusInput) error {
	if !validCohort(input.Cohort) {
		return errors.New("alpha corpus cohort is invalid")
	}
	if input.Serial == 0 {
		return errors.New("alpha corpus serial must be nonzero")
	}
	if input.Network == [32]byte{} {
		return errors.New("alpha corpus network is required")
	}
	if input.NotAfter.IsZero() || !input.NotAfter.After(input.NotBefore) {
		return errors.New("alpha corpus validity interval is invalid")
	}
	if len(input.Bindings) > maxCorpusBindings {
		return errors.New("alpha corpus has too many bindings")
	}
	if input.Withdrawn && len(input.Bindings) != 0 {
		return errors.New("withdrawn alpha corpus must not contain bindings")
	}
	if !input.Withdrawn && len(input.Bindings) == 0 {
		return errors.New("active alpha corpus must contain a binding")
	}
	seen := make(map[ServiceLink]struct{}, len(input.Bindings))
	for _, binding := range input.Bindings {
		if _, err := naming.Parse(string(binding.Link.name)); err != nil {
			return fmt.Errorf("alpha corpus binding link: %w", err)
		}
		if binding.Target == [32]byte{} {
			return errors.New("alpha corpus binding target is empty")
		}
		if _, exists := seen[binding.Link]; exists {
			return errors.New("alpha corpus has conflicting name bindings")
		}
		seen[binding.Link] = struct{}{}
	}
	return nil
}

func validCohort(cohort string) bool {
	if len(cohort) == 0 || len(cohort) > maxCohortLength || strings.HasPrefix(cohort, "-") || strings.HasSuffix(cohort, "-") {
		return false
	}
	for _, character := range cohort {
		if !((character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '-') {
			return false
		}
	}
	return true
}

func canonicalMilliseconds(value time.Time) (int64, error) {
	if value.IsZero() || value.Nanosecond()%int(time.Millisecond) != 0 {
		return 0, errors.New("must be a nonzero whole UTC millisecond")
	}
	milliseconds := value.UnixMilli()
	if !value.Equal(time.UnixMilli(milliseconds)) {
		return 0, errors.New("is outside the canonical millisecond range")
	}
	return milliseconds, nil
}
