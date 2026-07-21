// Package content provides immutable content operations through an Ardents Node.
package content

import "context"

type Reference struct {
	Kind string
	ID   string
}

type PutOptions struct {
	MediaType string
}

type PutOption func(*PutOptions)

func WithMediaType(mediaType string) PutOption {
	return func(options *PutOptions) { options.MediaType = mediaType }
}

type Service interface {
	Put(context.Context, []byte, ...PutOption) (Reference, error)
	Get(context.Context, Reference) ([]byte, error)
}
