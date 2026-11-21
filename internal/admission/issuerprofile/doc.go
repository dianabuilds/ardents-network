// Package issuerprofile owns the canonical signed public issuer inventory,
// its class/hour cohorts and the purpose-bound request for a Node signature.
// It knows neither private keys, storage, quotas nor accepted live State.
// Inspect authenticates offline public identities and reports the signed
// interval; Verify additionally requires the exact independently selected
// interval. Neither performs wall-clock or Network authority acceptance.
package issuerprofile
