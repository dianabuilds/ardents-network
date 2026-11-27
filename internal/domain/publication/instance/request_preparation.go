package instance

import (
	"context"
	"errors"
)

// InitializeRequest prepares the Instance and returns its public request after
// closing the original root.
func InitializeRequest(ctx context.Context, config Config) ([]byte, error) {
	root, err := Initialize(ctx, config)
	if err != nil {
		return nil, err
	}
	return requestAndClose(ctx, root)
}

// ReadRequest reopens a prepared Instance and returns the same public request
// after closing the original root.
func ReadRequest(ctx context.Context, path string) ([]byte, error) {
	root, err := Open(ctx, path)
	if err != nil {
		return nil, err
	}
	return requestAndClose(ctx, root)
}

func requestAndClose(ctx context.Context, root *Root) ([]byte, error) {
	raw, err := root.Request(ctx)
	if err := errors.Join(err, root.Close(), ctx.Err()); err != nil {
		return nil, err
	}
	return raw, nil
}
