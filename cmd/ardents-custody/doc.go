// Command ardents-custody is the separate Authority custody adapter. It owns
// Service Authority creation and request-bound Credential issuance, closed
// admission Authority creation and holder-proof-bound permission issuance,
// encrypted record verification, Recovery Bundles, confirmed record purge, and
// pure public-envelope inspection. Secrets come only from an interactive
// terminal; private permissions never appear in a command receipt.
package main
