package launch

import (
	"github.com/dianabuilds/ardents-network/internal/domain/endpoint/plan"
	endpointunit "github.com/dianabuilds/ardents-network/internal/domain/endpoint/unit"
)

// Template checks precede writable-root admission, retaining the original first refusal.
func renderEndpointUnit(template []byte, request installationRequest, directory string) ([]byte, error) {
	prepared, err := endpointunit.PrepareEndpointTemplate(template, request.InstallationRoot, directory)
	if err != nil {
		return nil, err
	}
	expected, err := expectedUnit(request)
	if err != nil {
		return nil, err
	}
	return prepared.Render(expected.WritePaths)
}

// Write-root admission stays with the Endpoint declaration. The fixed unit Module
// receives detached expected facts, never the private request or live custody.
func expectedUnit(request installationRequest) (endpointunit.Configuration, error) {
	paths, err := plan.WritableDirectories(request)
	return endpointunit.Configuration{InstallationRoot: request.InstallationRoot, WritePaths: paths}, err
}
