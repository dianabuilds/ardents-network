//go:build integration

package publication

import (
	"context"

	"ardents/internal/discovery"
)

func (m *Manager) SetDiscoveryPublicationErrorForIntegrationTest(err error) {
	m.publishDiscoveryEntries = func(context.Context, []discovery.Entry) error {
		return err
	}
}
