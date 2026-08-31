//go:build !browsercompat

package endpoint

// browserCompatibility is empty in the maintained headless build. The
// browsercompat variant exists only so retained historical evidence remains a
// checked, buildable profile without adding Browser dependencies to Endpoint.
type browserCompatibility struct{}

type browserCompatibilitySetup struct{}

func openBrowserCompatibility(browserCompatibilitySetup) (browserCompatibility, error) {
	return browserCompatibility{}, nil
}

func (*endpoint) closeBrowserCompatibility() error { return nil }
