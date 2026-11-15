// Package nodeprincipal validates the fixed Node account and independently
// observes current host account files and NSS. Each observation retains and
// closes its own files and helper; it owns no account creation, journal,
// process identity or installation authority.
package nodeprincipal
