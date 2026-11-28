// Package delivery publishes Custody results to local files. Callers supply
// the original committed receipt; its type alone does not prove provenance. Public
// Service responses and private Admission Permissions share no-replace staged
// publication but retain different access checks. Private delivery is accepted
// only on qualified platforms and filesystems; unsupported hosts refuse before
// sensitive writes. It does not open the Vault or issue authority.
package delivery
