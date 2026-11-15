// Package nodeprincipal validates the fixed Node account and independently
// observes current host account files and NSS. Each observation retains and
// closes its own files and helper. Creator separately retains original account
// preimages, invokes fixed useradd and observes postimages; the role owns effect
// admission and journal order. Neither owner grants Release or startup authority.
package nodeprincipal
