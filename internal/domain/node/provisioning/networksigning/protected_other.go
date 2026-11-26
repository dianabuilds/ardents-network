//go:build !linux

package networksigning

import (
	"context"
	"os"
)

func Sign(context.Context, []string, func(string, *os.File)) (*Result, error) {
	return nil, errNetworkSigning
}
