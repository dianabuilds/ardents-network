package request

import (
	"context"
	"errors"
	"slices"
)

// ErrInput preserves Installation's existing bounded input refusal identity.
var ErrInput = errors.New("installation: invalid generation authentication input")

// Document retains checked local declarations only. It grants no Release,
// Network, native ownership or runtime authority. Paths use the selected Linux
// installation grammar on every host; native admission is a later operation.
type Document struct {
	declared *Declaration
	custody  *Origin
}

// Decode checks one canonical bounded request before byte authentication.
// Initial input requires the independent pin; successor input forbids it.
func Decode(ctx context.Context, raw []byte, initial bool) (Document, error) {
	if ctx == nil {
		return Document{}, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return Document{}, err
	}
	declared, err := DecodeDeclaration(raw)
	if err != nil {
		return Document{}, errors.Join(ErrInput, err)
	}
	if initial && declared.ManifestSHA256 == "" || !initial && declared.ManifestSHA256 != "" {
		return Document{}, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return Document{}, err
	}
	return Document{declared: &declared}, nil
}

// Declaration returns detached checked schema data, never native or Release authority.
func (d Document) Declaration() (Declaration, bool) {
	if d.declared == nil {
		return Declaration{}, false
	}
	value := *d.declared
	value.Headless.NetworkAuthorities = slices.Clone(value.Headless.NetworkAuthorities)
	value.Source.AuthorityPublic = slices.Clone(value.Source.AuthorityPublic)
	value.Source.Sources = slices.Clone(value.Source.Sources)
	return value, true
}

// Origin returns opaque original-file provenance; decoded documents have none.
func (d Document) Origin() *Origin { return d.custody }
