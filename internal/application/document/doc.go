// Package document owns the fixed bounded document Service exchange, trusted
// import of an explicitly selected local snapshot, and safe UTF-8 presentation.
// It owns no destination or publication authority. Its worker entrypoints have
// no file-selection operation and receive only an Endpoint-supplied snapshot.
package document
