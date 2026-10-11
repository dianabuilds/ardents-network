// Package capsule owns the closed v3 Introduction envelope and recipient-only
// request grammar, with public-key sealing through the selected standard HPKE
// suite. It retains no private key, authority, registration, replay history or
// mutable Route lifetime. Decoded requests are untrusted candidate facts.
package capsule
