//go:build browsercompat

package endpoint

import (
	"sync"

	"github.com/dianabuilds/ardents-network/internal/browserentry"
	reference "github.com/dianabuilds/ardents-network/internal/browserreference"
)

// browserCompatibility retains only the fields required to compile and check
// historical Browser presentation evidence. It is absent from the maintained
// headless dependency graph.
type browserCompatibility struct {
	alphaBrowserMu     sync.Mutex
	alphaBrowserRoutes int
	alphaBrowserOwners int
	alphaBrowserProxy  *reference.AlphaProxy
	browserEntry       *browserentry.Publisher
}

type browserCompatibilitySetup struct {
	BrowserEntryStatePath string
}

func openBrowserCompatibility(input browserCompatibilitySetup) (browserCompatibility, error) {
	if input.BrowserEntryStatePath == "" {
		return browserCompatibility{}, nil
	}
	entry, err := browserentry.OpenPublisher(input.BrowserEntryStatePath)
	if err != nil {
		return browserCompatibility{}, err
	}
	return browserCompatibility{browserEntry: entry}, nil
}

func (endpoint *endpoint) closeBrowserCompatibility() error {
	endpoint.closeAlphaBrowserProxy()
	if endpoint.browserEntry == nil {
		return nil
	}
	return endpoint.browserEntry.Close()
}
