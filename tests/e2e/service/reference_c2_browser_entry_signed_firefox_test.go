//go:build referencec2 && h4_4_signed_firefox

package service_test

import "testing"

func TestReferenceC2CarriesDynamicPublisherApplicationThroughSignedFirefoxBrowserEntry(t *testing.T) {
	runReferenceC2(t, referenceC2Scenario{transparentApplication: true, browserEntryDynamic: true, signedFirefox: true})
}
