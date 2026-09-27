package route

// Profile is the exact retired native Interactive Route v2 wire profile.
// Node keeps it only to refuse exact stale State records without side
// effects (ADR-0093); its sealed Introduction v1 grammar is retired by
// ADR-0094, and no maintained composition accepts a v2 listener.
const Profile = "ardents-interactive-route-v2"
