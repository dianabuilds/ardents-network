// Package credential owns the original Node runtime TLS material and its
// erasure. Load supplies the Unix file policy. OpenJoin also pins the original
// files and reobserves them on Check. The role compositions decide membership,
// refuse work on loss of authority and join TLS users before calling Close.
// Decoding and erasure have portable tests. File acquisition requires the
// Linux or macOS ownership and descriptor mechanisms.
package credential
