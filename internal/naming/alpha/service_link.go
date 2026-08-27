package alpha

import (
	"errors"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/naming"
)

const serviceLinkPrefix = "ardents-alpha://"

// ServiceLink is an explicitly alpha-only reference to one canonical Service
// Name. It is intentionally distinct from the canonical Namespace's
// ardents:// Service Link.
type ServiceLink struct {
	name naming.Name
}

// ParseServiceLink parses exactly one canonical alpha-only Service Link.
func ParseServiceLink(raw string) (ServiceLink, error) {
	if !strings.HasPrefix(raw, serviceLinkPrefix) {
		return ServiceLink{}, errors.New("alpha Service Link lacks the ardents-alpha:// scheme")
	}
	name, err := naming.Parse(strings.TrimPrefix(raw, serviceLinkPrefix))
	if err != nil {
		return ServiceLink{}, err
	}
	return ServiceLink{name: name}, nil
}

// Name returns the canonical Name carried by this alpha-only link.
func (link ServiceLink) Name() naming.Name { return link.name }

// String returns the sole canonical serialized form of this alpha-only link.
func (link ServiceLink) String() string { return serviceLinkPrefix + string(link.name) }
